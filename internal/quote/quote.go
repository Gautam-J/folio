package quote

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/Gautam-J/Folio/internal/models"
)

const DefaultBaseURL = "https://zenquotes.io/api/random"

type Client struct {
	baseURL    string
	httpClient *http.Client

	mu       sync.Mutex
	cached   models.QuoteData
	hasCache bool
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Get(ctx context.Context) (models.QuoteData, error) {
	data, err := c.fetch(ctx)
	if err != nil {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.hasCache {
			slog.Warn("zenquotes fetch failed, serving cached quote", "error", err)
			return c.cached, nil
		}
		return models.QuoteData{}, err
	}

	c.mu.Lock()
	c.cached = data
	c.hasCache = true
	c.mu.Unlock()

	return data, nil
}

type zenQuote struct {
	Quote  string `json:"q"`
	Author string `json:"a"`
}

func (c *Client) fetch(ctx context.Context) (models.QuoteData, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL, nil)
	if err != nil {
		return models.QuoteData{}, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return models.QuoteData{}, fmt.Errorf("request zenquotes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models.QuoteData{}, fmt.Errorf("zenquotes returned status %d", resp.StatusCode)
	}

	var parsed []zenQuote
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return models.QuoteData{}, fmt.Errorf("decode zenquotes response: %w", err)
	}
	if len(parsed) == 0 {
		return models.QuoteData{}, fmt.Errorf("zenquotes returned no quotes")
	}

	return models.QuoteData{
		Quote:  parsed[0].Quote,
		Author: parsed[0].Author,
	}, nil
}
