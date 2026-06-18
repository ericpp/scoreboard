package common

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func openDB() (*sql.DB, error) {
	db, err := sql.Open("postgres", os.Getenv("POSTGRES_URL"))
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func SaveInvoice(invoice IncomingInvoice) error {
	_, err := saveInvoice(invoice)
	return err
}

func SaveInvoiceIfNew(invoice IncomingInvoice) (bool, error) {
	return saveInvoice(invoice)
}

func saveInvoice(invoice IncomingInvoice) (bool, error) {
	db, err := openDB()
	if err != nil {
		return false, err
	}
	defer db.Close()

	log.Printf("inserting %s", invoice.PaymentHash)

	serializedMetadata, err := invoice.GetSerializedMetadata()
	if err != nil {
		log.Printf("failed to serialize boostagram for invoice %s: %v", invoice.PaymentHash, err)
	}

	boostagram := invoice.GetBoostagram()
	insertSQL :=
		`INSERT INTO invoices
        (amount, boostagram, comment, created_at, creation_date, description, identifier, payer_name, payment_hash, value, podcast, episode, app_name, sender_name, message, value_msat_total, feed_id, item_id, guid, episode_guid, action, event_guid)
    VALUES
        ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)
    ON CONFLICT (payment_hash) DO NOTHING`

	result, err := db.Exec(
		insertSQL,
		invoice.Amount,
		serializedMetadata,
		invoice.Comment,
		invoice.CreatedAt,
		invoice.CreationDate,
		invoice.Description,
		invoice.Identifier,
		invoice.PayerName,
		invoice.PaymentHash,
		invoice.Value,
		boostagram.Podcast,
		boostagram.Episode,
		boostagram.AppName,
		boostagram.SenderName,
		boostagram.Message,
		boostagram.ValueMsatTotal,
		boostagram.FeedID,
		boostagram.ItemID,
		boostagram.Guid,
		boostagram.EpisodeGuid,
		boostagram.Action,
		boostagram.EventGuid,
	)

	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rowsAffected > 0, nil
}

func UpdateDatabaseWithRSSPayment(invoice IncomingInvoice) error {
	if invoice.RSSPayment == nil {
		return nil
	}

	db, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	log.Printf("updating %s with RSS payment info", invoice.PaymentHash)

	serializedMetadata, err := invoice.GetSerializedMetadata()
	if err != nil {
		log.Printf("failed to serialize boostagram for invoice %s: %v", invoice.PaymentHash, err)
		return err
	}

	boostagram := invoice.GetBoostagram()
	updateSQL :=
		`UPDATE invoices SET
        boostagram = $1,
        podcast = $2,
        episode = $3,
        app_name = $4,
        sender_name = $5,
        message = $6,
        value_msat_total = $7,
        feed_id = $8,
        item_id = $9,
        guid = $10,
        episode_guid = $11,
        action = $12,
        event_guid = $13
    WHERE payment_hash = $14`

	_, err = db.Exec(
		updateSQL,
		serializedMetadata,
		boostagram.Podcast,
		boostagram.Episode,
		boostagram.AppName,
		boostagram.SenderName,
		boostagram.Message,
		boostagram.ValueMsatTotal,
		boostagram.FeedID,
		boostagram.ItemID,
		boostagram.Guid,
		boostagram.EpisodeGuid,
		boostagram.Action,
		boostagram.EventGuid,
		invoice.PaymentHash,
	)

	return err
}