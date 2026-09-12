package quote

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGet_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"q":"Stay hungry, stay foolish.","a":"Steve Jobs","h":"<blockquote>&ldquo;Stay hungry, stay foolish.&rdquo; &mdash; <footer>Steve Jobs</footer></blockquote>"}]`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	data, err := client.Get(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Quote != "Stay hungry, stay foolish." {
		t.Errorf("Quote = %q, want %q", data.Quote, "Stay hungry, stay foolish.")
	}
	if data.Author != "Steve Jobs" {
		t.Errorf("Author = %q, want %q", data.Author, "Steve Jobs")
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
			w.Write([]byte(`[{"q":"First quote.","a":"Someone"}]`))
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

func TestGet_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	if _, err := client.Get(context.Background()); err == nil {
		t.Fatal("expected error for empty quote array")
	}
}
