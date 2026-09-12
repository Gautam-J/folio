package weather

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
		"%s?latitude=%g&longitude=%g"+
			"&current=temperature_2m,relative_humidity_2m,weather_code,wind_speed_10m,uv_index,cloud_cover"+
			"&daily=temperature_2m_min,temperature_2m_max,sunrise,sunset,precipitation_probability_max,wind_gusts_10m_max"+
			"&timezone=auto",
		c.baseURL, lat, lon,
	)

	data, err := c.fetch(ctx, url)
	if err != nil {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.hasCache {
			slog.Warn("open-meteo fetch failed, serving cached weather", "error", err)
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
		UVIndex          float64 `json:"uv_index"`
		CloudCover       int     `json:"cloud_cover"`
	} `json:"current"`
	Daily struct {
		TempMin              []float64 `json:"temperature_2m_min"`
		TempMax              []float64 `json:"temperature_2m_max"`
		Sunrise              []string  `json:"sunrise"`
		Sunset               []string  `json:"sunset"`
		PrecipProbabilityMax []int     `json:"precipitation_probability_max"`
		WindGustsMax         []float64 `json:"wind_gusts_10m_max"`
	} `json:"daily"`
}

// dailyTimeLayout matches Open-Meteo's iso8601 sunrise/sunset format when
// timezone=auto is set: a local timestamp with no offset, e.g. "2026-09-12T06:21".
const dailyTimeLayout = "2006-01-02T15:04"

func firstFloat(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	return vals[0]
}

func firstInt(vals []int) int {
	if len(vals) == 0 {
		return 0
	}
	return vals[0]
}

func firstTime(vals []string) time.Time {
	if len(vals) == 0 {
		return time.Time{}
	}
	t, err := time.Parse(dailyTimeLayout, vals[0])
	if err != nil {
		return time.Time{}
	}
	return t
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
		Temperature:       parsed.Current.Temperature,
		Humidity:          parsed.Current.RelativeHumidity,
		WindSpeed:         parsed.Current.WindSpeed,
		Description:       describeCode(parsed.Current.WeatherCode),
		Icon:              iconForCode(parsed.Current.WeatherCode),
		FetchedAt:         time.Now(),
		TempMin:           firstFloat(parsed.Daily.TempMin),
		TempMax:           firstFloat(parsed.Daily.TempMax),
		Sunrise:           firstTime(parsed.Daily.Sunrise),
		Sunset:            firstTime(parsed.Daily.Sunset),
		PrecipProbability: firstInt(parsed.Daily.PrecipProbabilityMax),
		WindGusts:         firstFloat(parsed.Daily.WindGustsMax),
		UVIndex:           parsed.Current.UVIndex,
		CloudCover:        parsed.Current.CloudCover,
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

// wmoIconGroups maps WMO weather codes to one of the icon keys weather.html
// has an SVG for: clear, partly-cloudy, cloudy, fog, rain, snow, thunderstorm.
var wmoIconGroups = map[int]string{
	0:  "clear",
	1:  "clear",
	2:  "partly-cloudy",
	3:  "cloudy",
	45: "fog",
	48: "fog",
	51: "rain",
	53: "rain",
	55: "rain",
	61: "rain",
	63: "rain",
	65: "rain",
	71: "snow",
	73: "snow",
	75: "snow",
	80: "rain",
	81: "rain",
	82: "rain",
	95: "thunderstorm",
	96: "thunderstorm",
	99: "thunderstorm",
}

func iconForCode(code int) string {
	if icon, ok := wmoIconGroups[code]; ok {
		return icon
	}
	return "cloudy"
}
