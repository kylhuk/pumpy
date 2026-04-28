package lightning

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const tradeURL = "https://pumpportal.fun/api/trade"

// Client sends trades to pumpportal.fun's Lightning endpoint.
//
// The API key is passed per-call so that a future wallet-rotation layer can
// supply different keys per token without restarting the client.
type Client struct {
	http       *http.Client
	maxRetries int
}

func NewClient(timeout time.Duration, maxRetries int) *Client {
	return &Client{
		http:       &http.Client{Timeout: timeout},
		maxRetries: maxRetries,
	}
}

// Trade submits a trade and returns the transaction signature.
// It retries on 5xx / network errors but not on 4xx (slippage exceeded, etc.).
func (c *Client) Trade(ctx context.Context, apiKey string, req TradeRequest) (TradeResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return TradeResult{}, fmt.Errorf("marshal: %w", err)
	}

	backoff := []time.Duration{500 * time.Millisecond, 1 * time.Second, 2 * time.Second}

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			wait := backoff[min(attempt-1, len(backoff)-1)]
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return TradeResult{}, ctx.Err()
			}
		}

		url := fmt.Sprintf("%s?api-key=%s", tradeURL, apiKey)
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return TradeResult{}, err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "application/json")

		resp, err := c.http.Do(httpReq)
		if err != nil {
			if attempt < c.maxRetries {
				continue
			}
			return TradeResult{}, fmt.Errorf("http: %w", err)
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == 400 || resp.StatusCode == 401 || resp.StatusCode == 403 {
			return TradeResult{}, fmt.Errorf("lightning %d: %s", resp.StatusCode, string(respBody))
		}
		if resp.StatusCode >= 500 || resp.StatusCode == 429 {
			if attempt < c.maxRetries {
				continue
			}
			return TradeResult{}, fmt.Errorf("lightning retries exhausted, last status %d", resp.StatusCode)
		}
		if resp.StatusCode != 200 {
			return TradeResult{}, fmt.Errorf("lightning unexpected %d: %s", resp.StatusCode, string(respBody))
		}

		// Response is a plain transaction signature string.
		sig := string(bytes.TrimSpace(respBody))
		if len(sig) > 2 && sig[0] == '"' {
			// Unquote if JSON string.
			var s string
			if err := json.Unmarshal(respBody, &s); err == nil {
				sig = s
			}
		}
		return TradeResult{Signature: sig}, nil
	}
	return TradeResult{}, fmt.Errorf("unreachable")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
