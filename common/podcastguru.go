package common

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
)

func ExtractPodcastGuruURL(comment string) string {
	idx := strings.Index(comment, "V4V:")
	if idx == -1 {
		return ""
	}

	remaining := comment[idx+4:]
	parts := strings.Fields(remaining)

	for _, part := range parts {
		if strings.Contains(part, "boost.podcastguru.io/boost/") {
			return part
		}
	}

	return ""
}

func FetchPodcastGuruPaymentIfNeeded(invoice *IncomingInvoice) error {
	if invoice.PodcastGuruPayment != nil {
		return nil
	}

	boostURL := ExtractPodcastGuruURL(invoice.Comment)
	if boostURL == "" {
		return nil
	}

	payment, err := FetchPodcastGuruPayment(boostURL)
	if err != nil {
		return fmt.Errorf("failed to fetch PodcastGuru payment: %w", err)
	}

	invoice.PodcastGuruPayment = &payment
	return nil
}

func FetchPodcastGuruPayment(boostURL string) (PodcastGuruPayment, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	return FetchPodcastGuruPaymentWithClient(client, boostURL)
}

func FetchPodcastGuruPaymentWithClient(client *http.Client, boostURL string) (PodcastGuruPayment, error) {
	resp, err := client.Get(boostURL)
	if err != nil {
		return PodcastGuruPayment{}, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	var payment PodcastGuruPayment
	if err := json.NewDecoder(resp.Body).Decode(&payment); err != nil {
		return PodcastGuruPayment{}, fmt.Errorf("failed to parse PodcastGuru payment: %w", err)
	}

	if payment.MetadataPayload != "" {
		var metadata PodcastGuruMetadata
		if err := json.Unmarshal([]byte(payment.MetadataPayload), &metadata); err == nil {
			payment.ParsedMetadata = &metadata
		}
	}

	return payment, nil
}

func (p PodcastGuruPayment) ParseBoostagram() Boostagram {
	var metadata PodcastGuruMetadata
	if p.ParsedMetadata != nil {
		metadata = *p.ParsedMetadata
	} else if p.MetadataPayload != "" {
		_ = json.Unmarshal([]byte(p.MetadataPayload), &metadata)
	}

	valueMsatTotal := metadata.ValueMsatTotal
	if valueMsatTotal == 0 && metadata.TotalAmount > 0 {
		valueMsatTotal = int(math.Floor(metadata.TotalAmount))
	}

	return Boostagram{
		Action:         strings.ToLower(metadata.Action),
		Podcast:        metadata.Podcast,
		Episode:        metadata.Episode,
		AppName:        metadata.AppName,
		SenderName:     metadata.SenderName,
		Message:        metadata.Message,
		ValueMsatTotal: valueMsatTotal,
		Guid:           metadata.Guid,
		EpisodeGuid:    metadata.EpisodeGuid,
		RemoteFeedGuid: metadata.RemoteFeedGuid,
		RemoteItemGuid: metadata.RemoteItemGuid,
		FeedID:         nil,
		ItemID:         nil,
		BlockGuid:      "",
		EventGuid:      "",
	}
}
