package portal

import (
	"encoding/json"
	"log"
	"math"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const wsURL = "wss://pumpportal.fun/api/data"

// Handler receives decoded events. Implementations must be goroutine-safe.
type Handler interface {
	OnNewToken(t NewToken)
	OnTrade(t Trade)
	OnMigration(m Migration)
}

// Client manages the single pumpportal WebSocket connection.
type Client struct {
	handler    Handler
	mu         sync.Mutex
	conn       *websocket.Conn
	activeMints map[string]struct{}
}

func NewClient(h Handler) *Client {
	return &Client{
		handler:     h,
		activeMints: make(map[string]struct{}),
	}
}

// SubscribeToken adds a mint to subscribeTokenTrade and to the active set.
// Sends the incremental subscribe over the open connection.
func (c *Client) SubscribeToken(mint string) {
	c.mu.Lock()
	c.activeMints[mint] = struct{}{}
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return
	}
	msg := map[string]any{"method": "subscribeTokenTrade", "keys": []string{mint}}
	data, _ := json.Marshal(msg)
	c.mu.Lock()
	_ = conn.WriteMessage(websocket.TextMessage, data)
	c.mu.Unlock()
}

// UnsubscribeToken removes mints from the active set and sends unsubscribeTokenTrade.
func (c *Client) UnsubscribeToken(mints []string) {
	c.mu.Lock()
	for _, m := range mints {
		delete(c.activeMints, m)
	}
	conn := c.conn
	c.mu.Unlock()

	if conn == nil || len(mints) == 0 {
		return
	}
	msg := map[string]any{"method": "unsubscribeTokenTrade", "keys": mints}
	data, _ := json.Marshal(msg)
	c.mu.Lock()
	_ = conn.WriteMessage(websocket.TextMessage, data)
	c.mu.Unlock()
}

// ActiveMints returns a snapshot of the current active mint set.
func (c *Client) ActiveMints() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, 0, len(c.activeMints))
	for m := range c.activeMints {
		out = append(out, m)
	}
	return out
}

// SetActiveMints replaces the in-memory active set (called on startup after DB hydration).
func (c *Client) SetActiveMints(mints []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activeMints = make(map[string]struct{}, len(mints))
	for _, m := range mints {
		c.activeMints[m] = struct{}{}
	}
}

// Run connects and processes messages, reconnecting with exponential backoff on failure.
// Blocks until ctx-equivalent signal; call from a goroutine.
func (c *Client) Run() {
	attempt := 0
	for {
		err := c.connect()
		if err != nil {
			wait := backoff(attempt)
			log.Printf("portal: disconnected (%v), reconnecting in %s", err, wait)
			time.Sleep(wait)
			attempt++
		} else {
			attempt = 0
		}
	}
}

func (c *Client) connect() error {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return err
	}
	log.Println("portal: connected to", wsURL)

	c.mu.Lock()
	c.conn = conn
	activeMints := make([]string, 0, len(c.activeMints))
	for m := range c.activeMints {
		activeMints = append(activeMints, m)
	}
	c.mu.Unlock()

	// Always subscribe to new tokens and migrations on every connect.
	send := func(v any) {
		data, _ := json.Marshal(v)
		if werr := conn.WriteMessage(websocket.TextMessage, data); werr != nil {
			log.Println("portal: write error:", werr)
		}
	}
	send(map[string]any{"method": "subscribeNewToken"})
	send(map[string]any{"method": "subscribeMigration"})

	// Re-subscribe to all active mints in chunks of 100 to avoid oversized messages.
	for i := 0; i < len(activeMints); i += 100 {
		end := i + 100
		if end > len(activeMints) {
			end = len(activeMints)
		}
		send(map[string]any{"method": "subscribeTokenTrade", "keys": activeMints[i:end]})
	}
	log.Printf("portal: subscribed — newToken/migration, %d active mints", len(activeMints))

	defer func() {
		c.mu.Lock()
		c.conn = nil
		c.mu.Unlock()
		conn.Close()
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		c.dispatch(msg)
	}
}

func (c *Client) dispatch(msg []byte) {
	var raw RawEvent
	if err := json.Unmarshal(msg, &raw); err != nil {
		return
	}

	switch raw.TxType {
	case "create":
		var t NewToken
		if err := json.Unmarshal(raw.Raw, &t); err == nil {
			c.handler.OnNewToken(t)
		}
	case "buy", "sell":
		var t Trade
		if err := json.Unmarshal(raw.Raw, &t); err == nil {
			c.handler.OnTrade(t)
		}
	default:
		// Check for migration (no txType on migration events — uses "mint" presence).
		if raw.TxType == "" {
			var m Migration
			if err := json.Unmarshal(raw.Raw, &m); err == nil && m.Mint != "" {
				c.handler.OnMigration(m)
			}
		}
	}
}

func backoff(attempt int) time.Duration {
	sec := math.Pow(2, float64(attempt))
	if sec > 60 {
		sec = 60
	}
	return time.Duration(sec) * time.Second
}
