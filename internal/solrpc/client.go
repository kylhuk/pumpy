package solrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client calls Solana JSON-RPC for balance queries only.
type Client struct {
	http     *http.Client
	endpoint string
}

func NewClient(endpoint string) *Client {
	return &Client{
		http:     &http.Client{Timeout: 5 * time.Second},
		endpoint: endpoint,
	}
}

// GetBalance returns the SOL balance of pubkey in lamports.
func (c *Client) GetBalance(ctx context.Context, pubkey string) (uint64, error) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getBalance",
		"params":  []any{pubkey},
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("rpc: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var result struct {
		Result struct {
			Value uint64 `json:"value"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return 0, fmt.Errorf("rpc decode: %w", err)
	}
	if result.Error != nil {
		return 0, fmt.Errorf("rpc error: %s", result.Error.Message)
	}
	return result.Result.Value, nil
}
