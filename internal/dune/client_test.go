package dune

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetTransactions_Basic(t *testing.T) {
	body, err := os.ReadFile("testdata/page_basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-Sim-Api-Key")
		if !strings.HasPrefix(r.URL.Path, "/beta/svm/transactions/") {
			t.Errorf("bad path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key", 100, 0)
	resp, _, err := c.GetTransactions(context.Background(), "Wallet111", 1000, "")
	if err != nil {
		t.Fatal(err)
	}
	if gotKey != "test-key" {
		t.Errorf("api key not sent: %q", gotKey)
	}
	if len(resp.Transactions) != 1 {
		t.Errorf("want 1 tx, got %d", len(resp.Transactions))
	}
	if resp.NextOffset != nil {
		t.Errorf("want nil offset, got %v", resp.NextOffset)
	}
}

func TestGetTransactions_Pagination(t *testing.T) {
	withNext, _ := os.ReadFile("testdata/page_with_next_offset.json")
	basic, _ := os.ReadFile("testdata/page_basic.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("offset") == "cursor-abc-123" {
			w.Write(basic)
		} else {
			w.Write(withNext)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key", 100, 0)
	page1, _, err := c.GetTransactions(context.Background(), "Wallet111", 1000, "")
	if err != nil {
		t.Fatal(err)
	}
	if page1.NextOffset == nil || *page1.NextOffset != "cursor-abc-123" {
		t.Fatalf("want next_offset cursor-abc-123, got %v", page1.NextOffset)
	}
	page2, _, err := c.GetTransactions(context.Background(), "Wallet111", 1000, *page1.NextOffset)
	if err != nil {
		t.Fatal(err)
	}
	if page2.NextOffset != nil {
		t.Errorf("want nil offset on page 2, got %v", page2.NextOffset)
	}
}

func TestGetTransactions_RetriesOn500(t *testing.T) {
	var calls atomic.Int32
	body, _ := os.ReadFile("testdata/page_basic.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(500)
			return
		}
		w.Write(body)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key", 100, 5)
	// Use a short-timeout context to avoid waiting on backoff in tests.
	// The client sleeps 1s on first retry; allow 30s total.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, _, err := c.GetTransactions(ctx, "Wallet111", 1000, "")
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if calls.Load() != 3 {
		t.Errorf("want 3 calls, got %d", calls.Load())
	}
}

func TestGetTransactions_NoRetryOn401(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(401)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "bad-key", 100, 5)
	_, _, err := c.GetTransactions(context.Background(), "Wallet111", 1000, "")
	if err == nil {
		t.Fatal("expected error on 401")
	}
	if calls.Load() != 1 {
		t.Errorf("want 1 call (no retry), got %d", calls.Load())
	}
}
