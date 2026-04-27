package dune

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/time/rate"
)

type Client struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
	limiter    *rate.Limiter
	maxRetries int
}

func NewClient(baseURL, apiKey string, rps float64, maxRetries int) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		apiKey:     apiKey,
		baseURL:    baseURL,
		limiter:    rate.NewLimiter(rate.Limit(rps), 1),
		maxRetries: maxRetries,
	}
}

func (c *Client) GetTransactions(ctx context.Context, address string, limit int, offset string) (*TransactionsResponse, *http.Response, error) {
	u, err := url.Parse(fmt.Sprintf("%s/beta/svm/transactions/%s", c.baseURL, address))
	if err != nil {
		return nil, nil, fmt.Errorf("parse url: %w", err)
	}
	q := u.Query()
	q.Set("limit", fmt.Sprintf("%d", limit))
	if offset != "" {
		q.Set("offset", offset)
	}
	u.RawQuery = q.Encode()

	backoff := []time.Duration{1 * time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 30 * time.Second}

	var lastResp *http.Response
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		// Wait out the backoff before consuming a rate-limit token.
		if attempt > 0 {
			select {
			case <-time.After(backoff[min(attempt-1, len(backoff)-1)]):
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			}
		}

		if err := c.limiter.Wait(ctx); err != nil {
			return nil, nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("X-Sim-Api-Key", c.apiKey)
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if attempt < c.maxRetries {
				continue // sleep handled at top of next iteration
			}
			return nil, nil, fmt.Errorf("http: %w", err)
		}
		lastResp = resp

		// No retry for non-recoverable client errors.
		if resp.StatusCode == 400 || resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 404 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, resp, fmt.Errorf("dune %d: %s", resp.StatusCode, string(body))
		}
		// Retry on rate-limit and server errors.
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			resp.Body.Close()
			if attempt < c.maxRetries {
				continue // sleep handled at top of next iteration
			}
			return nil, resp, fmt.Errorf("dune retries exhausted, last status %d", resp.StatusCode)
		}

		var out TransactionsResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			resp.Body.Close()
			return nil, resp, fmt.Errorf("decode: %w", err)
		}
		resp.Body.Close()
		if out.Transactions == nil {
			return nil, resp, fmt.Errorf("response missing transactions[]")
		}
		return &out, resp, nil
	}
	return nil, lastResp, fmt.Errorf("unreachable")
}
