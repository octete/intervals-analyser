package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/local/lr-analysis-go/internal/config"
	"github.com/local/lr-analysis-go/internal/model"
)

type Client struct {
	http   *http.Client
	config config.Config
}

func New(config config.Config) *Client {
	return &Client{
		http:   &http.Client{Timeout: 30 * time.Second},
		config: config,
	}
}

func (c *Client) ListActivities(oldest, newest string, limit int) ([]model.Activity, error) {
	endpoint := fmt.Sprintf("%s/api/v1/athlete/%s/activities", c.config.BaseURL, c.config.AthleteID)
	query := url.Values{"oldest": {oldest}}
	if newest != "" {
		query.Set("newest", newest)
	}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	var activities []model.Activity
	if err := c.get(endpoint+"?"+query.Encode(), "list activities", &activities); err != nil {
		return nil, err
	}
	return activities, nil
}

func (c *Client) Activity(id string) (model.Activity, error) {
	if !safeID(id) {
		return model.Activity{}, fmt.Errorf("activity ID contains unsupported characters")
	}
	var activity model.Activity
	err := c.get(fmt.Sprintf("%s/api/v1/activity/%s", c.config.BaseURL, id), "fetch activity", &activity)
	return activity, err
}

func (c *Client) ActivityStreams(id string) ([]model.ActivityStream, error) {
	if !safeID(id) {
		return nil, fmt.Errorf("activity ID contains unsupported characters")
	}
	endpoint := fmt.Sprintf("%s/api/v1/activity/%s/streams.json", c.config.BaseURL, id)
	query := url.Values{"types": {"time,watts,cadence,left_right_balance"}}
	var streams []model.ActivityStream
	if err := c.get(endpoint+"?"+query.Encode(), "fetch activity streams", &streams); err != nil {
		return nil, err
	}
	return streams, nil
}

func (c *Client) get(endpoint, action string, target any) error {
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create request to %s: %w", action, err)
	}
	request.SetBasicAuth("API_KEY", c.config.APIKey)
	request.Header.Set("User-Agent", "lr-analysis-go/0.1.0")
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("contact Intervals.icu to %s: %w", action, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 300))
		if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
			return fmt.Errorf("Intervals.icu rejected credentials or access (%s) while trying to %s", response.Status, action)
		}
		if response.StatusCode == http.StatusTooManyRequests {
			return fmt.Errorf("Intervals.icu rate limit reached while trying to %s; retry after %s seconds", action, response.Header.Get("Retry-After"))
		}
		return fmt.Errorf("Intervals.icu error %s while trying to %s: %s", response.Status, action, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("Intervals.icu returned invalid JSON while trying to %s: %w", action, err)
	}
	return nil
}

func safeID(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' && r != '-' {
			return false
		}
	}
	return true
}
