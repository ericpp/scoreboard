package common

import (
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestParsePaymentNotification(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"type": "payment_received",
		"payment": {
			"type": "incoming",
			"state": "settled",
			"description": "rss::payment::boost https://example.com/pay",
			"payment_hash": "abc123",
			"amount": 100000,
			"created_at": 1700000000,
			"metadata": {
				"comment": "rss::payment::boost https://example.com/pay",
				"payer_data": {"name": "Alice", "email": "alice@example.com"}
			}
		}
	}`)

	invoice, err := ParsePaymentNotification(payload)
	if err != nil {
		t.Fatalf("ParsePaymentNotification() error = %v", err)
	}

	if invoice.Amount != 100 {
		t.Errorf("Amount = %v, want 100 (msats converted to sats)", invoice.Amount)
	}

	if invoice.Comment != "rss::payment::boost https://example.com/pay" {
		t.Errorf("Comment = %q, want rss payment comment", invoice.Comment)
	}

	if invoice.PayerName != "Alice" {
		t.Errorf("PayerName = %q, want Alice", invoice.PayerName)
	}

	if invoice.PaymentHash != "abc123" {
		t.Errorf("PaymentHash = %q, want abc123", invoice.PaymentHash)
	}

	if invoice.Identifier != "abc123" {
		t.Errorf("Identifier = %q, want abc123", invoice.Identifier)
	}

	if invoice.CreatedAt == "" {
		t.Fatal("CreatedAt should be set from payment.created_at")
	}

	if invoice.CreationDate != 1700000000 {
		t.Errorf("CreationDate = %v, want 1700000000", invoice.CreationDate)
	}
}

func TestParsePaymentNotificationUsesCreatedAt(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"type": "payment_received",
		"payment": {
			"type": "incoming",
			"state": "settled",
			"payment_hash": "abc123",
			"amount": 5000,
			"created_at": 1700000000
		}
	}`)

	invoice, err := ParsePaymentNotification(payload)
	if err != nil {
		t.Fatalf("ParsePaymentNotification() error = %v", err)
	}

	want := "2023-11-14T22:13:20Z"
	if invoice.CreatedAt != want {
		t.Errorf("CreatedAt = %q, want %q", invoice.CreatedAt, want)
	}

	if invoice.Amount != 5 {
		t.Errorf("Amount = %v, want 5", invoice.Amount)
	}
}

func TestParsePaymentNotificationInvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := ParsePaymentNotification([]byte(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseInvoiceFromJson(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"amount": 5,
		"boostagram": null,
		"comment": "rss::payment::boost https://fountain.fm/episode/XOmI50tCRMgJFQat53UI?payment=jvwdRRi7hueCz5PyVd62 test",
		"created_at": "2026-06-18 00:31:08 UTC",
		"creation_date": 1781742669,
		"identifier": "fCxA3nx7NFM1thX9vNe5CTN9",
		"metadata": {
			"comment": "rss::payment::boost https://fountain.fm/episode/XOmI50tCRMgJFQat53UI?payment=jvwdRRi7hueCz5PyVd62 test"
		},
		"payment_hash": "c55f9025c8d2615852cd3d5d2c696f1078407d9b5fb78b0f31685ceb3f2afe09",
		"settled": true,
		"state": "SETTLED",
		"type": "incoming",
		"value": 5
	}`)

	invoice, err := ParseInvoiceFromJson(payload)
	if err != nil {
		t.Fatalf("ParseInvoiceFromJson() error = %v", err)
	}

	if invoice.Amount != 5 {
		t.Errorf("Amount = %v, want 5", invoice.Amount)
	}

	if invoice.Value != 5 {
		t.Errorf("Value = %v, want 5", invoice.Value)
	}

	if invoice.Identifier != "fCxA3nx7NFM1thX9vNe5CTN9" {
		t.Errorf("Identifier = %q, want fCxA3nx7NFM1thX9vNe5CTN9", invoice.Identifier)
	}

	if invoice.PaymentHash != "c55f9025c8d2615852cd3d5d2c696f1078407d9b5fb78b0f31685ceb3f2afe09" {
		t.Errorf("PaymentHash = %q, want c55f9025...", invoice.PaymentHash)
	}

	if invoice.CreationDate != 1781742669 {
		t.Errorf("CreationDate = %v, want 1781742669", invoice.CreationDate)
	}

	if invoice.Comment == "" {
		t.Fatal("Comment should be set from webhook payload")
	}
}

func TestGetBoostagramFromDirectBoostagram(t *testing.T) {
	t.Parallel()

	feedID := 123.0
	invoice := IncomingInvoice{
		Boostagram: &Boostagram{
			Action:  "boost",
			Podcast: "Direct Podcast",
			FeedID:  &feedID,
		},
	}

	got := invoice.GetBoostagram()
	if got.Podcast != "Direct Podcast" {
		t.Errorf("Podcast = %q, want Direct Podcast", got.Podcast)
	}
}

func TestGetBoostagramFromRSSPayment(t *testing.T) {
	t.Parallel()

	invoice := IncomingInvoice{
		RSSPayment: &RssPayment{
			Action:         "BOOST",
			FeedTitle:      "RSS Podcast",
			ItemTitle:      "Episode 1",
			AppName:        "Fountain",
			SenderName:     "Bob",
			Message:        "Nice episode",
			ValueMsatTotal: 10000.7,
			FeedGuid:       "feed-guid",
			ItemGuid:       "item-guid",
		},
	}

	got := invoice.GetBoostagram()
	if got.Action != "boost" {
		t.Errorf("Action = %q, want boost", got.Action)
	}
	if got.Podcast != "RSS Podcast" {
		t.Errorf("Podcast = %q, want RSS Podcast", got.Podcast)
	}
	if got.Episode != "Episode 1" {
		t.Errorf("Episode = %q, want Episode 1", got.Episode)
	}
	if got.ValueMsatTotal != 10000 {
		t.Errorf("ValueMsatTotal = %d, want 10000", got.ValueMsatTotal)
	}
}

func TestGetBoostagramEmpty(t *testing.T) {
	t.Parallel()

	got := IncomingInvoice{}.GetBoostagram()
	if got != (Boostagram{}) {
		t.Errorf("GetBoostagram() = %+v, want zero value", got)
	}
}

func TestGetSerializedMetadata(t *testing.T) {
	t.Parallel()

	t.Run("boostagram", func(t *testing.T) {
		t.Parallel()

		invoice := IncomingInvoice{
			Boostagram: &Boostagram{Action: "boost", Message: "hello"},
		}

		data, err := invoice.GetSerializedMetadata()
		if err != nil {
			t.Fatalf("GetSerializedMetadata() error = %v", err)
		}

		var parsed Boostagram
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("failed to unmarshal metadata: %v", err)
		}

		if parsed.Message != "hello" {
			t.Errorf("Message = %q, want hello", parsed.Message)
		}
	})

	t.Run("no metadata", func(t *testing.T) {
		t.Parallel()

		data, err := IncomingInvoice{}.GetSerializedMetadata()
		if err != nil {
			t.Fatalf("GetSerializedMetadata() error = %v", err)
		}

		if string(data) != "null" {
			t.Errorf("metadata = %s, want null", data)
		}
	})
}

func TestGetNostrRecord(t *testing.T) {
	t.Parallel()

	invoice := IncomingInvoice{
		Amount:      21,
		PaymentHash: "hash-1",
		Boostagram:  &Boostagram{Podcast: "Nostr Podcast"},
	}

	record := invoice.GetNostrRecord()
	if record.Amount != 21 {
		t.Errorf("Amount = %v, want 21", record.Amount)
	}
	if record.PaymentHash != "hash-1" {
		t.Errorf("PaymentHash = %q, want hash-1", record.PaymentHash)
	}
	if record.Boostagram == nil || record.Boostagram.Podcast != "Nostr Podcast" {
		t.Errorf("Boostagram = %+v, want Nostr Podcast", record.Boostagram)
	}
}

func TestParseBoostagramFromHex(t *testing.T) {
	t.Parallel()

	original := Boostagram{
		Action:  "boost",
		Podcast: "Hex Podcast",
		Message: "from hex",
	}
	raw, _ := json.Marshal(original)
	encoded := hex.EncodeToString(raw)

	got, err := ParseBoostagramFromHex(encoded)
	if err != nil {
		t.Fatalf("ParseBoostagramFromHex() error = %v", err)
	}

	if got.Podcast != "Hex Podcast" {
		t.Errorf("Podcast = %q, want Hex Podcast", got.Podcast)
	}
}

func TestParseBoostagramFromHexInvalid(t *testing.T) {
	t.Parallel()

	_, err := ParseBoostagramFromHex("not-hex")
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}
}