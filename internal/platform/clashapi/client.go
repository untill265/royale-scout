package clashapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"royalescout/internal/platform/netpolicy"
)

const (
	DefaultBaseURL       = "https://api.clashroyale.com/v1"
	defaultResponseLimit = 32 << 20
)

type Client struct {
	baseURL       string
	token         string
	http          *http.Client
	userAgent     string
	maxAttempts   int
	retryBaseWait time.Duration
}

type Option func(*Client)

func WithBaseURL(baseURL string) Option {
	return func(client *Client) {
		client.baseURL = strings.TrimRight(baseURL, "/")
	}
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		client.http = httpClient
	}
}

func WithRetry(maxAttempts int, baseWait time.Duration) Option {
	return func(client *Client) {
		client.maxAttempts = maxAttempts
		client.retryBaseWait = baseWait
	}
}

func New(token string, options ...Option) (*Client, error) {
	trimmedToken := strings.TrimSpace(token)
	if trimmedToken == "" {
		return nil, errors.New("Clash Royale API token is empty")
	}

	client := &Client{
		baseURL:       DefaultBaseURL,
		token:         trimmedToken,
		http:          netpolicy.NewHTTPSClient(20*time.Second, "api.clashroyale.com"),
		userAgent:     "RoyaleScout/0.1",
		maxAttempts:   4,
		retryBaseWait: 250 * time.Millisecond,
	}
	for _, option := range options {
		option(client)
	}

	if client.http == nil {
		return nil, errors.New("HTTP client is nil")
	}
	if client.maxAttempts < 1 {
		return nil, errors.New("max attempts must be positive")
	}
	if _, err := url.ParseRequestURI(client.baseURL); err != nil {
		return nil, fmt.Errorf("invalid Clash Royale API base URL: %w", err)
	}

	return client, nil
}

type ResponseMeta struct {
	StatusCode int
	Duration   time.Duration
	RetryAfter time.Duration
	Attempts   int
}

type APIError struct {
	StatusCode int
	Reason     string
	Message    string
	RetryAfter time.Duration
}

func (err *APIError) Error() string {
	if err.Message == "" {
		return fmt.Sprintf("Clash Royale API returned HTTP %d (%s)", err.StatusCode, err.Reason)
	}
	return fmt.Sprintf("Clash Royale API returned HTTP %d (%s): %s", err.StatusCode, err.Reason, err.Message)
}

func (client *Client) Locations(ctx context.Context, limit int, after string) (Page[Location], ResponseMeta, error) {
	var page Page[Location]
	meta, err := client.getJSON(ctx, "/locations", pageQuery(limit, after), &page)
	return page, meta, err
}

func (client *Client) Seasons(ctx context.Context, limit int, after string) (Page[Season], ResponseMeta, error) {
	var page Page[Season]
	meta, err := client.getJSON(ctx, "/locations/global/seasons", pageQuery(limit, after), &page)
	return page, meta, err
}

func (client *Client) PathOfLegendPlayers(ctx context.Context, locationID string, limit int, after string) (Page[LeaderboardPlayer], ResponseMeta, error) {
	var page Page[LeaderboardPlayer]
	endpoint := "/locations/" + url.PathEscape(locationID) + "/pathoflegend/players"
	meta, err := client.getJSON(ctx, endpoint, pageQuery(limit, after), &page)
	return page, meta, err
}

func (client *Client) BattleLog(ctx context.Context, playerTag string) ([]Battle, ResponseMeta, error) {
	var battles []Battle
	endpoint := "/players/" + url.PathEscape(playerTag) + "/battlelog"
	meta, err := client.getJSON(ctx, endpoint, nil, &battles)
	return battles, meta, err
}

func (client *Client) Cards(ctx context.Context, limit int, after string) (Page[Card], ResponseMeta, error) {
	var page Page[Card]
	meta, err := client.getJSON(ctx, "/cards", pageQuery(limit, after), &page)
	return page, meta, err
}

func pageQuery(limit int, after string) url.Values {
	query := make(url.Values)
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	if after != "" {
		query.Set("after", after)
	}
	return query
}

func (client *Client) getJSON(ctx context.Context, endpoint string, query url.Values, destination any) (ResponseMeta, error) {
	requestURL := client.baseURL + endpoint
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}

	startedAt := time.Now()
	meta := ResponseMeta{}
	for attempt := 1; attempt <= client.maxAttempts; attempt++ {
		meta.Attempts = attempt
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			return meta, fmt.Errorf("create Clash Royale API request: %w", err)
		}
		request.Header.Set("Authorization", "Bearer "+client.token)
		request.Header.Set("Accept", "application/json")
		request.Header.Set("User-Agent", client.userAgent)

		response, requestErr := client.http.Do(request)
		meta.Duration = time.Since(startedAt)
		if requestErr != nil {
			if attempt < client.maxAttempts && ctx.Err() == nil {
				if err := waitForRetry(ctx, client.retryWait(attempt, 0)); err != nil {
					return meta, err
				}
				continue
			}
			return meta, fmt.Errorf("perform Clash Royale API request: %w", requestErr)
		}

		meta.StatusCode = response.StatusCode
		meta.RetryAfter = parseRetryAfter(response.Header.Get("Retry-After"))
		body, readErr := io.ReadAll(io.LimitReader(response.Body, defaultResponseLimit))
		closeErr := response.Body.Close()
		if readErr != nil {
			return meta, fmt.Errorf("read Clash Royale API response: %w", readErr)
		}
		if closeErr != nil {
			return meta, fmt.Errorf("close Clash Royale API response: %w", closeErr)
		}

		if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
			if err := json.Unmarshal(body, destination); err != nil {
				return meta, fmt.Errorf("decode Clash Royale API response: %w", err)
			}
			return meta, nil
		}

		apiError := decodeAPIError(response.StatusCode, meta.RetryAfter, body)
		if attempt < client.maxAttempts && isRetryableStatus(response.StatusCode) {
			if err := waitForRetry(ctx, client.retryWait(attempt, meta.RetryAfter)); err != nil {
				return meta, err
			}
			continue
		}
		return meta, apiError
	}

	return meta, errors.New("Clash Royale API request exhausted retry attempts")
}

func decodeAPIError(statusCode int, retryAfter time.Duration, body []byte) *APIError {
	var payload struct {
		Reason  string `json:"reason"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &payload)
	return &APIError{
		StatusCode: statusCode,
		Reason:     payload.Reason,
		Message:    payload.Message,
		RetryAfter: retryAfter,
	}
}

func isRetryableStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func (client *Client) retryWait(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return retryAfter
	}
	return client.retryBaseWait * time.Duration(1<<(attempt-1))
}

func waitForRetry(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}
