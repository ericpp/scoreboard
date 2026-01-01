package handler

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	svix "github.com/svix/svix-webhooks/go"
)

var nostrRelays = []string{"wss://relay.damus.io", "wss://nos.lol", "wss://relay.primal.net"}

type IncomingInvoice struct {
	Amount       float64     `json:"amount"`
	Boostagram   interface{} `json:"boostagram"`
	Comment      string      `json:"comment"`
	CreatedAt    string      `json:"created_at"`
	CreationDate float64     `json:"creation_date"`
	Description  string      `json:"description"`
	Identifier   string      `json:"identifier"`
	PayerName    string      `json:"payer_name"`
	Value        float64     `json:"value"`
}

type Boostagram struct {
	Action         string  `json:"action"`
	Podcast        string  `json:"podcast"`
	Episode        string  `json:"episode"`
	AppName        string  `json:"app_name"`
	SenderName     string  `json:"sender_name"`
	Message        string  `json:"message"`
	ValueMsatTotal int     `json:"value_msat_total"`
	FeedID         float64 `json:"feedID"`
	ItemID         float64 `json:"itemID"`
	Guid           string  `json:"guid"`
	EpisodeGuid    string  `json:"episode_guid"`
	BlockGuid      string  `json:"blockGuid"` // splitkit
	EventGuid      string  `json:"eventGuid"` // splitkit
	RemoteFeedGuid string  `json:"remote_feed_guid"`
	RemoteItemGuid string  `json:"remote_item_guid"`
}

func (i IncomingInvoice) ParseBoostagram() (Boostagram, error) {
	// Check if boostagram is nil or missing
	if i.Boostagram == nil {
		return Boostagram{}, nil
	}

	serializedBoostagram, err := json.Marshal(i.Boostagram)
	if err != nil {
		return Boostagram{}, err
	}

	var boostagram Boostagram
	err = json.Unmarshal(serializedBoostagram, &boostagram)
	if err != nil {
		return Boostagram{}, err
	}

	return boostagram, nil
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

func fetchRSSPaymentBoostagram(paymentURL string) (interface{}, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Head(paymentURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	rssPaymentHeader := resp.Header.Get("x-rss-payment")
	if rssPaymentHeader == "" {
		return nil, fmt.Errorf("x-rss-payment header not found")
	}

	// URL-decode the header value before parsing as JSON
	decodedHeader, err := url.QueryUnescape(rssPaymentHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode x-rss-payment header: %w", err)
	}

	var boostagram interface{}
	if err := json.Unmarshal([]byte(decodedHeader), &boostagram); err != nil {
		return nil, fmt.Errorf("failed to parse x-rss-payment header: %w", err)
	}

	return boostagram, nil
}

func SaveToDatabase(invoice IncomingInvoice) error {
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

	log.Printf("inserting %s", invoice.Identifier)

	serializedBoostagram, err := json.Marshal(invoice.Boostagram)
	if err != nil {
		log.Printf("failed to serialize boostagram for invoice %s: %v", invoice.Identifier, err)
		serializedBoostagram = []byte("null")
	}

	boostagram, err := invoice.ParseBoostagram()
	if err != nil {
		log.Printf("failed to parse boostagram for invoice %s: %v", invoice.Identifier, err)
		boostagram = Boostagram{}
	}

	insertSql :=
		`INSERT INTO invoices
        (amount, boostagram, comment, created_at, creation_date, description, identifier, payer_name, value, podcast, episode, app_name, sender_name, message, value_msat_total, feed_id, item_id, guid, episode_guid, action, event_guid)
    VALUES
        ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
    ON CONFLICT (identifier) DO NOTHING`

	_, err = db.Exec(
		insertSql,
		invoice.Amount,
		serializedBoostagram,
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
		return err
	}

	return nil
}

func PublishToNostr(invoice IncomingInvoice) error {
	serialized, err := json.Marshal(invoice)

	if err != nil {
		return err
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
	hsh.Write([]byte(serialized))
	hash := fmt.Sprintf("%x", hsh.Sum(nil))

	tags := make(nostr.Tags, 0, 26)
	tags = append(tags, nostr.Tag{"d", hash})

	boostagram, err := invoice.ParseBoostagram()
	if err != nil {
		log.Printf("failed to parse boostagram for nostr publishing: %v", err)
		boostagram = Boostagram{}
	}

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
		Content:   string(serialized),
	}

	// calling Sign sets the event ID field and the event Sig field
	ev.Sign(skStr)

	// publish the event to relays with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, url := range nostrRelays {
		relay, err := nostr.RelayConnect(ctx, url)

		if err != nil {
			log.Printf("failed to connect to relay %s: %v", url, err)
			continue
		}

		if err := relay.Publish(ctx, ev); err != nil {
			log.Printf("failed to publish to relay %s: %v", url, err)
			relay.Close()
			continue
		}

		log.Printf("published to %s", url)
		relay.Close()
	}

	return nil
}

func Handler(w http.ResponseWriter, r *http.Request) {
	wh, err := svix.NewWebhook(os.Getenv("ALBY_WEBHOOK"))

	if err != nil {
		log.Printf("failed to create webhook verifier: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	payload, err := io.ReadAll(r.Body)

	if err != nil {
		log.Print("Unable to read webhook payload")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err = wh.Verify(payload, r.Header)
	if err != nil {
		log.Print("Unable to verify webhook payload")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Printf("incoming webhook %s", payload)

	var invoice IncomingInvoice

	if err := json.Unmarshal(payload, &invoice); err != nil {
		log.Printf("failed to unmarshal payload: %v", err)
		log.Printf("payload: %s", payload)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Process RSS payment if present in comment and boostagram is nil
	if invoice.Boostagram == nil {
		url := extractRSSPaymentURL(invoice.Comment)
		if url != "" {
			log.Printf("found RSS payment URL in comment: %s", url)

			invoice.Boostagram, err = fetchRSSPaymentBoostagram(url)
			if err != nil {
				log.Printf("failed to fetch RSS payment boostagram: %v", err)
				invoice.Boostagram = nil
			}
		}
	}

	if err := SaveToDatabase(invoice); err != nil {
		log.Printf("failed to save to database: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := PublishToNostr(invoice); err != nil {
		log.Printf("failed to publish to nostr: %v", err)
		// Don't fail the request if nostr publishing fails, as the data is already saved
	}

	// Do something with the message...

	w.WriteHeader(http.StatusNoContent)
}
