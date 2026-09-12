package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGet_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"current":{"temperature_2m":21.5,"relative_humidity_2m":55,"weather_code":3,"wind_speed_10m":4.2,"uv_index":1.5,"cloud_cover":98},
			"daily":{"temperature_2m_min":[15.0],"temperature_2m_max":[25.0],"sunrise":["2026-09-12T06:21"],"sunset":["2026-09-12T18:39"],"precipitation_probability_max":[90],"wind_gusts_10m_max":[47.0]}
		}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	data, err := client.Get(context.Background(), 12.9, 77.6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Temperature != 21.5 {
		t.Errorf("Temperature = %v, want 21.5", data.Temperature)
	}
	if data.Humidity != 55 {
		t.Errorf("Humidity = %v, want 55", data.Humidity)
	}
	if data.WindSpeed != 4.2 {
		t.Errorf("WindSpeed = %v, want 4.2", data.WindSpeed)
	}
	if data.Description != "Overcast" {
		t.Errorf("Description = %q, want %q", data.Description, "Overcast")
	}
	if data.Icon != "cloudy" {
		t.Errorf("Icon = %q, want %q", data.Icon, "cloudy")
	}
	if data.TempMin != 15.0 {
		t.Errorf("TempMin = %v, want 15.0", data.TempMin)
	}
	if data.TempMax != 25.0 {
		t.Errorf("TempMax = %v, want 25.0", data.TempMax)
	}
	wantSunrise := time.Date(2026, 9, 12, 6, 21, 0, 0, time.UTC)
	if !data.Sunrise.Equal(wantSunrise) {
		t.Errorf("Sunrise = %v, want %v", data.Sunrise, wantSunrise)
	}
	wantSunset := time.Date(2026, 9, 12, 18, 39, 0, 0, time.UTC)
	if !data.Sunset.Equal(wantSunset) {
		t.Errorf("Sunset = %v, want %v", data.Sunset, wantSunset)
	}
	if data.PrecipProbability != 90 {
		t.Errorf("PrecipProbability = %v, want 90", data.PrecipProbability)
	}
	if data.WindGusts != 47.0 {
		t.Errorf("WindGusts = %v, want 47.0", data.WindGusts)
	}
	if data.UVIndex != 1.5 {
		t.Errorf("UVIndex = %v, want 1.5", data.UVIndex)
	}
	if data.CloudCover != 98 {
		t.Errorf("CloudCover = %v, want 98", data.CloudCover)
	}
}

func TestGet_ServerErrorNoCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	if _, err := client.Get(context.Background(), 12.9, 77.6); err == nil {
		t.Fatal("expected error when no cache and server fails")
	}
}

func TestGet_ServerErrorFallsBackToCache(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"current":{"temperature_2m":18.0,"relative_humidity_2m":60,"weather_code":1,"wind_speed_10m":3.0}}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	first, err := client.Get(context.Background(), 12.9, 77.6)
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}

	second, err := client.Get(context.Background(), 12.9, 77.6)
	if err != nil {
		t.Fatalf("expected cached fallback, got error: %v", err)
	}
	if second.Temperature != first.Temperature {
		t.Errorf("expected cached data %v, got %v", first, second)
	}
}

func TestDescribeCode(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{0, "Clear sky"},
		{61, "Slight rain"},
		{999, "Unknown"},
	}
	for _, tc := range cases {
		if got := describeCode(tc.code); got != tc.want {
			t.Errorf("describeCode(%d) = %q, want %q", tc.code, got, tc.want)
		}
	}
}

func TestIconForCode(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{0, "clear"},
		{2, "partly-cloudy"},
		{3, "cloudy"},
		{45, "fog"},
		{63, "rain"},
		{73, "snow"},
		{95, "thunderstorm"},
		{999, "cloudy"},
	}
	for _, tc := range cases {
		if got := iconForCode(tc.code); got != tc.want {
			t.Errorf("iconForCode(%d) = %q, want %q", tc.code, got, tc.want)
		}
	}
}
