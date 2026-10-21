package orochi

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stscoundrel/orochi/internal/monitor"
)

type fakeRunner struct {
	result monitor.Summary
	err    error
	calls  int
}

func (f *fakeRunner) Run(context.Context) (monitor.Summary, error) {
	f.calls++
	return f.result, f.err
}

func TestHandler(t *testing.T) {
	for _, tc := range []struct {
		method string
		err    error
		status int
	}{
		{"GET", nil, 200},
		{"POST", nil, 200},
		{"GET", errors.New("private storage error"), 500},
	} {
		run := &fakeRunner{result: monitor.Summary{Fetched: 3, Matched: 2, New: 1}, err: tc.err}
		logs := &bytes.Buffer{}
		w := httptest.NewRecorder()
		handleMonitoring(w, httptest.NewRequest(tc.method, "/", nil), run, log.New(logs, "", 0))
		if w.Code != tc.status || w.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("response: %d %s", w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "private") {
			t.Fatal("internal error leaked")
		}
		if tc.status == 200 && !strings.Contains(w.Body.String(), `"new":1`) {
			t.Fatal("summary missing")
		}
	}
}

func TestMissingConfiguration(t *testing.T) {
	t.Setenv("EVENT_STORAGE_BUCKET", "")
	w := httptest.NewRecorder()
	FetchAndStoreEvents(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != 500 {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestUnsupportedMethod(t *testing.T) {
	t.Setenv("EVENT_STORAGE_BUCKET", "")
	response := httptest.NewRecorder()
	FetchAndStoreEvents(response, httptest.NewRequest(http.MethodDelete, "/", nil))
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET, POST" {
		t.Fatalf("unexpected method response: %d, %v", response.Code, response.Header())
	}
}
