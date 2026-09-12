package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Gautam-J/Folio/internal/models"
)

const DefaultBaseURL = "https://api.open-meteo.com/v1/forecast"

type Client struct {
	baseURL    string
	httpClient *http.Client

	mu       sync.Mutex
	cached   models.WeatherData
	hasCache bool
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Get(ctx context.Context, lat, lon float64) (models.WeatherData, error) {
	url := fmt.Sprintf(
		"%s?latitude=%g&longitude=%g&current=temperature_2m,relative_humidity_2m,weather_code,wind_speed_10m",
		c.baseURL, lat, lon,
	)

	data, err := c.fetch(ctx, url)
	if err != nil {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.hasCache {
			return c.cached, nil
		}
		return models.WeatherData{}, err
	}

	c.mu.Lock()
	c.cached = data
	c.hasCache = true
	c.mu.Unlock()

	return data, nil
}

type openMeteoResponse struct {
	Current struct {
		Temperature      float64 `json:"temperature_2m"`
		RelativeHumidity int     `json:"relative_humidity_2m"`
		WeatherCode      int     `json:"weather_code"`
		WindSpeed        float64 `json:"wind_speed_10m"`
	} `json:"current"`
}

func (c *Client) fetch(ctx context.Context, url string) (models.WeatherData, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return models.WeatherData{}, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return models.WeatherData{}, fmt.Errorf("request open-meteo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models.WeatherData{}, fmt.Errorf("open-meteo returned status %d", resp.StatusCode)
	}

	var parsed openMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return models.WeatherData{}, fmt.Errorf("decode open-meteo response: %w", err)
	}

	return models.WeatherData{
		Temperature: parsed.Current.Temperature,
		Humidity:    parsed.Current.RelativeHumidity,
		WindSpeed:   parsed.Current.WindSpeed,
		Description: describeCode(parsed.Current.WeatherCode),
		FetchedAt:   time.Now(),
	}, nil
}

var wmoDescriptions = map[int]string{
	0:  "Clear sky",
	1:  "Mainly clear",
	2:  "Partly cloudy",
	3:  "Overcast",
	45: "Fog",
	48: "Depositing rime fog",
	51: "Light drizzle",
	53: "Moderate drizzle",
	55: "Dense drizzle",
	61: "Slight rain",
	63: "Moderate rain",
	65: "Heavy rain",
	71: "Slight snow",
	73: "Moderate snow",
	75: "Heavy snow",
	80: "Slight rain showers",
	81: "Moderate rain showers",
	82: "Violent rain showers",
	95: "Thunderstorm",
	96: "Thunderstorm with slight hail",
	99: "Thunderstorm with heavy hail",
}

func describeCode(code int) string {
	if desc, ok := wmoDescriptions[code]; ok {
		return desc
	}
	return "Unknown"
}
