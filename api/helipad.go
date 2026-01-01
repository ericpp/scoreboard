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
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

var nostrRelays = []string{"wss://relay.damus.io", "wss://nos.lol", "wss://relay.primal.net", "wss://nos.social"}

type HelipadWebhook struct {
	Index          int64        `json:"index"`
	Time           int64        `json:"time"`
	ValueMsat      int64        `json:"value_msat"`
	ValueMsatTotal int64        `json:"value_msat_total"`
	Action         int8         `json:"action"`
	Sender         string       `json:"sender"`
	App            string       `json:"app"`
	Message        string       `json:"message"`
	Podcast        string       `json:"podcast"`
	Episode        string       `json:"episode"`
	Tlv            string       `json:"tlv"`
	RemotePodcast  *string      `json:"remote_podcast"`
	RemoteEpisode  *string      `json:"remote_episode"`
	ReplySent      bool         `json:"reply_sent"`
	PaymentInfo    *interface{} `json:"payment_info"`
}

type IncomingBoost struct {
	Amount       float64     `json:"amount"`
	Boostagram   interface{} `json:"boostagram"`
	CreatedAt    string      `json:"created_at"`
	CreationDate float64     `json:"creation_date"`
	Identifier   string      `json:"identifier"`
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

func (i IncomingBoost) ParseBoostagram() (Boostagram, error) {
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

func Handler(w http.ResponseWriter, r *http.Request) {
	authorization := r.Header.Get("Authorization")
	if authorization == "" {
		log.Print("Missing Authorization header")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	token := strings.Replace(authorization, "Bearer ", "", 1)
	if token == "" || token != os.Getenv("HELIPAD_TOKEN") {
		log.Print("Authorization token does not match")
		w.WriteHeader(http.StatusForbidden)
		return
	}

	payload, err := io.ReadAll(r.Body)

	if err != nil {
		log.Print("Unable to read webhook payload")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var webhook HelipadWebhook

	if err := json.Unmarshal([]byte(payload), &webhook); err != nil {
		log.Printf("failed to unmarshal webhook payload: %v", err)
		log.Printf("payload: %s", payload)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if webhook.Action != 2 || webhook.PaymentInfo != nil {
		// not interested in non-boosts
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var boostagram map[string]interface{}

	if err := json.Unmarshal([]byte(webhook.Tlv), &boostagram); err != nil {
		log.Printf("failed to unmarshal TLV boostagram: %v", err)
		log.Printf("payload: %s", payload)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tm := time.Unix(webhook.Time, 0)

	boost := IncomingBoost{
		Amount:       float64(webhook.ValueMsat) / 1000.0,
		Boostagram:   boostagram,
		CreatedAt:    tm.Format(time.RFC3339),
		CreationDate: float64(webhook.Time),
		Identifier:   fmt.Sprintf("helipad-%d", webhook.Index),
		Value:        float64(webhook.ValueMsat) / 1000.0,
	}

	if err := SaveToDatabase(boost); err != nil {
		log.Printf("failed to save boost to database: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := PublishToNostr(boost); err != nil {
		log.Printf("failed to publish boost to nostr: %v", err)
		// Don't fail the request if nostr publishing fails - it's a non-critical operation
		// The boost is already saved to the database
	}

	// Do something with the message...

	w.WriteHeader(http.StatusNoContent)
}

func SaveToDatabase(boost IncomingBoost) error {
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

	log.Printf("inserting %s", boost.Identifier)

	serializedBoostagram, err := json.Marshal(boost.Boostagram)
	if err != nil {
		log.Printf("failed to serialize boostagram for boost %s: %v", boost.Identifier, err)
		serializedBoostagram = []byte("null")
	}

	boostagram, err := boost.ParseBoostagram()
	if err != nil {
		log.Printf("failed to parse boostagram for boost %s: %v", boost.Identifier, err)
		boostagram = Boostagram{}
	}

	insertSql :=
		`INSERT INTO invoices
        (amount, boostagram, created_at, creation_date, identifier, value, podcast, episode, app_name, sender_name, message, value_msat_total, feed_id, item_id, guid, episode_guid)
    VALUES
        ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
    ON CONFLICT (identifier) DO NOTHING`

	_, err = db.Exec(
		insertSql,
		boost.Amount,
		serializedBoostagram,
		boost.CreatedAt,
		boost.CreationDate,
		boost.Identifier,
		boost.Value,
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
	)

	if err != nil {
		return err
	}

	return nil
}

func PublishToNostr(boost IncomingBoost) error {
	serialized, err := json.Marshal(boost)

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

	boostagram, err := boost.ParseBoostagram()
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
