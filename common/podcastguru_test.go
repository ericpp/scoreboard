package common

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractPodcastGuruURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		comment string
		want    string
	}{
		{
			name:    "basic V4V url",
			comment: "V4V: https://boost.podcastguru.io/boost/19iyHHzzf",
			want:    "https://boost.podcastguru.io/boost/19iyHHzzf",
		},
		{
			name:    "V4V url with trailing payment::stream tag",
			comment: "V4V: https://boost.podcastguru.io/boost/19iyHHzzf payment::stream",
			want:    "https://boost.podcastguru.io/boost/19iyHHzzf",
		},
		{
			name:    "V4V url with trailing payment::boost tag",
			comment: "V4V: https://boost.podcastguru.io/boost/19iyHHzzf payment::boost",
			want:    "https://boost.podcastguru.io/boost/19iyHHzzf",
		},
		{
			name:    "V4V url with no trailing payment tag",
			comment: "V4V: https://boost.podcastguru.io/boost/19iyHHzzf",
			want:    "https://boost.podcastguru.io/boost/19iyHHzzf",
		},
		{
			name:    "V4V url with prefix text and trailing text",
			comment: "Great episode V4V: https://boost.podcastguru.io/boost/abc123 keep it up!",
			want:    "https://boost.podcastguru.io/boost/abc123",
		},
		{
			name:    "no V4V tag",
			comment: "just a regular comment https://boost.podcastguru.io/boost/xyz",
			want:    "",
		},
		{
			name:    "V4V tag without matching url",
			comment: "V4V: https://example.com/other",
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

			got := ExtractPodcastGuruURL(tt.comment)
			if got != tt.want {
				t.Errorf("ExtractPodcastGuruURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

const samplePodcastGuruResponse = `{"boost_amount":"314.00000000","boost_currency":"sats","boost_hash":"19iyHHzzf","boost_id":10380,"created_at":1786824523,"episode_id":"1780cecb9cf2b56061b8295ebafbdfe4","metadata_payload":"{\"boost_link\":\"https:\\/\\/app.podcastguru.io\\/podcast\\/1584274529\\/episode\\/305228204?t=1646\",\"app_name\":\"PodcastGuru\",\"speed\":\"2\",\"episode\":\"Episode 267: Chocolove\",\"total_amount\":314,\"app_version\":\"1.0.38 (132)\",\"sender_name\":\"Bryan Entzminger\",\"ts\":1646,\"currency\":\"sats\",\"action\":\"stream\",\"podcast\":\"Podcasting 2.0\",\"url\":\"http:\\/\\/mp3s.nashownotes.com\\/pc20rss.xml\",\"sender_id\":\"BE2F36DE-9DE2-462B-8988-1469535C554F\",\"value_msat_total\":314000,\"episode_guid\":\"PC20-267\",\"guid\":\"917393e3-1b1e-5cef-ace4-edaa54e1f810\"}","podcast_id":"1584274529","sender_id":"BE2F36DE-9DE2-462B-8988-1469535C554F","url":"https://boost.podcastguru.io/boost/19iyHHzzf"}`

func TestFetchPodcastGuruPaymentWithClient(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(samplePodcastGuruResponse))
	}))
	defer server.Close()

	got, err := FetchPodcastGuruPaymentWithClient(server.Client(), server.URL)
	if err != nil {
		t.Fatalf("FetchPodcastGuruPaymentWithClient() error = %v", err)
	}

	if got.BoostHash != "19iyHHzzf" {
		t.Errorf("BoostHash = %q, want 19iyHHzzf", got.BoostHash)
	}
	if got.BoostId != 10380 {
		t.Errorf("BoostId = %d, want 10380", got.BoostId)
	}
	if got.PodcastId != "1584274529" {
		t.Errorf("PodcastId = %q, want 1584274529", got.PodcastId)
	}
	if got.ParsedMetadata == nil {
		t.Fatal("ParsedMetadata should be populated")
	}
	if got.ParsedMetadata.Podcast != "Podcasting 2.0" {
		t.Errorf("Podcast = %q, want Podcasting 2.0", got.ParsedMetadata.Podcast)
	}
	if got.ParsedMetadata.Episode != "Episode 267: Chocolove" {
		t.Errorf("Episode = %q, want Episode 267: Chocolove", got.ParsedMetadata.Episode)
	}
	if got.ParsedMetadata.EpisodeGuid != "PC20-267" {
		t.Errorf("EpisodeGuid = %q, want PC20-267", got.ParsedMetadata.EpisodeGuid)
	}
	if got.ParsedMetadata.Guid != "917393e3-1b1e-5cef-ace4-edaa54e1f810" {
		t.Errorf("Guid = %q, want 917393e3-1b1e-5cef-ace4-edaa54e1f810", got.ParsedMetadata.Guid)
	}
	if got.ParsedMetadata.Action != "stream" {
		t.Errorf("Action = %q, want stream", got.ParsedMetadata.Action)
	}
	if got.ParsedMetadata.AppName != "PodcastGuru" {
		t.Errorf("AppName = %q, want PodcastGuru", got.ParsedMetadata.AppName)
	}
	if got.ParsedMetadata.SenderName != "Bryan Entzminger" {
		t.Errorf("SenderName = %q, want Bryan Entzminger", got.ParsedMetadata.SenderName)
	}
	if got.ParsedMetadata.ValueMsatTotal != 314000 {
		t.Errorf("ValueMsatTotal = %d, want 314000", got.ParsedMetadata.ValueMsatTotal)
	}
	if got.ParsedMetadata.Ts != 1646 {
		t.Errorf("Ts = %d, want 1646", got.ParsedMetadata.Ts)
	}
	if got.ParsedMetadata.Speed != "2" {
		t.Errorf("Speed = %q, want 2", got.ParsedMetadata.Speed)
	}
}

func TestFetchPodcastGuruPaymentWithClientInvalidJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{not valid json`))
	}))
	defer server.Close()

	_, err := FetchPodcastGuruPaymentWithClient(server.Client(), server.URL)
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestFetchPodcastGuruPaymentIfNeededSkipsWhenNoURL(t *testing.T) {
	t.Parallel()

	invoice := IncomingInvoice{
		Comment: "rss::payment::stream https://example.com/pay",
	}

	if err := FetchPodcastGuruPaymentIfNeeded(&invoice); err != nil {
		t.Fatalf("FetchPodcastGuruPaymentIfNeeded() error = %v", err)
	}

	if invoice.PodcastGuruPayment != nil {
		t.Fatal("PodcastGuruPayment should remain nil when comment has no V4V URL")
	}
}

func TestFetchPodcastGuruPaymentIfNeededSkipsWhenAlreadyPresent(t *testing.T) {
	t.Parallel()

	existing := &PodcastGuruPayment{BoostHash: "already-set"}
	invoice := IncomingInvoice{
		Comment:            "V4V: https://boost.podcastguru.io/boost/should-not-be-called",
		PodcastGuruPayment: existing,
	}

	if err := FetchPodcastGuruPaymentIfNeeded(&invoice); err != nil {
		t.Fatalf("FetchPodcastGuruPaymentIfNeeded() error = %v", err)
	}

	if invoice.PodcastGuruPayment != existing {
		t.Fatal("PodcastGuruPayment should remain unchanged")
	}
}

func TestPodcastGuruPaymentParseBoostagram(t *testing.T) {
	t.Parallel()

	payment := PodcastGuruPayment{
		ParsedMetadata: &PodcastGuruMetadata{
			Action:         "STREAM",
			Podcast:        "Podcasting 2.0",
			Episode:        "Episode 267: Chocolove",
			EpisodeGuid:    "PC20-267",
			AppName:        "PodcastGuru",
			SenderName:     "Bryan Entzminger",
			Message:        "hello",
			Guid:           "feed-guid",
			ValueMsatTotal: 314000,
		},
	}

	got := payment.ParseBoostagram()
	if got.Action != "stream" {
		t.Errorf("Action = %q, want stream (lowercased)", got.Action)
	}
	if got.Podcast != "Podcasting 2.0" {
		t.Errorf("Podcast = %q, want Podcasting 2.0", got.Podcast)
	}
	if got.Episode != "Episode 267: Chocolove" {
		t.Errorf("Episode = %q, want Episode 267: Chocolove", got.Episode)
	}
	if got.EpisodeGuid != "PC20-267" {
		t.Errorf("EpisodeGuid = %q, want PC20-267", got.EpisodeGuid)
	}
	if got.Guid != "feed-guid" {
		t.Errorf("Guid = %q, want feed-guid", got.Guid)
	}
	if got.AppName != "PodcastGuru" {
		t.Errorf("AppName = %q, want PodcastGuru", got.AppName)
	}
	if got.SenderName != "Bryan Entzminger" {
		t.Errorf("SenderName = %q, want Bryan Entzminger", got.SenderName)
	}
	if got.Message != "hello" {
		t.Errorf("Message = %q, want hello", got.Message)
	}
	if got.ValueMsatTotal != 314000 {
		t.Errorf("ValueMsatTotal = %d, want 314000", got.ValueMsatTotal)
	}
}

func TestPodcastGuruPaymentParseBoostagramFromRawPayload(t *testing.T) {
	t.Parallel()

	payment := PodcastGuruPayment{
		MetadataPayload: `{"action":"boost","podcast":"Test Show","episode":"Ep 1","episode_guid":"ep-1","sender_name":"Alice","value_msat_total":5000,"guid":"feed-1"}`,
	}

	got := payment.ParseBoostagram()
	if got.Action != "boost" {
		t.Errorf("Action = %q, want boost", got.Action)
	}
	if got.Podcast != "Test Show" {
		t.Errorf("Podcast = %q, want Test Show", got.Podcast)
	}
	if got.Episode != "Ep 1" {
		t.Errorf("Episode = %q, want Ep 1", got.Episode)
	}
	if got.EpisodeGuid != "ep-1" {
		t.Errorf("EpisodeGuid = %q, want ep-1", got.EpisodeGuid)
	}
	if got.ValueMsatTotal != 5000 {
		t.Errorf("ValueMsatTotal = %d, want 5000", got.ValueMsatTotal)
	}
}

func TestPodcastGuruPaymentParseBoostagramFallsBackToTotalAmount(t *testing.T) {
	t.Parallel()

	payment := PodcastGuruPayment{
		ParsedMetadata: &PodcastGuruMetadata{
			Action:      "boost",
			TotalAmount: 314,
		},
	}

	got := payment.ParseBoostagram()
	if got.ValueMsatTotal != 314 {
		t.Errorf("ValueMsatTotal = %d, want 314 (from total_amount fallback)", got.ValueMsatTotal)
	}
}

func TestGetBoostagramPrefersPodcastGuruOverRSS(t *testing.T) {
	t.Parallel()

	invoice := IncomingInvoice{
		RSSPayment: &RssPayment{
			FeedTitle: "RSS Podcast",
		},
		PodcastGuruPayment: &PodcastGuruPayment{
			ParsedMetadata: &PodcastGuruMetadata{
				Podcast: "PodcastGuru Podcast",
				Action:  "stream",
			},
		},
	}

	got := invoice.GetBoostagram()
	if got.Podcast != "PodcastGuru Podcast" {
		t.Errorf("Podcast = %q, want PodcastGuru Podcast (PodcastGuru should win over RSS)", got.Podcast)
	}
}
