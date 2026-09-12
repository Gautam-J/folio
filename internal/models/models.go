package models

import "time"

type WeatherData struct {
	Temperature       float64
	Humidity          int
	WindSpeed         float64
	Description       string
	Icon              string
	FetchedAt         time.Time
	TempMin           float64
	TempMax           float64
	Sunrise           time.Time
	Sunset            time.Time
	PrecipProbability int
	WindGusts         float64
	UVIndex           float64
	CloudCover        int
}

type QuoteData struct {
	Quote  string
	Author string
}

type DisplayResponse struct {
	Status      int    `json:"status"`
	ImageURL    string `json:"image_url,omitempty"`
	Filename    string `json:"filename,omitempty"`
	RefreshRate int    `json:"refresh_rate,omitempty"`
	Error       string `json:"error,omitempty"`
}
