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
	"strconv"
	"testing"

	gcs "cloud.google.com/go/storage"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

// Use the real SDK against a local server. This catches conditional-write and
// writer.Close failures that an in-memory repository cannot exercise.
func TestUploadProtectsExistingStateAndReportsFailures(t *testing.T) {
	data := []byte(`{"version":1}`)
	// Successful GCS upload responses include the checksum verified by the SDK.
	var checksum [4]byte
	binary.BigEndian.PutUint32(checksum[:], crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli)))
	metadata := fmt.Sprintf(`{"name":"events.json","bucket":"bucket","generation":"43","crc32c":"%s"}`,
		base64.StdEncoding.EncodeToString(checksum[:]))

	for _, scenario := range []struct {
		name       string
		generation int64
		status     int
	}{
		{name: "create only if absent", generation: 0, status: http.StatusOK},
		{name: "update only the version we read", generation: 42, status: http.StatusOK},
		{name: "another writer changed the object", generation: 42, status: http.StatusPreconditionFailed},
		{name: "upload denied when writer closes", generation: 42, status: http.StatusForbidden},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			called := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				called = true
				expectedGeneration := strconv.FormatInt(scenario.generation, 10)
				if generation := request.URL.Query().Get("ifGenerationMatch"); generation != expectedGeneration {
					t.Errorf("upload precondition = %q, want %q", generation, expectedGeneration)
				}
				if _, err := io.Copy(io.Discard, request.Body); err != nil {
					t.Error(err)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(scenario.status)
				if scenario.status == http.StatusOK {
					_, _ = io.WriteString(w, metadata)
				} else {
					_, _ = fmt.Fprintf(w, `{"error":{"code":%d,"message":"upload failed"}}`, scenario.status)
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

			err = object.Write(context.Background(), data, scenario.generation)

			if !called {
				t.Fatal("no upload was attempted")
			}
			switch scenario.status {
			case http.StatusOK:
				if err != nil {
					t.Fatalf("successful upload failed: %v", err)
				}
			case http.StatusPreconditionFailed:
				if !errors.Is(err, ErrConflict) {
					t.Fatalf("competing write error = %v, want ErrConflict", err)
				}
			case http.StatusForbidden:
				var apiError *googleapi.Error
				if !errors.As(err, &apiError) || apiError.Code != http.StatusForbidden {
					t.Fatalf("upload error = %v, want permission denied", err)
				}
			}
		})
	}
}
