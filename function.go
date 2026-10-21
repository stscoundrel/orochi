// Package orochi exposes the scheduled Cloud Run function.
package orochi

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	gcs "cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/stscoundrel/orochi/internal/monitor"
	"github.com/stscoundrel/orochi/internal/mpk"
	"github.com/stscoundrel/orochi/internal/storage"
)

var (
	logger       = log.Default()
	clientMutex  sync.Mutex
	sharedClient *gcs.Client
)

func init() {
	functions.HTTP("FetchAndStoreEvents", FetchAndStoreEvents)
}

// Reuse the client across requests, but allow initialization to retry on failure.
func cloudClient(ctx context.Context) (*gcs.Client, error) {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	if sharedClient != nil {
		return sharedClient, nil
	}

	client, err := gcs.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	sharedClient = client
	return client, nil
}

type monitorRunner interface {
	Run(context.Context) (monitor.Summary, error)
}

// FetchAndStoreEvents is invoked by an authenticated scheduler. Infrastructure
// is responsible for enforcing invoker IAM permissions.
func FetchAndStoreEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		respond(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	bucketName := strings.TrimSpace(os.Getenv("EVENT_STORAGE_BUCKET"))
	if bucketName == "" {
		logger.Print("EVENT_STORAGE_BUCKET environment variable is not set")
		respond(w, http.StatusInternalServerError, map[string]string{"error": "monitoring is not configured"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 50*time.Second)
	defer cancel()

	client, err := cloudClient(ctx)
	if err != nil {
		logger.Printf("Failed to initialize Cloud Storage: %v", err)
		respond(w, http.StatusInternalServerError, map[string]string{"error": "storage initialization failed"})
		return
	}

	service := &monitor.Service{
		Source:     mpk.NewClient(),
		Repository: storage.NewGCSRepository(client, bucketName, "events.json"),
		Logger:     logger,
		Now:        time.Now,
	}

	handleMonitoring(w, r.WithContext(ctx), service, logger)
}

func handleMonitoring(w http.ResponseWriter, r *http.Request, service monitorRunner, logger *log.Logger) {
	summary, err := service.Run(r.Context())
	if err != nil {
		logger.Printf("Orochi check failed: %v", err)
		respond(w, http.StatusInternalServerError, map[string]string{"error": "monitoring failed"})
		return
	}

	respond(w, http.StatusOK, summary)
}

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		logger.Printf("Failed to encode response: %v", err)
	}
}
