package mpk

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetch(t *testing.T) {
	data := fixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("startDate") != "21-10-2026" || q.Get("Location") != "Helsinki" || q.Get("Specialfield") != "Ammunta" {
			t.Errorf("query: %v", q)
		}
		_, _ = w.Write([]byte(data))
	}))
	defer server.Close()
	c := Client{Endpoint: server.URL, HTTP: server.Client()}
	events, err := c.Fetch(context.Background(), time.Date(2026, 10, 20, 22, 30, 0, 0, time.UTC))
	if err != nil || len(events) != 1 {
		t.Fatalf("fetch: %v, %d", err, len(events))
	}
}

func TestFetchFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"status", 503, "unavailable"},
		{"malformed", 200, "<rss>"},
		{"oversize", 200, strings.Repeat("x", int(maxResponseBytes)+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			c := Client{Endpoint: server.URL, HTTP: server.Client()}
			if _, err := c.Fetch(context.Background(), time.Now()); err == nil {
				t.Fatal("expected failure")
			}
		})
	}
	c := NewClient()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Fetch(ctx, time.Now()); err == nil {
		t.Fatal("cancelled request succeeded")
	}
}
