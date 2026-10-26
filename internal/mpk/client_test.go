package mpk

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchRequestsTodayInHelsinki(t *testing.T) {
	feed := readFeedFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		if query.Get("startDate") != "21-10-2026" {
			t.Errorf("start date = %q, want 21-10-2026", query.Get("startDate"))
		}
		if query.Get("Location") != "Helsinki" || query.Get("Specialfield") != "Ammunta" {
			t.Errorf("unexpected feed filters: %v", query)
		}
		_, _ = io.WriteString(w, feed)
	}))
	defer server.Close()
	client := Client{Endpoint: server.URL, HTTP: server.Client()}
	// It is already the next day in Helsinki at this UTC time.
	now := time.Date(2026, 10, 20, 22, 30, 0, 0, time.UTC)

	events, err := client.Fetch(context.Background(), now)

	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("fetched %d events, want 2", len(events))
	}
}

func TestFetchRejectsUnavailableOrOversizedResponse(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		status int
		body   string
	}{
		{name: "upstream unavailable", status: http.StatusServiceUnavailable, body: "unavailable"},
		{name: "response exceeds size limit", status: http.StatusOK, body: strings.Repeat("x", int(maxResponseBytes)+1)},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				w.WriteHeader(scenario.status)
				_, _ = io.WriteString(w, scenario.body)
			}))
			defer server.Close()
			client := Client{Endpoint: server.URL, HTTP: server.Client()}

			events, err := client.Fetch(context.Background(), time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC))

			if err == nil || len(events) != 0 {
				t.Fatalf("failed fetch returned events: %+v; error: %v", events, err)
			}
		})
	}
}
