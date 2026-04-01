package n8n

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultPointSearchWebhookURL = "https://mpit-bot.kostya1024.ru/webhook/point-search"

type Client struct {
	pointSearchWebhookURL string
	httpClient            *http.Client
}

type PointSearchBudget struct {
	Type     string  `json:"type"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type PointSearchRequest struct {
	Event        string            `json:"event"`
	City         string            `json:"city"`
	Date         string            `json:"date"`
	Time         string            `json:"time"`
	Budget       PointSearchBudget `json:"budget"`
	Participants int               `json:"participants"`
	Preferences  []string          `json:"preferences"`
}

type PointSearchVenue struct {
	ID             *string `json:"id"`
	Name           *string `json:"name"`
	Address        *string `json:"address"`
	AddressName    *string `json:"address_name"`
	AddressComment *string `json:"address_comment"`
	PurposeName    *string `json:"purpose_name"`
	Type           *string `json:"type"`
}

type PointSearchResponse struct {
	Total  int                `json:"total"`
	Venues []PointSearchVenue `json:"venues"`
}

func NewClient(pointSearchWebhookURL string, timeout time.Duration) *Client {
	pointSearchWebhookURL = strings.TrimSpace(pointSearchWebhookURL)
	if pointSearchWebhookURL == "" {
		pointSearchWebhookURL = defaultPointSearchWebhookURL
	}

	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	return &Client{
		pointSearchWebhookURL: pointSearchWebhookURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) PointSearch(ctx context.Context, input PointSearchRequest) (PointSearchResponse, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return PointSearchResponse{}, fmt.Errorf("marshal point search request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.pointSearchWebhookURL, bytes.NewReader(body))
	if err != nil {
		return PointSearchResponse{}, fmt.Errorf("build point search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return PointSearchResponse{}, fmt.Errorf("call point search webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return PointSearchResponse{}, fmt.Errorf(
			"point search webhook returned status %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(payload)),
		)
	}

	var out PointSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return PointSearchResponse{}, fmt.Errorf("decode point search response: %w", err)
	}

	return out, nil
}
