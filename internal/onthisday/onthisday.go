// internal/onthisday/onthisday.go
package onthisday

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/Gautam-J/Folio/internal/models"
)

const DefaultBaseURL = "https://today.zenquotes.io/api"

type Client struct {
	baseURL    string
	httpClient *http.Client

	mu       sync.Mutex
	cached   models.OnThisDayData
	hasCache bool
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Get(ctx context.Context) (models.OnThisDayData, error) {
	data, err := c.fetch(ctx)
	if err != nil {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.hasCache {
			slog.Warn("zenquotes on-this-day fetch failed, serving cached event", "error", err)
			return c.cached, nil
		}
		return models.OnThisDayData{}, err
	}

	c.mu.Lock()
	c.cached = data
	c.hasCache = true
	c.mu.Unlock()

	return data, nil
}

type onThisDayEvent struct {
	Text string `json:"text"`
}

type onThisDayResponse struct {
	Data struct {
		Events []onThisDayEvent `json:"Events"`
	} `json:"data"`
}

func (c *Client) fetch(ctx context.Context) (models.OnThisDayData, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL, nil)
	if err != nil {
		return models.OnThisDayData{}, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return models.OnThisDayData{}, fmt.Errorf("request zenquotes on-this-day: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models.OnThisDayData{}, fmt.Errorf("zenquotes on-this-day returned status %d", resp.StatusCode)
	}

	var parsed onThisDayResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return models.OnThisDayData{}, fmt.Errorf("decode zenquotes on-this-day response: %w", err)
	}
	if len(parsed.Data.Events) == 0 {
		return models.OnThisDayData{}, fmt.Errorf("zenquotes on-this-day returned no events")
	}

	event := parsed.Data.Events[rand.Intn(len(parsed.Data.Events))]
	return models.OnThisDayData{Text: event.Text}, nil
}
