package common

import (
	"testing"
	"time"
)

func TestParseHelipadWebhook(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"direction": "incoming",
		"index": 42,
		"time": 1700000000,
		"value_msat": 5000,
		"value_msat_total": 10000,
		"action": 2,
		"sender": "Mark Pugner",
		"app": "CurioCaster",
		"message": "My boost message",
		"podcast": "Podcast name",
		"episode": "Episode name",
		"tlv": "{\"action\":\"boost\",\"podcast\":\"Test Show\"}",
		"reply_sent": false,
		"memo": "",
		"payment_info": null
	}`)

	webhook, err := ParseHelipadWebhook(payload)
	if err != nil {
		t.Fatalf("ParseHelipadWebhook() error = %v", err)
	}

	if webhook.Direction != "incoming" {
		t.Errorf("Direction = %q, want incoming", webhook.Direction)
	}
	if webhook.Index != 42 {
		t.Errorf("Index = %d, want 42", webhook.Index)
	}
	if webhook.ValueMsat != 5000 {
		t.Errorf("ValueMsat = %d, want 5000", webhook.ValueMsat)
	}
	if webhook.ValueMsatTotal != 10000 {
		t.Errorf("ValueMsatTotal = %d, want 10000", webhook.ValueMsatTotal)
	}
	if webhook.Action != 2 {
		t.Errorf("Action = %d, want 2", webhook.Action)
	}
	if webhook.Sender != "Mark Pugner" {
		t.Errorf("Sender = %q, want Mark Pugner", webhook.Sender)
	}
	if webhook.PaymentInfo != nil {
		t.Fatal("PaymentInfo should be nil for incoming boosts")
	}
}

func TestParseHelipadWebhookInvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := ParseHelipadWebhook([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestIsHelipadBoost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		webhook HelipadWebhook
		want    bool
	}{
		{
			name:    "incoming boost without payment info",
			webhook: HelipadWebhook{Direction: "incoming", Action: 2, PaymentInfo: nil},
			want:    true,
		},
		{
			name:    "legacy boost without direction",
			webhook: HelipadWebhook{Action: 2, PaymentInfo: nil},
			want:    true,
		},
		{
			name:    "stream action",
			webhook: HelipadWebhook{Direction: "incoming", Action: 1, PaymentInfo: nil},
			want:    false,
		},
		{
			name: "outgoing boost with payment info",
			webhook: HelipadWebhook{
				Direction: "outgoing",
				Action:    2,
				PaymentInfo: &HelipadPaymentInfo{
					PaymentHash: "abc123",
				},
			},
			want: false,
		},
		{
			name:    "outgoing direction without payment info",
			webhook: HelipadWebhook{Direction: "outgoing", Action: 2, PaymentInfo: nil},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := IsHelipadBoost(tt.webhook); got != tt.want {
				t.Errorf("IsHelipadBoost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHelipadWebhookToInvoice(t *testing.T) {
	t.Parallel()

	webhook := HelipadWebhook{
		Direction:      "incoming",
		Index:          7,
		Time:           1700000000,
		ValueMsat:      2500,
		ValueMsatTotal: 5000,
		Action:         2,
		Sender:         "Dave",
		App:            "CurioCaster",
		Message:        "hello",
		Podcast:        "Top Podcast",
		Episode:        "Top Episode",
		Memo:           "invoice memo",
	}

	tlv := Boostagram{Action: "boost", Podcast: "Helipad Podcast", SenderName: "Dave"}
	invoice := HelipadWebhookToInvoice(webhook, tlv)

	if invoice.Identifier != "helipad-7" {
		t.Errorf("Identifier = %q, want helipad-7", invoice.Identifier)
	}
	if invoice.PaymentHash != "helipad-7" {
		t.Errorf("PaymentHash = %q, want helipad-7", invoice.PaymentHash)
	}
	if invoice.Amount != 2.5 {
		t.Errorf("Amount = %v, want 2.5", invoice.Amount)
	}
	if invoice.Value != 2.5 {
		t.Errorf("Value = %v, want 2.5", invoice.Value)
	}
	if invoice.CreationDate != 1700000000 {
		t.Errorf("CreationDate = %v, want 1700000000", invoice.CreationDate)
	}
	if invoice.Comment != "hello" {
		t.Errorf("Comment = %q, want hello", invoice.Comment)
	}
	if invoice.Description != "invoice memo" {
		t.Errorf("Description = %q, want invoice memo", invoice.Description)
	}
	if invoice.PayerName != "Dave" {
		t.Errorf("PayerName = %q, want Dave", invoice.PayerName)
	}
	if invoice.Type != "incoming" {
		t.Errorf("Type = %q, want incoming", invoice.Type)
	}

	parsedTime, err := time.Parse(time.RFC3339, invoice.CreatedAt)
	if err != nil {
		t.Fatalf("CreatedAt %q is not valid RFC3339: %v", invoice.CreatedAt, err)
	}
	if parsedTime.Unix() != webhook.Time {
		t.Errorf("CreatedAt unix = %d, want %d", parsedTime.Unix(), webhook.Time)
	}

	if invoice.Boostagram == nil {
		t.Fatal("Boostagram should be set")
	}
	if invoice.Boostagram.Podcast != "Helipad Podcast" {
		t.Errorf("Boostagram.Podcast = %q, want Helipad Podcast", invoice.Boostagram.Podcast)
	}
	if invoice.Boostagram.ValueMsatTotal != 5000 {
		t.Errorf("Boostagram.ValueMsatTotal = %d, want 5000", invoice.Boostagram.ValueMsatTotal)
	}
}

func TestHelipadWebhookToInvoiceFillsMissingTLVFields(t *testing.T) {
	t.Parallel()

	webhook := HelipadWebhook{
		Index:          9,
		Time:           1700000000,
		ValueMsat:      1000,
		ValueMsatTotal: 2000,
		Action:         2,
		Sender:         "Alice",
		App:            "Fountain",
		Message:        "great episode",
		Podcast:        "From Webhook",
		Episode:        "Episode 1",
	}

	invoice := HelipadWebhookToInvoice(webhook, Boostagram{})

	if invoice.Boostagram.Podcast != "From Webhook" {
		t.Errorf("Boostagram.Podcast = %q, want From Webhook", invoice.Boostagram.Podcast)
	}
	if invoice.Boostagram.Episode != "Episode 1" {
		t.Errorf("Boostagram.Episode = %q, want Episode 1", invoice.Boostagram.Episode)
	}
	if invoice.Boostagram.SenderName != "Alice" {
		t.Errorf("Boostagram.SenderName = %q, want Alice", invoice.Boostagram.SenderName)
	}
	if invoice.Boostagram.AppName != "Fountain" {
		t.Errorf("Boostagram.AppName = %q, want Fountain", invoice.Boostagram.AppName)
	}
	if invoice.Boostagram.Message != "great episode" {
		t.Errorf("Boostagram.Message = %q, want great episode", invoice.Boostagram.Message)
	}
	if invoice.Boostagram.Action != "boost" {
		t.Errorf("Boostagram.Action = %q, want boost", invoice.Boostagram.Action)
	}
}

func TestParseHelipadTLV(t *testing.T) {
	t.Parallel()

	tlv := `{"action":"boost","podcast":"From TLV","sender_name":"Dave","feedId":1234567,"remote_feed_guid":"feed-guid","remote_item_guid":"item-guid"}`
	got, err := ParseHelipadTLV(tlv)
	if err != nil {
		t.Fatalf("ParseHelipadTLV() error = %v", err)
	}

	if got.Podcast != "From TLV" {
		t.Errorf("podcast = %q, want From TLV", got.Podcast)
	}
	if got.SenderName != "Dave" {
		t.Errorf("sender_name = %q, want Dave", got.SenderName)
	}
	if got.FeedID == nil || *got.FeedID != 1234567 {
		t.Errorf("feedID = %v, want 1234567", got.FeedID)
	}
	if got.RemoteFeedGuid != "feed-guid" {
		t.Errorf("remote_feed_guid = %q, want feed-guid", got.RemoteFeedGuid)
	}
}

func TestParseHelipadTLVEmpty(t *testing.T) {
	t.Parallel()

	got, err := ParseHelipadTLV("")
	if err != nil {
		t.Fatalf("ParseHelipadTLV() error = %v", err)
	}
	if got.Podcast != "" {
		t.Errorf("podcast = %q, want empty", got.Podcast)
	}
}

func TestParseHelipadTLVInvalid(t *testing.T) {
	t.Parallel()

	_, err := ParseHelipadTLV(`{bad json`)
	if err == nil {
		t.Fatal("expected error for invalid TLV JSON")
	}
}
