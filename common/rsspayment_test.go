package common

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestExtractRSSPaymentURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		comment string
		want    string
	}{
		{
			name:    "boost url",
			comment: "rss::payment::boost https://fountain.fm/api/v1/payment/abc123",
			want:    "https://fountain.fm/api/v1/payment/abc123",
		},
		{
			name:    "stream url with prefix text",
			comment: "Payment for episode rss::payment::stream http://example.com/pay/xyz extra",
			want:    "http://example.com/pay/xyz",
		},
		{
			name:    "no rss payment tag",
			comment: "just a regular comment https://example.com",
			want:    "",
		},
		{
			name:    "empty comment",
			comment: "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ExtractRSSPaymentURL(tt.comment)
			if got != tt.want {
				t.Errorf("ExtractRSSPaymentURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFetchRSSPaymentWithClient(t *testing.T) {
	t.Parallel()

	rssJSON := `{"action":"boost","feed_title":"Test Feed","item_title":"Test Episode","value_msat_total":5000}`
	encoded := url.QueryEscape(rssJSON)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("method = %s, want HEAD", r.Method)
		}
		w.Header().Set("x-rss-payment", encoded)
	}))
	defer server.Close()

	client := server.Client()
	got, err := FetchRSSPaymentWithClient(client, server.URL)
	if err != nil {
		t.Fatalf("FetchRSSPaymentWithClient() error = %v", err)
	}

	if got.FeedTitle != "Test Feed" {
		t.Errorf("FeedTitle = %q, want Test Feed", got.FeedTitle)
	}
	if got.ItemTitle != "Test Episode" {
		t.Errorf("ItemTitle = %q, want Test Episode", got.ItemTitle)
	}
}

func TestFetchRSSPaymentWithClientMissingHeader(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	_, err := FetchRSSPaymentWithClient(server.Client(), server.URL)
	if err == nil {
		t.Fatal("expected error when x-rss-payment header is missing")
	}
}

func TestFetchRSSPaymentIfNeeded(t *testing.T) {
	t.Parallel()

	rssJSON := `{"action":"boost","feed_title":"Fetched Feed","sender_name":"Carol"}`
	encoded := url.QueryEscape(rssJSON)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-rss-payment", encoded)
	}))
	defer server.Close()

	invoice := IncomingInvoice{
		Comment: "rss::payment::boost " + server.URL,
	}

	if err := FetchRSSPaymentIfNeeded(&invoice); err != nil {
		t.Fatalf("FetchRSSPaymentIfNeeded() error = %v", err)
	}

	if invoice.RSSPayment == nil {
		t.Fatal("RSSPayment should be populated")
	}

	if invoice.RSSPayment.FeedTitle != "Fetched Feed" {
		t.Errorf("FeedTitle = %q, want Fetched Feed", invoice.RSSPayment.FeedTitle)
	}

	boostagram := invoice.GetBoostagram()
	if boostagram.SenderName != "Carol" {
		t.Errorf("SenderName = %q, want Carol", boostagram.SenderName)
	}
}

func TestFetchRSSPaymentIfNeededSkipsWhenAlreadyPresent(t *testing.T) {
	t.Parallel()

	existing := &RssPayment{FeedTitle: "Already Set"}
	invoice := IncomingInvoice{
		Comment:    "rss::payment::boost https://should-not-be-called.example",
		RSSPayment: existing,
	}

	if err := FetchRSSPaymentIfNeeded(&invoice); err != nil {
		t.Fatalf("FetchRSSPaymentIfNeeded() error = %v", err)
	}

	if invoice.RSSPayment != existing {
		t.Fatal("RSSPayment should remain unchanged")
	}
}

func TestRssPaymentParseBoostagram(t *testing.T) {
	t.Parallel()

	rss := RssPayment{
		Action:         "STREAM",
		FeedTitle:      "Show",
		ItemTitle:      "Ep",
		ValueMsatTotal: 999.9,
		RemoteFeedGuid: "remote-feed",
		RemoteItemGuid: "remote-item",
	}

	got := rss.ParseBoostagram()
	if got.Action != "stream" {
		t.Errorf("Action = %q, want stream", got.Action)
	}
	if got.ValueMsatTotal != 999 {
		t.Errorf("ValueMsatTotal = %d, want 999", got.ValueMsatTotal)
	}
	if got.RemoteFeedGuid != "remote-feed" {
		t.Errorf("RemoteFeedGuid = %q, want remote-feed", got.RemoteFeedGuid)
	}
}
