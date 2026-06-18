package common

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

var NostrRelays = []string{
	"wss://relay.damus.io",
	"wss://nos.lol",
	"wss://relay.primal.net",
	"wss://nostr-pub.wellorder.net",
	"wss://nostr.oxtr.dev",
}

func PublishInvoiceToNostr(invoice IncomingInvoice) error {
	nostrRecord := invoice.GetNostrRecord()
	serializedMetadata, err := json.Marshal(nostrRecord)
	if err != nil {
		return fmt.Errorf("failed to serialize nostr record: %w", err)
	}

	return publishToNostr(serializedMetadata, invoice.GetBoostagram())
}

func publishToNostr(content []byte, boostagram Boostagram) error {
	_, pk, err := nip19.Decode(os.Getenv("NOSTR_NPUB"))
	if err != nil {
		return fmt.Errorf("failed to decode NOSTR_NPUB: %w", err)
	}

	_, sk, err := nip19.Decode(os.Getenv("NOSTR_NSEC"))
	if err != nil {
		return fmt.Errorf("failed to decode NOSTR_NSEC: %w", err)
	}

	hsh := sha256.New()
	hsh.Write(content)
	hash := fmt.Sprintf("%x", hsh.Sum(nil))

	tags := boostagramTags(hash, boostagram)

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
		Content:   string(content),
	}

	ev.Sign(skStr)

	var wg sync.WaitGroup
	for _, relayURL := range NostrRelays {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			relay, err := nostr.RelayConnect(ctx, url)
			if err != nil {
				log.Printf("failed to connect to relay %s: %v", url, err)
				return
			}
			defer relay.Close()

			if err := relay.Publish(ctx, ev); err != nil {
				log.Printf("failed to publish to relay %s: %v", url, err)
				return
			}

			log.Printf("published to %s", url)
		}(relayURL)
	}
	wg.Wait()

	return nil
}

func boostagramTags(hash string, boostagram Boostagram) nostr.Tags {
	tags := make(nostr.Tags, 0, 26)
	tags = append(tags, nostr.Tag{"d", hash})

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

	return tags
}
