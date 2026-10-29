package common

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	podcastIndexValueByEpisodeURL = "https://api.podcastindex.org/api/1.0/value/byepisodeguid"
	scoreboardUserAgent           = "scoreboard"
)

type RemoteItemNames struct {
	Podcast string
	Episode string
}

type podcastIndexValueResponse struct {
	Status string `json:"status"`
	Value  struct {
		Title     string `json:"title"`
		FeedTitle string `json:"feedTitle"`
	} `json:"value"`
}

func FetchRemoteNamesIfNeeded(invoice *IncomingInvoice) (RemoteItemNames, error) {
	if invoice == nil {
		return RemoteItemNames{}, nil
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	names, err := fetchRemoteItemNames(client, boostagram.RemoteFeedGuid, boostagram.RemoteItemGuid)
	if err != nil {
		return RemoteItemNames{}, err
	}

	return names, nil
}

func fetchRemoteItemNames(client *http.Client, feedGuid, itemGuid string) (RemoteItemNames, error) {
	parsed, err := url.Parse(podcastIndexValueByEpisodeURL)
	if err != nil {
		return RemoteItemNames{}, fmt.Errorf("failed to parse podcast index url: %w", err)
	}

	query := parsed.Query()
	query.Set("podcastguid", feedGuid)
	query.Set("episodeguid", itemGuid)
	parsed.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodGet, parsed.String(), nil)
	if err != nil {
		return RemoteItemNames{}, fmt.Errorf("failed to create podcast index request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", scoreboardUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return RemoteItemNames{}, fmt.Errorf("failed to fetch podcast index value: %w", err)
	}
	defer resp.Body.Close()

	var body podcastIndexValueResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return RemoteItemNames{}, fmt.Errorf("failed to decode podcast index value: %w", err)
	}

	if body.Status != "true" {
		return RemoteItemNames{}, nil
	}

	return RemoteItemNames{Podcast: body.Value.FeedTitle, Episode: body.Value.Title}, nil
}
