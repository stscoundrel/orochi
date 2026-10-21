package orochi

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	gcs "cloud.google.com/go/storage"
	"github.com/stscoundrel/orochi/internal/monitor"
	"github.com/stscoundrel/orochi/internal/mpk"
	"github.com/stscoundrel/orochi/internal/storage"
)

type memoryObject struct {
	data       []byte
	generation int64
}

func (o *memoryObject) Read(context.Context) ([]byte, int64, error) {
	if o.generation == 0 {
		return nil, 0, gcs.ErrObjectNotExist
	}
	return append([]byte(nil), o.data...), o.generation, nil
}

func (o *memoryObject) Write(_ context.Context, data []byte, g int64) error {
	if g != o.generation {
		return storage.ErrConflict
	}
	o.data = append([]byte(nil), data...)
	o.generation++
	return nil
}

func TestHTTPMonitoringLifecycle(t *testing.T) {
	fixture, err := os.ReadFile("internal/mpk/testdata/events.xml")
	if err != nil {
		t.Fatal(err)
	}
	body := string(fixture)
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, body) }))
	defer feed.Close()
	object := &memoryObject{}
	logs := &bytes.Buffer{}
	testLogger := log.New(logs, "", 0)
	service := &monitor.Service{
		Source: &mpk.Client{Endpoint: feed.URL, HTTP: feed.Client()}, Repository: storage.NewRepository(object), Logger: testLogger,
		Now: func() time.Time { return time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC) },
	}
	endpoint := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleMonitoring(w, r, service, testLogger)
	})
	run := func(status int, want string) {
		t.Helper()
		w := httptest.NewRecorder()
		endpoint.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
		if w.Code != status || !strings.Contains(w.Body.String(), want) {
			t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
		}
	}
	run(200, `"baseline":true`)
	if strings.Contains(logs.String(), monitor.DiscoveryMarker) {
		t.Fatal("baseline alerted")
	}
	itemStart := strings.Index(body, "<item>")
	itemEnd := strings.Index(body, "</item>") + len("</item>")
	second := strings.Replace(body[itemStart:itemEnd], "112615111274", "112615111275", 1)
	body = strings.Replace(body, "</channel>", second+"</channel>", 1)
	run(200, `"new":1`)
	run(200, `"new":0`)
	if strings.Count(logs.String(), monitor.DiscoveryMarker) != 1 {
		t.Fatal("expected exactly one discovery marker")
	}
	before := append([]byte(nil), object.data...)
	body = "<rss><channel><item/>"
	run(500, `"error"`)
	if !bytes.Equal(before, object.data) {
		t.Fatal("malformed snapshot modified state")
	}
}
