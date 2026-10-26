package storage

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	gcs "cloud.google.com/go/storage"
	"github.com/stscoundrel/orochi/internal/domain"
)

type memoryObject struct {
	data       []byte
	generation int64
	readError  error
}

func (o *memoryObject) Read(context.Context) ([]byte, int64, error) {
	return o.data, o.generation, o.readError
}

func (o *memoryObject) Write(_ context.Context, data []byte, generation int64) error {
	if generation != o.generation {
		return ErrConflict
	}
	o.data = data
	o.generation++
	return nil
}

func TestMissingObjectStartsWithAnUninitializedState(t *testing.T) {
	repository := NewRepository(&memoryObject{readError: gcs.ErrObjectNotExist})

	state, generation, err := repository.Load(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	if generation != 0 || state.Initialized || state.Events == nil || state.Version != Version {
		t.Fatalf("unexpected initial state: %+v, generation %d", state, generation)
	}
}

func TestStoredEventsSurviveSaveAndReload(t *testing.T) {
	object := &memoryObject{}
	repository := NewRepository(object)
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	event := domain.Event{
		ID:              "mpk:112615111274",
		Name:            "Kivääriammunta",
		City:            "Helsinki",
		Location:        "Helsinki, Santahamina",
		SpecialField:    "Ammunta",
		Status:          "Ilmo auki",
		URL:             "https://example.test/calendar",
		RegistrationURL: "https://example.test/signup",
		StartTime:       now.Add(24 * time.Hour),
		EndTime:         now.Add(27 * time.Hour),
		FirstSeen:       now,
	}
	state := State{
		Version:             Version,
		Initialized:         true,
		LastSuccessfulCheck: now,
		Events:              map[string]domain.Event{event.ID: event},
	}

	if err := repository.Save(context.Background(), state, 0); err != nil {
		t.Fatal(err)
	}
	// A new repository instance represents a later function invocation.
	reloaded, generation, err := NewRepository(object).Load(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reloaded, state) || generation != 1 {
		t.Fatalf("reloaded state = %+v (generation %d), want %+v", reloaded, generation, state)
	}
}

func TestUnreadableStateIsNotTreatedAsAnEmptyBaseline(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		object memoryObject
	}{
		{name: "permission denied", object: memoryObject{readError: errors.New("permission denied")}},
		{name: "corrupt JSON", object: memoryObject{data: []byte("broken")}},
		{name: "unsupported version", object: memoryObject{data: []byte(`{"version":2,"initialized":true,"last_successful_check":"2026-10-26T12:00:00Z","events":{}}`)}},
		{name: "incomplete state", object: memoryObject{data: []byte(`{"version":1,"initialized":true,"events":{}}`)}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			repository := NewRepository(&scenario.object)

			_, _, err := repository.Load(context.Background())

			if err == nil {
				t.Fatal("unreadable state was treated as a fresh installation")
			}
		})
	}
}
