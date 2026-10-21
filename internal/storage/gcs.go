package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	gcs "cloud.google.com/go/storage"
	"google.golang.org/api/googleapi"
)

const maxStateBytes int64 = 32 << 20

// ObjectStore isolates SDK I/O and conditional writes for offline testing.
type ObjectStore interface {
	Read(context.Context) ([]byte, int64, error)
	Write(context.Context, []byte, int64) error
}

type GCSRepository struct {
	object ObjectStore
}

func NewRepository(object ObjectStore) *GCSRepository {
	return &GCSRepository{object: object}
}

func NewGCSRepository(client *gcs.Client, bucket, object string) *GCSRepository {
	return NewRepository(&gcsObject{object: client.Bucket(bucket).Object(object)})
}

func (r *GCSRepository) Load(ctx context.Context) (State, int64, error) {
	data, generation, err := r.object.Read(ctx)
	if errors.Is(err, gcs.ErrObjectNotExist) {
		return EmptyState(), 0, nil
	}
	if err != nil {
		return State{}, 0, fmt.Errorf("read state: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, 0, fmt.Errorf("decode state: %w", err)
	}
	if state.Version != Version || !state.Initialized || state.Events == nil || state.LastSuccessfulCheck.IsZero() {
		return State{}, 0, fmt.Errorf("unsupported or invalid state")
	}
	for id, event := range state.Events {
		if id == "" || id != event.ID || event.FirstSeen.IsZero() {
			return State{}, 0, fmt.Errorf("invalid stored event identity")
		}
	}
	return state, generation, nil
}

func (r *GCSRepository) Save(ctx context.Context, state State, generation int64) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	if int64(len(data)) > maxStateBytes {
		return fmt.Errorf("state exceeds size limit")
	}
	return r.object.Write(ctx, data, generation)
}

type gcsObject struct {
	object *gcs.ObjectHandle
}

func (o *gcsObject) Read(ctx context.Context) ([]byte, int64, error) {
	reader, err := o.object.NewReader(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, maxStateBytes+1))
	if err == nil && int64(len(data)) > maxStateBytes {
		err = fmt.Errorf("state exceeds size limit")
	}
	return data, reader.Attrs.Generation, err
}

func (o *gcsObject) Write(ctx context.Context, data []byte, generation int64) error {
	condition := gcs.Conditions{GenerationMatch: generation}
	if generation == 0 {
		condition = gcs.Conditions{DoesNotExist: true}
	}
	writer := o.object.If(condition).NewWriter(ctx)
	writer.ContentType = "application/json"
	writer.ChunkSize = 0 // Small state snapshots do not need a resumable upload buffer.
	if _, err := writer.Write(data); err != nil {
		_ = writer.Close()
		return writeError(err)
	}
	return writeError(writer.Close())
}

func writeError(err error) error {
	var apiError *googleapi.Error
	if errors.As(err, &apiError) && apiError.Code == 412 {
		return ErrConflict
	}
	return err
}
