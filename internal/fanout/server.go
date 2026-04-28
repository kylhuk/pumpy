package fanout

import (
	"encoding/json"
	"log"
	"net"
	"sync"

	"pumpy/internal/portal"
)

const clientBufSize = 1024

// Server listens on a TCP address and broadcasts portal events to connected clients.
// It implements portal.Handler so it can be registered directly with portal.Client.
type Server struct {
	addr    string
	mu      sync.Mutex
	clients map[*serverClient]struct{}
}

type serverClient struct {
	ch chan []byte
}

func NewServer(addr string) *Server {
	return &Server{
		addr:    addr,
		clients: make(map[*serverClient]struct{}),
	}
}

// Listen binds and accepts connections. Blocks until the process exits.
func (s *Server) Listen() {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		log.Printf("fanout: listen %s: %v", s.addr, err)
		return
	}
	log.Printf("fanout: listening on %s", s.addr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("fanout: accept: %v", err)
			continue
		}
		c := &serverClient{ch: make(chan []byte, clientBufSize)}
		s.mu.Lock()
		s.clients[c] = struct{}{}
		s.mu.Unlock()
		go s.serve(conn, c)
	}
}

func (s *Server) serve(conn net.Conn, c *serverClient) {
	defer func() {
		conn.Close()
		s.mu.Lock()
		delete(s.clients, c)
		s.mu.Unlock()
	}()
	for msg := range c.ch {
		if _, err := conn.Write(msg); err != nil {
			return
		}
	}
}

func (s *Server) broadcast(env Envelope) {
	data, err := json.Marshal(env)
	if err != nil {
		return
	}
	data = append(data, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.clients {
		select {
		case c.ch <- data:
		default:
			log.Printf("fanout: slow client — dropped event %s", env.Type)
		}
	}
}

func (s *Server) OnNewToken(t portal.NewToken) {
	d, _ := json.Marshal(t)
	s.broadcast(Envelope{Type: "create", Data: d})
}

func (s *Server) OnTrade(t portal.Trade) {
	d, _ := json.Marshal(t)
	s.broadcast(Envelope{Type: "trade", Data: d})
}

func (s *Server) OnMigration(m portal.Migration) {
	d, _ := json.Marshal(m)
	s.broadcast(Envelope{Type: "migration", Data: d})
}
