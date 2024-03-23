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

    svix "github.com/svix/svix-webhooks/go"
    "github.com/nbd-wtf/go-nostr"
    "github.com/nbd-wtf/go-nostr/nip19"
    _ "github.com/lib/pq"
)

type IncomingBoost struct {
    Amount           float64      `json:"amount"`
    Boostagram       interface{}  `json:"boostagram"`
    CreatedAt        string       `json:"created_at"`
    CreationDate     float64      `json:"creation_date"`
    Identifier       string       `json:"identifier"`
    Value            float64      `json:"value"`
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

    insertSql :=
    `INSERT INTO invoices
        (amount, boostagram, created_at, creation_date, identifier, value, podcast, episode, app_name, sender_name, message, value_msat_total)
    VALUES
        ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
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

    var transaction map[string]interface{}

    if err := json.Unmarshal([]byte(payload), &transaction); err != nil {
        log.Print(err)
        log.Print(payload)
        os.Exit(1)
    }

    boost := IncomingBoost{
        Amount: transaction["amount"].(float64),
        Boostagram: transaction["boostagram"],
        CreatedAt: transaction["created_at"].(string),
        CreationDate: transaction["creation_date"].(float64),
        Identifier: transaction["identifier"].(string),
        Value: transaction["value"].(float64),
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
