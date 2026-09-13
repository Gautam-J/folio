// internal/github/github.go
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/Gautam-J/Folio/internal/models"
)

const DefaultBaseURL = "https://api.github.com"

type Client struct {
	baseURL    string
	username   string
	token      string
	httpClient *http.Client

	mu       sync.Mutex
	cached   models.GitHubStatsData
	hasCache bool
}

func NewClient(baseURL, username, token string) *Client {
	return &Client{
		baseURL:    baseURL,
		username:   username,
		token:      token,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Get(ctx context.Context) (models.GitHubStatsData, error) {
	data, err := c.fetch(ctx)
	if err != nil {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.hasCache {
			slog.Warn("github stats fetch failed, serving cached stats", "error", err)
			return c.cached, nil
		}
		return models.GitHubStatsData{}, err
	}

	c.mu.Lock()
	c.cached = data
	c.hasCache = true
	c.mu.Unlock()

	return data, nil
}

type repo struct {
	StargazersCount int `json:"stargazers_count"`
}

func (c *Client) fetch(ctx context.Context) (models.GitHubStatsData, error) {
	totalStars, err := c.fetchTotalStars(ctx)
	if err != nil {
		return models.GitHubStatsData{}, err
	}

	contributions, commits, prs, issues, err := c.fetchContributions(ctx)
	if err != nil {
		return models.GitHubStatsData{}, err
	}

	return models.GitHubStatsData{
		ContributionsThisYear: contributions,
		TotalStars:            totalStars,
		Commits:               commits,
		PullRequests:          prs,
		Issues:                issues,
	}, nil
}

func (c *Client) fetchTotalStars(ctx context.Context) (int, error) {
	url := fmt.Sprintf("%s/users/%s/repos?per_page=100&type=owner", c.baseURL, c.username)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("build repos request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request github repos: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("github repos returned status %d", resp.StatusCode)
	}

	var repos []repo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return 0, fmt.Errorf("decode github repos response: %w", err)
	}

	total := 0
	for _, r := range repos {
		total += r.StargazersCount
	}
	return total, nil
}

const contributionsQuery = `query($login: String!) {
  user(login: $login) {
    contributionsCollection {
      contributionCalendar { totalContributions }
      totalCommitContributions
      totalPullRequestContributions
      totalIssueContributions
    }
  }
}`

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type graphQLResponse struct {
	Data struct {
		User struct {
			ContributionsCollection struct {
				ContributionCalendar struct {
					TotalContributions int `json:"totalContributions"`
				} `json:"contributionCalendar"`
				TotalCommitContributions      int `json:"totalCommitContributions"`
				TotalPullRequestContributions int `json:"totalPullRequestContributions"`
				TotalIssueContributions       int `json:"totalIssueContributions"`
			} `json:"contributionsCollection"`
		} `json:"user"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (c *Client) fetchContributions(ctx context.Context) (contributions, commits, prs, issues int, err error) {
	body, err := json.Marshal(graphQLRequest{
		Query:     contributionsQuery,
		Variables: map[string]any{"login": c.username},
	})
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("build graphql request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/graphql", bytes.NewReader(body))
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("build graphql request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("request github graphql: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, 0, 0, fmt.Errorf("github graphql returned status %d", resp.StatusCode)
	}

	var parsed graphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("decode github graphql response: %w", err)
	}
	if len(parsed.Errors) > 0 {
		return 0, 0, 0, 0, fmt.Errorf("github graphql error: %s", parsed.Errors[0].Message)
	}

	cc := parsed.Data.User.ContributionsCollection
	return cc.ContributionCalendar.TotalContributions, cc.TotalCommitContributions, cc.TotalPullRequestContributions, cc.TotalIssueContributions, nil
}
