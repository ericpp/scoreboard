package handler

import (
	"io"
	"log"
	"net/http"
	"os"

	"github.com/ericpp/scoreboard/common"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	status, ok := common.ValidateHelipadToken(authHeader, os.Getenv("HELIPAD_TOKEN"))
	if !ok {
		if authHeader == "" {
			log.Print("Missing Authorization header")
		} else {
			log.Print("Authorization token does not match")
		}
		w.WriteHeader(status)
		return
	}

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		log.Print("Unable to read webhook payload")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	webhook, err := common.ParseHelipadWebhook(payload)
	if err != nil {
		log.Printf("failed to unmarshal webhook payload: %v", err)
		log.Printf("payload: %s", payload)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !common.IsHelipadBoost(webhook) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	boostagram, err := common.ParseHelipadTLV(webhook.Tlv)
	if err != nil {
		log.Printf("failed to unmarshal TLV boostagram: %v", err)
		log.Printf("payload: %s", payload)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	invoice := common.HelipadWebhookToInvoice(webhook, boostagram)

	if err := common.SaveInvoice(invoice); err != nil {
		log.Printf("failed to save boost to database: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if webhook.RemotePodcast != "" && webhook.RemoteEpisode != "" {
		remoteItemNames := common.RemoteItemNames{
			Podcast: webhook.RemotePodcast,
			Episode: webhook.RemoteEpisode,
		}

		if err := common.UpdateRemoteItemNames(invoice.PaymentHash, remoteItemNames); err != nil {
			log.Printf("failed to update remote item names: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
