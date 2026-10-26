package orochi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stscoundrel/orochi/internal/monitor"
)

type stubMonitor struct {
	summary monitor.Summary
	err     error
}

func (s stubMonitor) Run(context.Context) (monitor.Summary, error) {
	return s.summary, s.err
}

func TestMonitoringReturnsRunSummary(t *testing.T) {
	expected := monitor.Summary{Fetched: 12, Matched: 3, New: 2}
	service := stubMonitor{summary: expected}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	handleMonitoring(response, request, service, log.New(io.Discard, "", 0))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Type") != "application/json" {
		t.Fatal("response is not JSON")
	}
	var summary monitor.Summary
	if err := json.NewDecoder(response.Body).Decode(&summary); err != nil {
		t.Fatal(err)
	}
	if summary != expected {
		t.Fatalf("summary = %+v, want %+v", summary, expected)
	}
}

func TestMonitoringFailureDoesNotExposeInternalError(t *testing.T) {
	service := stubMonitor{err: errors.New("private storage details")}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	handleMonitoring(response, request, service, log.New(io.Discard, "", 0))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "monitoring failed" {
		t.Fatalf("unexpected public error: %q", body["error"])
	}
}

func TestMissingBucketReturnsConfigurationError(t *testing.T) {
	t.Setenv("EVENT_STORAGE_BUCKET", "")
	response := httptest.NewRecorder()

	FetchAndStoreEvents(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "monitoring is not configured" {
		t.Fatalf("unexpected configuration error: %q", body["error"])
	}
}

func TestUnsupportedMethodIsRejectedBeforeInitializingStorage(t *testing.T) {
	t.Setenv("EVENT_STORAGE_BUCKET", "")
	response := httptest.NewRecorder()

	FetchAndStoreEvents(response, httptest.NewRequest(http.MethodDelete, "/", nil))

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", response.Code)
	}
	if response.Header().Get("Allow") != "GET, POST" {
		t.Fatalf("allowed methods = %q, want GET, POST", response.Header().Get("Allow"))
	}
}
