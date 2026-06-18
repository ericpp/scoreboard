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
	status, ok := common.ValidateBearerToken(authHeader, os.Getenv("NWC_WEBHOOK_TOKEN"))
	if !ok {
		switch status {
		case http.StatusInternalServerError:
			log.Print("NWC_WEBHOOK_TOKEN environment variable not set")
		case http.StatusUnauthorized:
			if authHeader == "" {
				log.Print("Missing Authorization header")
			} else {
				log.Print("Invalid bearer token")
			}
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

	log.Printf("incoming webhook %s", payload)

	invoice, err := common.ParsePaymentNotification(payload)
	if err != nil {
		log.Printf("failed to parse payment notification: %v", err)
		log.Printf("payload: %s", payload)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	isNewPayment, err := common.SaveInvoiceIfNew(invoice)
	if err != nil {
		log.Printf("failed to save to database: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !isNewPayment {
		log.Printf("payment %s already exists in database, skipping broadcast", invoice.PaymentHash)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := common.FetchRSSPaymentIfNeeded(&invoice); err != nil {
		log.Printf("failed to fetch RSS payment: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if invoice.RSSPayment != nil {
		if err := common.UpdateDatabaseWithRSSPayment(invoice); err != nil {
			log.Printf("failed to update database with RSS payment: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	if err := common.PublishInvoiceToNostr(invoice); err != nil {
		log.Printf("failed to publish to nostr: %v", err)
	}

	w.WriteHeader(http.StatusNoContent)
}
