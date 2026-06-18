package common

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func ParseHelipadWebhook(payload []byte) (HelipadWebhook, error) {
	var webhook HelipadWebhook
	if err := json.Unmarshal(payload, &webhook); err != nil {
		return HelipadWebhook{}, fmt.Errorf("failed to unmarshal webhook payload: %w", err)
	}

	return webhook, nil
}

func IsHelipadBoost(webhook HelipadWebhook) bool {
	if webhook.Action != 2 || webhook.PaymentInfo != nil {
		return false
	}

	return webhook.Direction == "" || webhook.Direction == "incoming"
}

func HelipadWebhookToInvoice(webhook HelipadWebhook, tlv Boostagram) IncomingInvoice {
	tm := time.Unix(webhook.Time, 0)
	amount := float64(webhook.ValueMsat) / 1000.0
	identifier := fmt.Sprintf("helipad-%d", webhook.Index)
	boostagram := mergeHelipadBoostagram(webhook, tlv)

	description := webhook.Memo
	if description == "" {
		description = webhook.Message
	}

	return IncomingInvoice{
		Amount:       amount,
		Boostagram:   &boostagram,
		Comment:      webhook.Message,
		CreatedAt:    tm.Format(time.RFC3339),
		CreationDate: float64(webhook.Time),
		Description:  description,
		Identifier:   identifier,
		PaymentHash:  identifier,
		PayerName:    webhook.Sender,
		Type:         "incoming",
		Value:        amount,
	}
}

func mergeHelipadBoostagram(webhook HelipadWebhook, tlv Boostagram) Boostagram {
	if tlv.Podcast == "" {
		tlv.Podcast = webhook.Podcast
	}
	if tlv.Episode == "" {
		tlv.Episode = webhook.Episode
	}
	if tlv.SenderName == "" {
		tlv.SenderName = webhook.Sender
	}
	if tlv.AppName == "" {
		tlv.AppName = webhook.App
	}
	if tlv.Message == "" {
		tlv.Message = webhook.Message
	}
	if tlv.ValueMsatTotal == 0 && webhook.ValueMsatTotal > 0 {
		tlv.ValueMsatTotal = int(webhook.ValueMsatTotal)
	}
	if tlv.Action == "" && webhook.Action == 2 {
		tlv.Action = "boost"
	}

	return tlv
}

func ParseHelipadTLV(tlv string) (Boostagram, error) {
	tlv = strings.TrimSpace(tlv)
	if tlv == "" {
		return Boostagram{}, nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(tlv), &raw); err != nil {
		return Boostagram{}, fmt.Errorf("failed to unmarshal TLV boostagram: %w", err)
	}

	if feedID, ok := raw["feedId"]; ok {
		raw["feedID"] = feedID
		delete(raw, "feedId")
	}

	normalized, err := json.Marshal(raw)
	if err != nil {
		return Boostagram{}, fmt.Errorf("failed to normalize TLV boostagram: %w", err)
	}

	var boostagram Boostagram
	if err := json.Unmarshal(normalized, &boostagram); err != nil {
		return Boostagram{}, fmt.Errorf("failed to unmarshal TLV boostagram: %w", err)
	}

	return boostagram, nil
}