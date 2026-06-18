package common

type PaymentNotification struct {
	Type    string     `json:"type"`
	Payment *NWCPayment `json:"payment"`
}

type NWCPayment struct {
	Type            string           `json:"type"`
	State           string           `json:"state"`
	Invoice         string           `json:"invoice"`
	Description     string           `json:"description"`
	DescriptionHash string           `json:"description_hash"`
	Preimage        string           `json:"preimage"`
	PaymentHash     string           `json:"payment_hash"`
	Amount          float64          `json:"amount"`
	FeesPaid        float64          `json:"fees_paid"`
	CreatedAt       int64            `json:"created_at"`
	ExpiresAt       int64            `json:"expires_at"`
	SettledAt       int64            `json:"settled_at"`
	Metadata        *InvoiceMetadata `json:"metadata"`
}

type IncomingInvoice struct {
	Amount       float64          `json:"amount"`
	Boostagram   *Boostagram      `json:"boostagram"`
	Comment      string           `json:"comment"`
	CreatedAt    string           `json:"created_at"`
	CreationDate float64          `json:"creation_date"`
	Description  string           `json:"description"`
	Identifier   string           `json:"identifier"`
	Metadata     *InvoiceMetadata `json:"metadata"`
	PayerName    string           `json:"payer_name"`
	PaymentHash  string           `json:"payment_hash"`
	Type         string           `json:"type"`
	Value        float64          `json:"value"`
	RSSPayment   *RssPayment
}

type InvoiceMetadata struct {
	Comment    string             `json:"comment"`
	PayerData  *InvoicePayerData  `json:"payer_data"`
	TLVRecords []InvoiceTLVRecord `json:"tlv_records"`
}

type InvoicePayerData struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type InvoiceTLVRecord struct {
	Type  int64  `json:"type"`
	Value string `json:"value"`
}

type NostrRecord struct {
	Amount       float64     `json:"amount"`
	Boostagram   *Boostagram `json:"boostagram"`
	Comment      string      `json:"comment"`
	CreatedAt    string      `json:"created_at"`
	CreationDate float64     `json:"creation_date"`
	Description  string      `json:"description"`
	Identifier   string      `json:"identifier"`
	PayerName    string      `json:"payer_name"`
	PaymentHash  string      `json:"payment_hash"`
	Value        float64     `json:"value"`
}

type Boostagram struct {
	Action         string   `json:"action"`
	Podcast        string   `json:"podcast"`
	Episode        string   `json:"episode"`
	AppName        string   `json:"app_name"`
	SenderName     string   `json:"sender_name"`
	Message        string   `json:"message"`
	ValueMsatTotal int      `json:"value_msat_total"`
	FeedID         *float64 `json:"feedID"`
	ItemID         *float64 `json:"itemID"`
	Guid           string   `json:"guid"`
	EpisodeGuid    string   `json:"episode_guid"`
	BlockGuid      string   `json:"blockGuid"`
	EventGuid      string   `json:"eventGuid"`
	RemoteFeedGuid string   `json:"remote_feed_guid"`
	RemoteItemGuid string   `json:"remote_item_guid"`
}

type RssPayment struct {
	Action              string  `json:"action"`
	AppName             string  `json:"app_name"`
	FeedGuid            string  `json:"feed_guid"`
	FeedTitle           string  `json:"feed_title"`
	Group               string  `json:"group"`
	Id                  string  `json:"id"`
	ItemGuid            string  `json:"item_guid"`
	ItemTitle           string  `json:"item_title"`
	Link                string  `json:"link"`
	Message             string  `json:"message"`
	Position            int     `json:"position"`
	PublisherGuid       string  `json:"publisher_guid"`
	PublisherTitle      string  `json:"publisher_title"`
	RecipientAddress    string  `json:"recipient_address"`
	RemoteFeedGuid      string  `json:"remote_feed_guid"`
	RemoteItemGuid      string  `json:"remote_item_guid"`
	RemotePublisherGuid string  `json:"remote_publisher_guid"`
	SenderId            string  `json:"sender_id"`
	SenderName          string  `json:"sender_name"`
	SenderNpub          string  `json:"sender_npub"`
	Split               float64 `json:"split"`
	Timestamp           string  `json:"timestamp"`
	ValueMsat           float64 `json:"value_msat"`
	ValueMsatTotal      float64 `json:"value_msat_total"`
	ValueUsd            float64 `json:"value_usd"`
}

type HelipadPaymentInfo struct {
	PaymentHash string `json:"payment_hash"`
	Pubkey      string `json:"pubkey"`
	CustomKey   int64  `json:"custom_key"`
	CustomValue string `json:"custom_value"`
	FeeMsat     int64  `json:"fee_msat"`
	ReplyToIdx  *int64 `json:"reply_to_idx"`
}

type HelipadWebhook struct {
	Direction      string              `json:"direction"`
	Index          int64               `json:"index"`
	Time           int64               `json:"time"`
	ValueMsat      int64               `json:"value_msat"`
	ValueMsatTotal int64               `json:"value_msat_total"`
	Action         int8                `json:"action"`
	Sender         string              `json:"sender"`
	App            string              `json:"app"`
	Message        string              `json:"message"`
	Podcast        string              `json:"podcast"`
	Episode        string              `json:"episode"`
	Tlv            string              `json:"tlv"`
	RemotePodcast  string              `json:"remote_podcast"`
	RemoteEpisode  string              `json:"remote_episode"`
	ReplySent      bool                `json:"reply_sent"`
	Memo           string              `json:"memo"`
	PaymentInfo    *HelipadPaymentInfo `json:"payment_info"`
}