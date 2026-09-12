package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempConfig(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func TestLoad_Success(t *testing.T) {
	path := writeTempConfig(t, `
access_token: "secret123"
port: 8080
latitude: 12.9
longitude: 77.6
refresh_rate_seconds: 1800
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AccessToken != "secret123" {
		t.Errorf("AccessToken = %q, want %q", cfg.AccessToken, "secret123")
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
	if cfg.Latitude != 12.9 || cfg.Longitude != 77.6 {
		t.Errorf("Latitude/Longitude = %v/%v, want 12.9/77.6", cfg.Latitude, cfg.Longitude)
	}
	if cfg.RefreshRateSeconds != 1800 {
		t.Errorf("RefreshRateSeconds = %d, want 1800", cfg.RefreshRateSeconds)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	if _, err := Load("/nonexistent/config.yaml"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_MissingAccessToken(t *testing.T) {
	path := writeTempConfig(t, `
port: 8080
refresh_rate_seconds: 1800
`)
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for missing access_token")
	}
}

func TestLoad_MissingRefreshRate(t *testing.T) {
	path := writeTempConfig(t, `
access_token: "secret123"
port: 8080
`)
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for missing/invalid refresh_rate_seconds")
	}
}
