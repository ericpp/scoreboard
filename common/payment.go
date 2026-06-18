package common

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

func ParsePaymentNotification(payload []byte) (IncomingInvoice, error) {
	var notification PaymentNotification

	if err := json.Unmarshal(payload, &notification); err != nil {
		return IncomingInvoice{}, fmt.Errorf("failed to unmarshal NWC payload: %w", err)
	}

	if notification.Type != "payment_received" {
		return IncomingInvoice{}, fmt.Errorf("unsupported NWC notification type: %s", notification.Type)
	}

	if notification.Payment == nil {
		return IncomingInvoice{}, fmt.Errorf("payment is nil")
	}

	payment := notification.Payment

	if payment.Type != "incoming" {
		return IncomingInvoice{}, fmt.Errorf("unsupported payment type: %s", payment.Type)
	}

	if payment.State != "settled" {
		return IncomingInvoice{}, fmt.Errorf("unsupported payment state: %s", payment.State)
	}

	sats := float64(int(payment.Amount / 1000))
	invoice := IncomingInvoice{
		Amount:       sats,
		Value:        sats,
		Description:  payment.Description,
		PaymentHash:  payment.PaymentHash,
		Identifier:   payment.PaymentHash,
		Metadata:     payment.Metadata,
		Type:         payment.Type,
		CreationDate: float64(payment.CreatedAt),
		CreatedAt:    time.Unix(payment.CreatedAt, 0).UTC().Format(time.RFC3339),
	}

	applyInvoiceMetadata(&invoice)
	if invoice.Comment == "" {
		invoice.Comment = payment.Description
	}

	return invoice, nil
}

func ParseInvoiceFromJson(payload []byte) (IncomingInvoice, error) {
	var invoice IncomingInvoice

	if err := json.Unmarshal(payload, &invoice); err != nil {
		return IncomingInvoice{}, fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	applyInvoiceMetadata(&invoice)
	return invoice, nil
}

func applyInvoiceMetadata(invoice *IncomingInvoice) {
	if invoice.Metadata == nil {
		return
	}

	if invoice.Metadata.Comment != "" {
		invoice.Comment = invoice.Metadata.Comment
	}

	if invoice.Metadata.PayerData != nil && invoice.Metadata.PayerData.Name != "" {
		invoice.PayerName = invoice.Metadata.PayerData.Name
	}
}

func ParseBoostagramFromHex(hexValue string) (Boostagram, error) {
	decoded, err := hex.DecodeString(hexValue)
	if err != nil {
		return Boostagram{}, fmt.Errorf("failed to decode hex: %w", err)
	}

	var data Boostagram
	if err := json.Unmarshal(decoded, &data); err != nil {
		return Boostagram{}, fmt.Errorf("failed to unmarshal boostagram: %w", err)
	}

	return data, nil
}

func (i IncomingInvoice) GetBoostagram() Boostagram {
	if i.Boostagram != nil {
		return *i.Boostagram
	}

	if i.RSSPayment != nil {
		return i.RSSPayment.ParseBoostagram()
	}

	return Boostagram{}
}

func (i IncomingInvoice) GetSerializedMetadata() ([]byte, error) {
	var metadata interface{}

	if i.Boostagram != nil {
		metadata = i.Boostagram
	} else if i.RSSPayment != nil {
		metadata = i.RSSPayment
	}

	if metadata != nil {
		return json.Marshal(metadata)
	}

	return []byte("null"), nil
}

func (i IncomingInvoice) GetNostrRecord() NostrRecord {
	boostagram := i.GetBoostagram()

	return NostrRecord{
		Amount:       i.Amount,
		Boostagram:   &boostagram,
		Comment:      i.Comment,
		CreatedAt:    i.CreatedAt,
		CreationDate: i.CreationDate,
		Description:  i.Description,
		Identifier:   i.Identifier,
		PayerName:    i.PayerName,
		PaymentHash:  i.PaymentHash,
		Value:        i.Value,
	}
}

func (r RssPayment) ParseBoostagram() Boostagram {
	return Boostagram{
		Action:         strings.ToLower(r.Action),
		Podcast:        r.FeedTitle,
		Episode:        r.ItemTitle,
		AppName:        r.AppName,
		SenderName:     r.SenderName,
		Message:        r.Message,
		ValueMsatTotal: int(math.Floor(r.ValueMsatTotal)),
		Guid:           r.FeedGuid,
		EpisodeGuid:    r.ItemGuid,
		RemoteFeedGuid: r.RemoteFeedGuid,
		RemoteItemGuid: r.RemoteItemGuid,
		FeedID:         nil,
		ItemID:         nil,
		BlockGuid:      "",
		EventGuid:      "",
	}
}