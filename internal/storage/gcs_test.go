package storage

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	gcs "cloud.google.com/go/storage"
	"google.golang.org/api/googleapi"
)

type fakeObject struct {
	data       []byte
	generation int64
	readErr    error
	writeErr   error
	written    []byte
	expected   int64
}

func (o *fakeObject) Read(context.Context) ([]byte, int64, error) {
	return o.data, o.generation, o.readErr
}

func (o *fakeObject) Write(_ context.Context, b []byte, g int64) error {
	o.written = b
	o.expected = g
	return o.writeErr
}

func TestMissingAndInvalidState(t *testing.T) {
	ctx := context.Background()
	o := &fakeObject{readErr: gcs.ErrObjectNotExist}
	r := NewRepository(o)
	state, generation, err := r.Load(ctx)
	if err != nil || generation != 0 || state.Initialized || state.Events == nil {
		t.Fatalf("missing: %+v %v", state, err)
	}
	for _, data := range []string{"broken", `{"version":2}`, `{"version":1,"initialized":true,"events":{}}`} {
		o.readErr = nil
		o.data = []byte(data)
		if _, _, err := r.Load(ctx); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
	o.readErr = errors.New("denied")
	if _, _, err := r.Load(ctx); err == nil {
		t.Fatal("access failure treated as missing")
	}
}

func TestRoundTripAndConditionalWrite(t *testing.T) {
	ctx := context.Background()
	o := &fakeObject{generation: 42}
	r := NewRepository(o)
	state := EmptyState()
	state.Initialized = true
	state.LastSuccessfulCheck = time.Now().UTC()
	if err := r.Save(ctx, state, 42); err != nil {
		t.Fatal(err)
	}
	if o.expected != 42 {
		t.Fatal("generation lost")
	}
	o.data = o.written
	loaded, g, err := r.Load(ctx)
	if err != nil || g != 42 || !loaded.LastSuccessfulCheck.Equal(state.LastSuccessfulCheck) {
		t.Fatalf("round trip: %+v %v", loaded, err)
	}
	var decoded State
	if err := json.Unmarshal(o.written, &decoded); err != nil {
		t.Fatal(err)
	}
	o.writeErr = ErrConflict
	if err := r.Save(ctx, state, 42); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflict: %v", err)
	}
}

func TestWriteError(t *testing.T) {
	if !errors.Is(writeError(&googleapi.Error{Code: 412}), ErrConflict) {
		t.Fatal("precondition not mapped")
	}
	failure := &googleapi.Error{Code: 403}
	if writeError(failure) != failure || writeError(nil) != nil {
		t.Fatal("write error changed")
	}
}
