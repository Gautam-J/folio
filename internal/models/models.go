package models

import "time"

type WeatherData struct {
	Temperature float64
	Humidity    int
	WindSpeed   float64
	Description string
	FetchedAt   time.Time
}

type DisplayResponse struct {
	Status      int    `json:"status"`
	ImageURL    string `json:"image_url,omitempty"`
	Filename    string `json:"filename,omitempty"`
	RefreshRate int    `json:"refresh_rate,omitempty"`
	Error       string `json:"error,omitempty"`
}
