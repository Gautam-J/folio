package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func mockServer(t *testing.T, reposBody string, reposStatus int, graphqlBody string, graphqlStatus int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/graphql") {
			w.WriteHeader(graphqlStatus)
			w.Write([]byte(graphqlBody))
			return
		}
		w.WriteHeader(reposStatus)
		w.Write([]byte(reposBody))
	}))
}

const okGraphQL = `{"data":{"user":{"contributionsCollection":{"contributionCalendar":{"totalContributions":224},"totalCommitContributions":54,"totalPullRequestContributions":2,"totalIssueContributions":0}}}}`

func TestGet_Success(t *testing.T) {
	srv := mockServer(t, `[{"stargazers_count":411},{"stargazers_count":149}]`, http.StatusOK, okGraphQL, http.StatusOK)
	defer srv.Close()

	client := NewClient(srv.URL, "Gautam-J", "token")
	data, err := client.Get(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.TotalStars != 560 {
		t.Errorf("TotalStars = %d, want 560", data.TotalStars)
	}
	if data.ContributionsThisYear != 224 {
		t.Errorf("ContributionsThisYear = %d, want 224", data.ContributionsThisYear)
	}
	if data.Commits != 54 || data.PullRequests != 2 || data.Issues != 0 {
		t.Errorf("Commits/PRs/Issues = %d/%d/%d, want 54/2/0", data.Commits, data.PullRequests, data.Issues)
	}
}

func TestGet_ReposErrorNoCache(t *testing.T) {
	srv := mockServer(t, ``, http.StatusInternalServerError, okGraphQL, http.StatusOK)
	defer srv.Close()

	client := NewClient(srv.URL, "Gautam-J", "token")
	if _, err := client.Get(context.Background()); err == nil {
		t.Fatal("expected error when repos endpoint fails and no cache")
	}
}

func TestGet_GraphQLErrorNoCache(t *testing.T) {
	srv := mockServer(t, `[]`, http.StatusOK, ``, http.StatusInternalServerError)
	defer srv.Close()

	client := NewClient(srv.URL, "Gautam-J", "token")
	if _, err := client.Get(context.Background()); err == nil {
		t.Fatal("expected error when graphql endpoint fails and no cache")
	}
}

func TestGet_ErrorFallsBackToCache(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/graphql") {
			if callCount <= 2 {
				w.Write([]byte(okGraphQL))
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte(`[{"stargazers_count":10}]`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "Gautam-J", "token")
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

func TestGet_GraphQLAPIError(t *testing.T) {
	srv := mockServer(t, `[]`, http.StatusOK, `{"errors":[{"message":"Could not resolve to a User"}]}`, http.StatusOK)
	defer srv.Close()

	client := NewClient(srv.URL, "nonexistent", "token")
	if _, err := client.Get(context.Background()); err == nil {
		t.Fatal("expected error for graphql errors array")
	}
}
