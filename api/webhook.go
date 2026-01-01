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

	_ "github.com/lib/pq"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	svix "github.com/svix/svix-webhooks/go"
)

const nostrRelays = []string{"wss://relay.damus.io", "wss://nos.lol", "wss://relay.nostr.band"}

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
	js, err := json.Marshal(invoice)

	if err != nil {
		return err
	}

	_, pk, _ := nip19.Decode(os.Getenv("NOSTR_NPUB"))
	_, sk, _ := nip19.Decode(os.Getenv("NOSTR_NSEC"))

	hsh := sha256.New()
	hsh.Write([]byte(js))
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

	ev := nostr.Event{
		PubKey:    pk.(string),
		CreatedAt: nostr.Now(),
		Kind:      nostr.KindApplicationSpecificData,
		Tags:      tags,
		Content:   string(js),
	}

	// calling Sign sets the event ID field and the event Sig field
	ev.Sign(sk.(string))

	// publish the event to two relays
	ctx := context.Background()

	for _, url := range nostrRelays {
		relay, err := nostr.RelayConnect(ctx, url)

		if err != nil {
			fmt.Println(err)
			continue
		}

		if err := relay.Publish(ctx, ev); err != nil {
			fmt.Println(err)
			continue
		}

		fmt.Printf("published to %s\n", url)
	}

	return nil
}

func Handler(w http.ResponseWriter, r *http.Request) {
	wh, err := svix.NewWebhook(os.Getenv("ALBY_WEBHOOK"))

	if err != nil {
		log.Fatal(err)
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

	var transaction map[string]interface{}

	if err := json.Unmarshal([]byte(payload), &transaction); err != nil {
		log.Print(err)
		log.Print(payload)
		os.Exit(1)
	}

	comment := ""
	if val, ok := transaction["comment"].(string); ok {
		comment = val
	}

	description := ""
	if val, ok := transaction["description"].(string); ok {
		description = val
	}

	payer_name := ""
	if val, ok := transaction["payer_name"].(string); ok {
		payer_name = val
	}

	invoice := IncomingInvoice{
		Amount:       transaction["amount"].(float64),
		Boostagram:   transaction["boostagram"],
		Comment:      comment,
		CreatedAt:    transaction["created_at"].(string),
		CreationDate: transaction["creation_date"].(float64),
		Description:  description,
		Identifier:   transaction["identifier"].(string),
		PayerName:    payer_name,
		Value:        transaction["value"].(float64),
	}

	if err := SaveToDatabase(invoice); err != nil {
		log.Fatal(err)
	}

	if err := PublishToNostr(invoice); err != nil {
		log.Fatal(err)
	}

	// Do something with the message...

	w.WriteHeader(http.StatusNoContent)
}
