package handler

import (
  "context"
  "crypto/sha256"
  "encoding/json"
  "fmt"
  "log"
  "net/http"
  "io"
  "os"

  svix "github.com/svix/svix-webhooks/go"
  "github.com/nbd-wtf/go-nostr"
  "github.com/nbd-wtf/go-nostr/nip19"
)

type IncomingBoost struct {
  Amount           float64      `json:"amount"`
  Boostagram       interface{}  `json:"boostagram"`
  CreatedAt        string       `json:"created_at"`
  CreationDate     float64      `json:"creation_date"`
  Identifier       string       `json:"identifier"`
  Value            float64      `json:"value"`
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

  js, err := json.Marshal(IncomingBoost{
    Amount: transaction["amount"].(float64),
    Boostagram: transaction["boostagram"],
    CreatedAt: transaction["created_at"].(string),
    CreationDate: transaction["creation_date"].(float64),
    Identifier: transaction["identifier"].(string),
    Value: transaction["value"].(float64),
  });

  if err != nil {
    log.Print(err)
    os.Exit(1)
  }

  _, pk, _ := nip19.Decode(os.Getenv("NOSTR_NPUB"))
  _, sk, _ := nip19.Decode(os.Getenv("NOSTR_NSEC"))

  hsh := sha256.New()
  hsh.Write([]byte(payload))
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

  // Do something with the message...

  w.WriteHeader(http.StatusNoContent)
}
