package storage

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	gcs "cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

// Exercise the real SDK against a local server, including errors surfaced only
// when the upload writer closes. No cloud credentials or services are used.
func TestSDKConditionalWrites(t *testing.T) {
	for _, tc := range []struct {
		name       string
		generation int64
		status     int
		conflict   bool
	}{
		{"create", 0, 200, false},
		{"update", 42, 200, false},
		{"conflict", 42, 412, true},
		{"close failure", 42, 403, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var called bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				want := "42"
				if tc.generation == 0 {
					want = "0"
				}
				if got := r.URL.Query().Get("ifGenerationMatch"); got != want {
					t.Errorf("precondition=%q want %q", got, want)
				}
				if _, err := io.Copy(io.Discard, r.Body); err != nil {
					t.Error(err)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				if tc.status == 200 {
					var checksum [4]byte
					binary.BigEndian.PutUint32(checksum[:], crc32.Checksum([]byte(`{"version":1}`), crc32.MakeTable(crc32.Castagnoli)))
					_, _ = fmt.Fprintf(w, `{"name":"events.json","bucket":"bucket","generation":"43","crc32c":"%s"}`, base64.StdEncoding.EncodeToString(checksum[:]))
				} else {
					_, _ = fmt.Fprintf(w, `{"error":{"code":%d,"message":"test failure"}}`, tc.status)
				}
			}))
			defer server.Close()
			client, err := gcs.NewClient(context.Background(),
				option.WithEndpoint(server.URL),
				option.WithoutAuthentication(),
				option.WithHTTPClient(server.Client()),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			object := &gcsObject{object: client.Bucket("bucket").Object("events.json")}
			err = object.Write(context.Background(), []byte(`{"version":1}`), tc.generation)
			if !called {
				t.Fatal("SDK did not make request")
			}
			if tc.status == 200 && err != nil {
				t.Fatal(err)
			}
			if tc.status != 200 && err == nil {
				t.Fatal("upload failure ignored")
			}
			if tc.conflict && !errors.Is(err, ErrConflict) {
				t.Fatalf("conflict: %v", err)
			}
		})
	}
}
