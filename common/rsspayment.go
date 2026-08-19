package common

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func ExtractRSSPaymentURL(comment string) string {
	idx := strings.Index(comment, "rss::payment::")
	if idx == -1 {
		return ""
	}

	remaining := comment[idx:]
	parts := strings.Fields(remaining)

	for _, part := range parts {
		if strings.HasPrefix(part, "http://") || strings.HasPrefix(part, "https://") {
			return part
		}
	}

	return ""
}

func FetchRSSPaymentIfNeeded(invoice *IncomingInvoice) error {
	if invoice.RSSPayment != nil {
		return nil
	}

	paymentURL := ExtractRSSPaymentURL(invoice.Comment)
	if paymentURL == "" {
		return nil
	}

	rssPayment, err := FetchRSSPayment(paymentURL)
	if err != nil {
		return fmt.Errorf("failed to fetch RSS payment boostagram: %w", err)
	}

	invoice.RSSPayment = &rssPayment
	return nil
}

func FetchRSSPayment(paymentURL string) (RssPayment, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return FetchRSSPaymentWithRetry(client, paymentURL)
}

func FetchRSSPaymentWithRetry(client *http.Client, paymentURL string) (RssPayment, error) {
	rssPayment, err, retry := fetchRSSPaymentWithClient(client, paymentURL)
	if err == nil || !retry {
		return rssPayment, err
	}

	time.Sleep(500 * time.Millisecond)
	rssPayment, err, _ = fetchRSSPaymentWithClient(client, paymentURL)
	return rssPayment, err
}

func fetchRSSPaymentWithClient(client *http.Client, paymentURL string) (RssPayment, error, bool) {
	resp, err := client.Head(paymentURL)
	if err != nil {
		return RssPayment{}, fmt.Errorf("failed to fetch URL: %w", err), true
	}
	defer resp.Body.Close()

	rssPaymentHeader := resp.Header.Get("x-rss-payment")
	if rssPaymentHeader == "" {
		return RssPayment{}, fmt.Errorf("x-rss-payment header not found"), true
	}

	decodedHeader, err := url.QueryUnescape(rssPaymentHeader)
	if err != nil {
		return RssPayment{}, fmt.Errorf("failed to decode x-rss-payment header: %w", err), false
	}

	var rssPayment RssPayment
	if err := json.Unmarshal([]byte(decodedHeader), &rssPayment); err != nil {
		return RssPayment{}, fmt.Errorf("failed to parse x-rss-payment header: %w", err), false
	}

	return rssPayment, nil, false
}
