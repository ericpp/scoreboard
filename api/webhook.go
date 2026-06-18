package handler

import (
	"io"
	"log"
	"net/http"
	"os"

	"github.com/ericpp/scoreboard/common"
	svix "github.com/svix/svix-webhooks/go"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	wh, err := svix.NewWebhook(os.Getenv("ALBY_WEBHOOK"))
	if err != nil {
		log.Printf("failed to create webhook verifier: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		log.Print("Unable to read webhook payload")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := wh.Verify(payload, r.Header); err != nil {
		log.Print("Unable to verify webhook payload")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Printf("incoming webhook %s", payload)

	invoice, err := common.ParseInvoiceFromJson(payload)
	if err != nil {
		log.Printf("failed to parse invoice from json: %v", err)
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
