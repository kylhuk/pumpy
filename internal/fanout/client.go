package fanout

import (
	"bufio"
	"encoding/json"
	"log"
	"math"
	"net"
	"time"

	"pumpy/internal/portal"
)

// EventType constants match the server wire format.
const (
	EventCreate    = "create"
	EventTrade     = "trade"
	EventMigration = "migration"
)

// Event is a decoded fanout message delivered to the bot.
type Event struct {
	Type      string
	Create    portal.NewToken
	Trade     portal.Trade
	Migration portal.Migration
}

// Client connects to a fanout.Server and delivers events over a channel.
// It reconnects automatically with exponential backoff on disconnect.
type Client struct {
	addr   string
	events chan Event
}

// NewClient creates a Client that will connect to addr. Call Start to begin receiving.
func NewClient(addr string) *Client {
	return &Client{
		addr:   addr,
		events: make(chan Event, 1024),
	}
}

// Events returns the receive-only channel of decoded events.
func (c *Client) Events() <-chan Event {
	return c.events
}

// Start connects and pumps events into Events(). Blocks; call in a goroutine.
func (c *Client) Start() {
	for attempt := 0; ; attempt++ {
		if attempt > 0 {
			wait := backoff(attempt)
			log.Printf("fanout: reconnecting in %s", wait)
			time.Sleep(wait)
		}
		if err := c.connect(); err != nil {
			log.Printf("fanout: disconnected: %v", err)
		} else {
			attempt = 0
		}
	}
}

func (c *Client) connect() error {
	conn, err := net.DialTimeout("tcp", c.addr, 10*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	log.Printf("fanout: connected to %s", c.addr)

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		var env Envelope
		if err := json.Unmarshal(scanner.Bytes(), &env); err != nil {
			continue
		}
		ev := c.decode(env)
		if ev == nil {
			continue
		}
		select {
		case c.events <- *ev:
		default:
			log.Printf("fanout: bot event buffer full — dropped %s", env.Type)
		}
	}
	return scanner.Err()
}

func (c *Client) decode(env Envelope) *Event {
	switch env.Type {
	case EventCreate:
		var t portal.NewToken
		if err := json.Unmarshal(env.Data, &t); err != nil {
			return nil
		}
		return &Event{Type: EventCreate, Create: t}
	case EventTrade:
		var t portal.Trade
		if err := json.Unmarshal(env.Data, &t); err != nil {
			return nil
		}
		return &Event{Type: EventTrade, Trade: t}
	case EventMigration:
		var m portal.Migration
		if err := json.Unmarshal(env.Data, &m); err != nil {
			return nil
		}
		return &Event{Type: EventMigration, Migration: m}
	}
	return nil
}

func backoff(attempt int) time.Duration {
	sec := math.Pow(2, float64(attempt))
	if sec > 60 {
		sec = 60
	}
	return time.Duration(sec) * time.Second
}
