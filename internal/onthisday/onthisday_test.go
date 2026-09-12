package onthisday

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGet_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"date":"September_12","data":{"Events":[{"text":"490 BC – Battle of Marathon"}],"Births":[],"Deaths":[]}}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	data, err := client.Get(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Text != "490 BC – Battle of Marathon" {
		t.Errorf("Text = %q, want %q", data.Text, "490 BC – Battle of Marathon")
	}
}

func TestGet_ServerErrorNoCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	if _, err := client.Get(context.Background()); err == nil {
		t.Fatal("expected error when no cache and server fails")
	}
}

func TestGet_ServerErrorFallsBackToCache(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":{"Events":[{"text":"First event."}]}}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	first, err := client.Get(context.Background())
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}

	second, err := client.Get(context.Background())
	if err != nil {
		t.Fatalf("expected cached fallback, got error: %v", err)
	}
	if second != first {
		t.Errorf("expected cached data %v, got %v", first, second)
	}
}

func TestGet_EmptyEvents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"Events":[]}}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	if _, err := client.Get(context.Background()); err == nil {
		t.Fatal("expected error for empty events array")
	}
}
