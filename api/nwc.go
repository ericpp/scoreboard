package handler

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

var nostrRelays = []string{"wss://relay.damus.io", "wss://nos.lol", "wss://relay.primal.net"}

type WebhookPayload struct {
	Type    string              `json:"type"`
	Payment PaymentNotification `json:"payment"`
}

type PaymentNotification struct {
	Type            string                 `json:"type"`
	State           *string                `json:"state"`
	Invoice         *string                `json:"invoice"`
	Description     *string                `json:"description"`
	DescriptionHash *string                `json:"description_hash"`
	Preimage        *string                `json:"preimage"`
	PaymentHash     string                 `json:"payment_hash"`
	Amount          int64                  `json:"amount"`
	FeesPaid        int64                  `json:"fees_paid"`
	CreatedAt       int64                  `json:"created_at"`
	ExpiresAt       *int64                 `json:"expires_at"`
	SettledAt       *int64                 `json:"settled_at"`
	Metadata        map[string]interface{} `json:"metadata"`
}

type IncomingInvoice struct {
	Amount       float64     `json:"amount"`
	Boostagram   *Boostagram `json:"boostagram"`
	Comment      string      `json:"comment"`
	CreatedAt    string      `json:"created_at"`
	CreationDate float64     `json:"creation_date"`
	Description  string      `json:"description"`
	Identifier   string      `json:"identifier"`
	PayerName    string      `json:"payer_name"`
	Value        float64     `json:"value"`
	RSSPayment   *RssPayment // parsed from comment
}

type NostrRecord struct {
	Amount       float64     `json:"amount"`
	Boostagram   *Boostagram `json:"boostagram"`
	Comment      string      `json:"comment"`
	CreatedAt    string      `json:"created_at"`
	CreationDate float64     `json:"creation_date"`
	Description  string      `json:"description"`
	Identifier   string      `json:"identifier"`
	PayerName    string      `json:"payer_name"`
	Value        float64     `json:"value"`
}

type Boostagram struct {
	Action         string   `json:"action"`
	Podcast        string   `json:"podcast"`
	Episode        string   `json:"episode"`
	AppName        string   `json:"app_name"`
	SenderName     string   `json:"sender_name"`
	Message        string   `json:"message"`
	ValueMsatTotal int      `json:"value_msat_total"`
	FeedID         *float64 `json:"feedID"`
	ItemID         *float64 `json:"itemID"`
	Guid           string   `json:"guid"`
	EpisodeGuid    string   `json:"episode_guid"`
	BlockGuid      string   `json:"blockGuid"` // splitkit
	EventGuid      string   `json:"eventGuid"` // splitkit
	RemoteFeedGuid string   `json:"remote_feed_guid"`
	RemoteItemGuid string   `json:"remote_item_guid"`
}

func ParsePaymentNotification(payload []byte) (IncomingInvoice, error) {
	var webhook WebhookPayload

	if err := json.Unmarshal(payload, &webhook); err != nil {
		return IncomingInvoice{}, fmt.Errorf("failed to unmarshal webhook payload: %w", err)
	}

	if webhook.Type != "payment_received" {
		return IncomingInvoice{}, fmt.Errorf("unsupported webhook type: %s", webhook.Type)
	}

	notification := webhook.Payment

	if notification.Type != "incoming" {
		return IncomingInvoice{}, fmt.Errorf("unsupported payment type: %s", notification.Type)
	}

	invoice := IncomingInvoice{
		Amount:       float64(int(notification.Amount / 1000)),
		Value:        float64(int(notification.Amount / 1000)),
		Identifier:   notification.PaymentHash,
		CreatedAt:    time.Unix(notification.CreatedAt, 0).Format(time.RFC3339),
		CreationDate: float64(notification.CreatedAt),
	}

	if notification.Description != nil {
		invoice.Description = *notification.Description
	}

	if notification.Metadata != nil {
		if tlvRecords, ok := notification.Metadata["tlv_records"].([]interface{}); ok && len(tlvRecords) > 0 {
			for _, record := range tlvRecords {
				if recordMap, ok := record.(map[string]interface{}); ok {
					typeNum, _ := recordMap["type"].(float64)
					if int64(typeNum) == 7629169 {
						if hexValue, ok := recordMap["value"].(string); ok {
							boostagram, err := parseBoostagramFromHex(hexValue)
							if err != nil {
								log.Printf("failed to parse boostagram from hex: %v", err)
								continue
							}
							invoice.Boostagram = &boostagram
							invoice.Comment = boostagram.Message
							invoice.PayerName = boostagram.SenderName
							break
						}
					}
				}
			}
		}
	}

	return invoice, nil
}

func parseBoostagramFromHex(hexValue string) (Boostagram, error) {
	decoded, err := hex.DecodeString(hexValue)
	if err != nil {
		return Boostagram{}, fmt.Errorf("failed to decode hex: %w", err)
	}

	var data Boostagram
	if err := json.Unmarshal(decoded, &data); err != nil {
		return Boostagram{}, fmt.Errorf("failed to unmarshal boostagram: %w", err)
	}

	return data, nil
}

func ParseInvoiceFromJson(payload []byte) (IncomingInvoice, error) {
	var invoice IncomingInvoice

	if err := json.Unmarshal(payload, &invoice); err != nil {
		return IncomingInvoice{}, fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	return invoice, nil
}

func FetchRSSPaymentIfNeeded(invoice *IncomingInvoice) error {
	// Process RSS payment if present in comment
	if invoice.RSSPayment == nil {
		url := extractRSSPaymentURL(invoice.Comment)
		if url != "" {
			rssPayment, err := fetchRSSPaymentBoostagram(url)
			if err != nil {
				return fmt.Errorf("failed to fetch RSS payment boostagram: %w", err)
			}

			invoice.RSSPayment = &rssPayment
		}
	}

	return nil
}

func (i IncomingInvoice) GetBoostagram() Boostagram {
	if i.Boostagram != nil {
		return *i.Boostagram
	}

	// Convert RSS payment to Boostagram
	if i.RSSPayment != nil {
		rssPayment := *i.RSSPayment
		return rssPayment.ParseBoostagram()
	}

	return Boostagram{}
}

func (i IncomingInvoice) GetSerializedMetadata() ([]byte, error) {
	var metadata interface{}

	if i.Boostagram != nil {
		metadata = i.Boostagram
	} else if i.RSSPayment != nil {
		metadata = i.RSSPayment
	}

	if metadata != nil {
		return json.Marshal(metadata)
	}

	return []byte("null"), nil
}

func (i IncomingInvoice) GetNostrRecord() NostrRecord {
	boostagram := i.GetBoostagram()

	return NostrRecord{
		Amount:       i.Amount,
		Boostagram:   &boostagram,
		Comment:      i.Comment,
		CreatedAt:    i.CreatedAt,
		CreationDate: i.CreationDate,
		Description:  i.Description,
		Identifier:   i.Identifier,
		PayerName:    i.PayerName,
		Value:        i.Value,
	}
}

type RssPayment struct {
	Action              string  `json:"action"`
	AppName             string  `json:"app_name"`
	FeedGuid            string  `json:"feed_guid"`
	FeedTitle           string  `json:"feed_title"`
	Group               string  `json:"group"`
	Id                  string  `json:"id"`
	ItemGuid            string  `json:"item_guid"`
	ItemTitle           string  `json:"item_title"`
	Link                string  `json:"link"`
	Message             string  `json:"message"`
	Position            int     `json:"position"`
	PublisherGuid       string  `json:"publisher_guid"`
	PublisherTitle      string  `json:"publisher_title"`
	RecipientAddress    string  `json:"recipient_address"`
	RemoteFeedGuid      string  `json:"remote_feed_guid"`
	RemoteItemGuid      string  `json:"remote_item_guid"`
	RemotePublisherGuid string  `json:"remote_publisher_guid"`
	SenderId            string  `json:"sender_id"`
	SenderName          string  `json:"sender_name"`
	SenderNpub          string  `json:"sender_npub"`
	Split               float64 `json:"split"`
	Timestamp           string  `json:"timestamp"`
	ValueMsat           float64 `json:"value_msat"`
	ValueMsatTotal      float64 `json:"value_msat_total"`
	ValueUsd            float64 `json:"value_usd"`
}

func (r RssPayment) ParseBoostagram() Boostagram {
	// WHY DID FOUNTAIN MAKE THIS DIFFERENT!?
	return Boostagram{
		Action:         strings.ToLower(r.Action),
		Podcast:        r.FeedTitle,
		Episode:        r.ItemTitle,
		AppName:        r.AppName,
		SenderName:     r.SenderName,
		Message:        r.Message,
		ValueMsatTotal: int(math.Floor(r.ValueMsatTotal)),
		Guid:           r.FeedGuid,
		EpisodeGuid:    r.ItemGuid,
		RemoteFeedGuid: r.RemoteFeedGuid,
		RemoteItemGuid: r.RemoteItemGuid,
		FeedID:         nil,
		ItemID:         nil,
		BlockGuid:      "",
		EventGuid:      "",
	}
}

func extractRSSPaymentURL(comment string) string {
	// Look for rss::payment:: pattern (e.g., "rss::payment::stream")
	idx := strings.Index(comment, "rss::payment::")
	if idx == -1 {
		return ""
	}

	// The HTTP link always follows the rss::payment::<something> tag
	// Extract everything after "rss::payment::" and find the first HTTP/HTTPS URL
	remaining := comment[idx:]
	parts := strings.Fields(remaining)

	// Find the first HTTP/HTTPS URL (skip the rss::payment::<something> part)
	for _, part := range parts {
		if strings.HasPrefix(part, "http://") || strings.HasPrefix(part, "https://") {
			return part
		}
	}
	return ""
}

func fetchRSSPaymentBoostagram(paymentURL string) (RssPayment, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Head(paymentURL)
	if err != nil {
		return RssPayment{}, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	rssPaymentHeader := resp.Header.Get("x-rss-payment")
	if rssPaymentHeader == "" {
		return RssPayment{}, fmt.Errorf("x-rss-payment header not found")
	}

	// URL-decode the header value before parsing as JSON
	decodedHeader, err := url.QueryUnescape(rssPaymentHeader)
	if err != nil {
		return RssPayment{}, fmt.Errorf("failed to decode x-rss-payment header: %w", err)
	}

	var rssPayment RssPayment
	if err := json.Unmarshal([]byte(decodedHeader), &rssPayment); err != nil {
		return RssPayment{}, fmt.Errorf("failed to parse x-rss-payment header: %w", err)
	}

	return rssPayment, nil
}

func SaveToDatabase(invoice IncomingInvoice) (bool, error) {
	// open database
	db, err := sql.Open("postgres", os.Getenv("POSTGRES_URL"))
	if err != nil {
		return false, err
	}

	// close database
	defer db.Close()

	// check db
	if err = db.Ping(); err != nil {
		return false, err
	}

	log.Printf("inserting %s", invoice.Identifier)

	serializedMetadata, err := invoice.GetSerializedMetadata()
	if err != nil {
		log.Printf("failed to serialize boostagram for invoice %s: %v", invoice.Identifier, err)
	}

	boostagram := invoice.GetBoostagram()
	insertSql :=
		`INSERT INTO invoices
        (amount, boostagram, comment, created_at, creation_date, description, identifier, payer_name, value, podcast, episode, app_name, sender_name, message, value_msat_total, feed_id, item_id, guid, episode_guid, action, event_guid)
    VALUES
        ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
    ON CONFLICT (identifier) DO NOTHING`

	result, err := db.Exec(
		insertSql,
		invoice.Amount,
		serializedMetadata,
		invoice.Comment,
		invoice.CreatedAt,
		invoice.CreationDate,
		invoice.Description,
		invoice.Identifier,
		invoice.PayerName,
		invoice.Value,
		boostagram.Podcast,
		boostagram.Episode,
		boostagram.AppName,
		boostagram.SenderName,
		boostagram.Message,
		boostagram.ValueMsatTotal,
		boostagram.FeedID,
		boostagram.ItemID,
		boostagram.Guid,
		boostagram.EpisodeGuid,
		boostagram.Action,
		boostagram.EventGuid,
	)

	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rowsAffected > 0, nil
}

func UpdateDatabaseWithRSSPayment(invoice IncomingInvoice) error {
	if invoice.RSSPayment == nil {
		return nil
	}

	// open database
	db, err := sql.Open("postgres", os.Getenv("POSTGRES_URL"))
	if err != nil {
		return err
	}

	// close database
	defer db.Close()

	// check db
	if err = db.Ping(); err != nil {
		return err
	}

	log.Printf("updating %s with RSS payment info", invoice.Identifier)

	serializedMetadata, err := invoice.GetSerializedMetadata()
	if err != nil {
		log.Printf("failed to serialize boostagram for invoice %s: %v", invoice.Identifier, err)
		return err
	}

	boostagram := invoice.GetBoostagram()
	updateSql :=
		`UPDATE invoices SET
        boostagram = $1,
        podcast = $2,
        episode = $3,
        app_name = $4,
        sender_name = $5,
        message = $6,
        value_msat_total = $7,
        feed_id = $8,
        item_id = $9,
        guid = $10,
        episode_guid = $11,
        action = $12,
        event_guid = $13
    WHERE identifier = $14`

	_, err = db.Exec(
		updateSql,
		serializedMetadata,
		boostagram.Podcast,
		boostagram.Episode,
		boostagram.AppName,
		boostagram.SenderName,
		boostagram.Message,
		boostagram.ValueMsatTotal,
		boostagram.FeedID,
		boostagram.ItemID,
		boostagram.Guid,
		boostagram.EpisodeGuid,
		boostagram.Action,
		boostagram.EventGuid,
		invoice.Identifier,
	)

	if err != nil {
		return err
	}

	return nil
}

func PublishToNostr(invoice IncomingInvoice) error {
	nostrRecord := invoice.GetNostrRecord()
	serializedMetadata, err := json.Marshal(nostrRecord)

	if err != nil {
		return fmt.Errorf("failed to serialize nostr record: %w", err)
	}

	_, pk, err := nip19.Decode(os.Getenv("NOSTR_NPUB"))
	if err != nil {
		return fmt.Errorf("failed to decode NOSTR_NPUB: %w", err)
	}

	_, sk, err := nip19.Decode(os.Getenv("NOSTR_NSEC"))
	if err != nil {
		return fmt.Errorf("failed to decode NOSTR_NSEC: %w", err)
	}

	hsh := sha256.New()
	hsh.Write([]byte(serializedMetadata))
	hash := fmt.Sprintf("%x", hsh.Sum(nil))

	tags := make(nostr.Tags, 0, 26)
	tags = append(tags, nostr.Tag{"d", hash})

	boostagram := invoice.GetBoostagram()
	if boostagram.Guid != "" {
		tags = append(tags, nostr.Tag{"i", "podcast:guid:" + boostagram.Guid})
		tags = append(tags, nostr.Tag{"k", "podcast:guid"})
	}

	if boostagram.EpisodeGuid != "" {
		tags = append(tags, nostr.Tag{"i", "podcast:item:guid:" + boostagram.EpisodeGuid})
		tags = append(tags, nostr.Tag{"k", "podcast:item:guid"})
	}

	if boostagram.RemoteFeedGuid != "" {
		tags = append(tags, nostr.Tag{"i", "podcast:remote:guid:" + boostagram.RemoteFeedGuid})
		tags = append(tags, nostr.Tag{"k", "podcast:remote:guid"})
	}

	if boostagram.RemoteItemGuid != "" {
		tags = append(tags, nostr.Tag{"i", "podcast:remote:item:guid:" + boostagram.RemoteItemGuid})
		tags = append(tags, nostr.Tag{"k", "podcast:remote:item:guid"})
	}

	if boostagram.BlockGuid != "" {
		tags = append(tags, nostr.Tag{"i", "thesplitkit:block:guid:" + boostagram.BlockGuid})
		tags = append(tags, nostr.Tag{"k", "thesplitkit:block:guid"})
	}

	if boostagram.EventGuid != "" {
		tags = append(tags, nostr.Tag{"i", "thesplitkit:event:guid:" + boostagram.EventGuid})
		tags = append(tags, nostr.Tag{"k", "thesplitkit:event:guid"})
	}

	pkStr, ok := pk.(string)
	if !ok {
		return fmt.Errorf("NOSTR_NPUB did not decode to string")
	}

	skStr, ok := sk.(string)
	if !ok {
		return fmt.Errorf("NOSTR_NSEC did not decode to string")
	}

	ev := nostr.Event{
		PubKey:    pkStr,
		CreatedAt: nostr.Now(),
		Kind:      nostr.KindApplicationSpecificData,
		Tags:      tags,
		Content:   string(serializedMetadata),
	}

	// calling Sign sets the event ID field and the event Sig field
	ev.Sign(skStr)

	var wg sync.WaitGroup
	for _, url := range nostrRelays {
		wg.Add(1)
		go func(relayURL string) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			relay, err := nostr.RelayConnect(ctx, relayURL)
			if err != nil {
				log.Printf("failed to connect to relay %s: %v", relayURL, err)
				return
			}
			defer relay.Close()

			if err := relay.Publish(ctx, ev); err != nil {
				log.Printf("failed to publish to relay %s: %v", relayURL, err)
				return
			}

			log.Printf("published to %s", relayURL)
		}(url)
	}
	wg.Wait()

	return nil
}

func Handler(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		log.Print("Missing Authorization header")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		log.Print("Invalid Authorization header format")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	token := parts[1]
	expectedToken := os.Getenv("NWC_WEBHOOK_TOKEN")
	if expectedToken == "" {
		log.Print("NWC_WEBHOOK_TOKEN environment variable not set")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if token != expectedToken {
		log.Print("Invalid bearer token")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		log.Print("Unable to read webhook payload")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Printf("incoming webhook %s", payload)

	invoice, err := ParsePaymentNotification(payload)
	if err != nil {
		log.Printf("failed to parse payment notification: %v", err)
		log.Printf("payload: %s", payload)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	isNewPayment, err := SaveToDatabase(invoice)
	if err != nil {
		log.Printf("failed to save to database: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !isNewPayment {
		log.Printf("payment %s already exists in database, skipping broadcast", invoice.Identifier)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := FetchRSSPaymentIfNeeded(&invoice); err != nil {
		log.Printf("failed to fetch RSS payment: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if invoice.RSSPayment != nil {
		if err := UpdateDatabaseWithRSSPayment(invoice); err != nil {
			log.Printf("failed to update database with RSS payment: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	if err := PublishToNostr(invoice); err != nil {
		log.Printf("failed to publish to nostr: %v", err)
	}

	w.WriteHeader(http.StatusNoContent)
}
