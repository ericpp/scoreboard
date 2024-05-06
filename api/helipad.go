package handler

import (
    "context"
    "crypto/sha256"
    "database/sql"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "io"
    "os"
    "strings"
    "time"

    "github.com/nbd-wtf/go-nostr"
    "github.com/nbd-wtf/go-nostr/nip19"
    _ "github.com/lib/pq"
)

type HelipadWebhook struct {
    Index           int64         `json:"index"`
    Time            int64         `json:"time"`
    ValueMsat       int64         `json:"value_msat"`
    ValueMsatTotal  int64         `json:"value_msat_total"`
    Action          int8          `json:"action"`
    Sender          string        `json:"sender"`
    App             string        `json:"app"`
    Message         string        `json:"message"`
    Podcast         string        `json:"podcast"`
    Episode         string        `json:"episode"`
    Tlv             string        `json:"tlv"`
    RemotePodcast   *string       `json:"remote_podcast"`
    RemoteEpisode   *string       `json:"remote_epsidoe"`
    ReplySent       bool          `json:"reply_sent"`
    PaymentInfo     *interface{}  `json:"payment_info"`
}

type IncomingBoost struct {
    Amount           float64      `json:"amount"`
    Boostagram       interface{}  `json:"boostagram"`
    CreatedAt        string       `json:"created_at"`
    CreationDate     float64      `json:"creation_date"`
    Identifier       string       `json:"identifier"`
    Value            float64      `json:"value"`
}

func Handler(w http.ResponseWriter, r *http.Request) {
    authorization := r.Header.Get("Authorization")
    token := strings.Replace(authorization, "Bearer ", "", 1)

    if token != os.Getenv("HELIPAD_TOKEN") {
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
        log.Print(err)
        log.Print(payload)
        os.Exit(1)
    }

    if webhook.Action != 2 || webhook.PaymentInfo != nil {
        // not interested in non-boosts
        w.WriteHeader(http.StatusNoContent)
        return
    }

    var boostagram map[string]interface{}

    if err := json.Unmarshal([]byte(webhook.Tlv), &boostagram); err != nil {
        log.Print(err)
        log.Print(payload)
        os.Exit(1)
    }

    tm := time.Unix(webhook.Time, 0)

    boost := IncomingBoost {
        Amount: float64(webhook.ValueMsat) / 1000.0,
        Boostagram: boostagram,
        CreatedAt: tm.Format(time.RFC3339),
        CreationDate: float64(webhook.Time),
        Identifier: fmt.Sprintf("helipad-%d", webhook.Index),
        Value: float64(webhook.ValueMsat) / 1000.0,
    }

    if err := SaveToDatabase(boost); err != nil {
        log.Fatal(err)
    }

    if err := PublishToNostr(boost); err != nil {
        log.Fatal(err)
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

    boostagram, err := json.Marshal(boost.Boostagram)

    if err != nil {
        return err
    }

    var tlv map[string]interface{} = boost.Boostagram.(map[string]interface{})

    podcast := ""
    if val, ok := tlv["podcast"].(string); ok {
        podcast = val
    }

    episode := ""
    if val, ok := tlv["episode"].(string); ok {
        episode = val
    }

    app_name := ""
    if val, ok := tlv["app_name"].(string); ok {
        app_name = val
    }

    sender_name := ""
    if val, ok := tlv["sender_name"].(string); ok {
        sender_name = val
    }

    message := ""
    if val, ok := tlv["message"].(string); ok {
        message = val
    }

    var value_msat_total float64 = 0
    if val, ok := tlv["value_msat_total"].(float64); ok {
        value_msat_total = val
    }

    var feedID *float64 = nil
    if val, ok := tlv["feedID"].(float64); ok {
        feedID = &val
    }

    var itemID *float64 = nil
    if val, ok := tlv["itemID"].(float64); ok {
        itemID = &val
    }

    guid := ""
    if val, ok := tlv["guid"].(string); ok {
        guid = val
    }

    episode_guid := ""
    if val, ok := tlv["episode_guid"].(string); ok {
        episode_guid = val
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
        boostagram,
        boost.CreatedAt,
        boost.CreationDate,
        boost.Identifier,
        boost.Value,
        podcast,
        episode,
        app_name,
        sender_name,
        message,
        value_msat_total,
        feedID,
        itemID,
        guid,
        episode_guid,
    )

    if err != nil {
        return err
    }

    return nil
}

func PublishToNostr(boost IncomingBoost) error {
    js, err := json.Marshal(boost);

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

    for _, url := range []string{"wss://relay.damus.io", "wss://nos.lol", "wss://relay.nostr.band"} {
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
