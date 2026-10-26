package monitor

import (
	"bytes"
	"context"
	"errors"
	"log"
	"maps"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stscoundrel/orochi/internal/domain"
	"github.com/stscoundrel/orochi/internal/storage"
)

var checkTime = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

type stubSource struct {
	events []domain.Event
	err    error
}

func (s *stubSource) Fetch(context.Context, time.Time) ([]domain.Event, error) {
	return s.events, s.err
}

type memoryRepository struct {
	state              storage.State
	loadError          error
	saveError          error
	conflictsRemaining int
	onConflict         func()
}

func (r *memoryRepository) Load(context.Context) (storage.State, int64, error) {
	state := r.state
	state.Events = maps.Clone(r.state.Events)
	return state, 1, r.loadError
}

func (r *memoryRepository) Save(_ context.Context, state storage.State, _ int64) error {
	if r.conflictsRemaining > 0 {
		r.conflictsRemaining--
		if r.onConflict != nil {
			r.onConflict()
		}
		return storage.ErrConflict
	}
	if r.saveError != nil {
		return r.saveError
	}
	r.state = state
	return nil
}

func shootingEvent(id string) domain.Event {
	return domain.Event{
		ID:              id,
		Name:            "Kivääriammunta",
		City:            "Helsinki",
		Location:        "Helsinki, Santahamina",
		SpecialField:    "Ammunta",
		Status:          "Julkaistu",
		StartTime:       checkTime.Add(24 * time.Hour),
		EndTime:         checkTime.Add(27 * time.Hour),
		RegistrationURL: "https://example.test/signup/" + id,
	}
}

func knownState(events ...domain.Event) storage.State {
	state := storage.EmptyState()
	state.Initialized = true
	state.LastSuccessfulCheck = checkTime.Add(-24 * time.Hour)
	for _, event := range events {
		event.FirstSeen = state.LastSuccessfulCheck
		state.Events[event.ID] = event
	}
	return state
}

func testService(source *stubSource, repository *memoryRepository, logs *bytes.Buffer) *Service {
	return &Service{
		Source:     source,
		Repository: repository,
		Logger:     log.New(logs, "", 0),
		Now:        func() time.Time { return checkTime },
	}
}

func TestFirstCheckStoresBaselineWithoutAlerting(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		events []domain.Event
	}{
		{name: "existing events", events: []domain.Event{shootingEvent("mpk:1")}},
		{name: "empty feed"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			source := &stubSource{events: scenario.events}
			repository := &memoryRepository{state: storage.EmptyState()}
			logs := &bytes.Buffer{}
			service := testService(source, repository, logs)

			summary, err := service.Run(context.Background())

			if err != nil {
				t.Fatal(err)
			}
			if !summary.Baseline || summary.New != 0 {
				t.Fatalf("first check should be a silent baseline: %+v", summary)
			}
			if !repository.state.Initialized || len(repository.state.Events) != len(scenario.events) {
				t.Fatalf("baseline was not stored: %+v", repository.state)
			}
			if !repository.state.LastSuccessfulCheck.Equal(checkTime) {
				t.Fatal("successful check time was not saved")
			}
			for _, event := range scenario.events {
				stored := repository.state.Events[event.ID]
				if stored.Name != event.Name || !stored.FirstSeen.Equal(checkTime) {
					t.Fatalf("baseline event was not saved correctly: %+v", stored)
				}
			}
			if strings.Contains(logs.String(), DiscoveryMarker) {
				t.Fatal("baseline sent a discovery alert")
			}
		})
	}
}

func TestOnlyUnseenEventsTriggerDiscovery(t *testing.T) {
	existing := shootingEvent("mpk:1")
	repository := &memoryRepository{state: knownState(existing)}
	existing.Name = "Updated course name"
	newEvent := shootingEvent("mpk:2")
	source := &stubSource{events: []domain.Event{existing, newEvent, newEvent}}
	logs := &bytes.Buffer{}
	service := testService(source, repository, logs)

	summary, err := service.Run(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	expected := Summary{Fetched: 3, Matched: 2, New: 1}
	if summary != expected {
		t.Fatalf("summary = %+v, want %+v", summary, expected)
	}
	if len(repository.state.Events) != 2 {
		t.Fatalf("stored %d events, want two distinct IDs", len(repository.state.Events))
	}
	storedExisting := repository.state.Events[existing.ID]
	if storedExisting.Name != existing.Name {
		t.Fatal("existing event metadata was not updated")
	}
	if !storedExisting.FirstSeen.Equal(checkTime.Add(-24 * time.Hour)) {
		t.Fatal("updating metadata changed the original first-seen time")
	}
	if !repository.state.Events[newEvent.ID].FirstSeen.Equal(checkTime) {
		t.Fatal("new event has the wrong first-seen time")
	}
	if strings.Count(logs.String(), DiscoveryMarker) != 1 {
		t.Fatalf("expected one discovery marker; logs: %s", logs.String())
	}
	if !strings.Contains(logs.String(), newEvent.RegistrationURL) {
		t.Fatal("discovery log is missing the signup link")
	}
	if strings.Contains(logs.String(), existing.Name) {
		t.Fatal("metadata update was logged as a discovery")
	}

	// The same feed on the next check must not alert again.
	logs.Reset()
	summary, err = service.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.New != 0 || strings.Contains(logs.String(), DiscoveryMarker) {
		t.Fatalf("unchanged feed alerted again: %+v; logs: %s", summary, logs.String())
	}
}

func TestDisappearingEventsRemainKnownWhenTheyReturn(t *testing.T) {
	event := shootingEvent("mpk:1")
	repository := &memoryRepository{state: knownState(event)}
	source := &stubSource{}
	logs := &bytes.Buffer{}
	service := testService(source, repository, logs)

	if _, err := service.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, exists := repository.state.Events[event.ID]; !exists {
		t.Fatal("an event disappearing from the feed erased its history")
	}

	source.events = []domain.Event{event}
	summary, err := service.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.New != 0 || strings.Contains(logs.String(), DiscoveryMarker) {
		t.Fatal("a returning event triggered another discovery alert")
	}
}

func TestFiltersForUpcomingHelsinkiShootingEvents(t *testing.T) {
	announced := shootingEvent("announced")
	open := shootingEvent("open")
	open.Status = "Ilmo auki"
	closed := shootingEvent("closed")
	closed.Status = "Ilmo kiinni"
	otherCity := shootingEvent("other-city")
	otherCity.City = "Upinniemi"
	otherSubject := shootingEvent("other-subject")
	otherSubject.SpecialField = "Ensiapu"
	cancelled := shootingEvent("cancelled")
	cancelled.Status = "Peruutettu"
	ended := shootingEvent("ended")
	ended.StartTime = checkTime.Add(-3 * time.Hour)
	ended.EndTime = checkTime.Add(-time.Hour)
	venueTypo := shootingEvent("venue-typo")
	venueTypo.City = " HELSINKI "
	venueTypo.SpecialField = " ammunta "
	venueTypo.Location = "Heslinki, Santahamina"

	events := filterEvents([]domain.Event{announced, open, closed, otherCity, otherSubject, cancelled, ended, venueTypo}, checkTime)

	var ids []string
	for _, event := range events {
		ids = append(ids, event.ID)
	}
	expected := []string{"announced", "open", "closed", "venue-typo"}
	if !reflect.DeepEqual(ids, expected) {
		t.Fatalf("matching events = %v, want %v", ids, expected)
	}
}

func TestFailedChecksPreserveStateAndDoNotAlert(t *testing.T) {
	failure := errors.New("operation failed")
	for _, scenario := range []struct {
		name          string
		fetchError    error
		loadError     error
		saveError     error
		conflicts     int
		expectedError error
	}{
		{name: "feed unavailable", fetchError: failure, expectedError: failure},
		{name: "state unreadable", loadError: failure, expectedError: failure},
		{name: "state write failed", saveError: failure, expectedError: failure},
		{name: "repeated competing writes", conflicts: 10, expectedError: storage.ErrConflict},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			original := knownState(shootingEvent("mpk:1"))
			source := &stubSource{events: []domain.Event{shootingEvent("mpk:2")}, err: scenario.fetchError}
			repository := &memoryRepository{
				state:              original,
				loadError:          scenario.loadError,
				saveError:          scenario.saveError,
				conflictsRemaining: scenario.conflicts,
			}
			logs := &bytes.Buffer{}
			service := testService(source, repository, logs)

			_, err := service.Run(context.Background())

			if !errors.Is(err, scenario.expectedError) {
				t.Fatalf("error = %v, want %v", err, scenario.expectedError)
			}
			if !reflect.DeepEqual(repository.state, original) {
				t.Fatal("failed check changed the saved state")
			}
			if strings.Contains(logs.String(), DiscoveryMarker) {
				t.Fatal("failed check sent a discovery alert")
			}
		})
	}
}

func TestCompetingRunDoesNotCauseDuplicateDiscovery(t *testing.T) {
	event := shootingEvent("mpk:1")
	source := &stubSource{events: []domain.Event{event}}
	repository := &memoryRepository{state: knownState(), conflictsRemaining: 1}
	repository.onConflict = func() {
		// Another invocation saved this event while our invocation was working.
		repository.state = knownState(event)
	}
	logs := &bytes.Buffer{}
	service := testService(source, repository, logs)

	summary, err := service.Run(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	if summary.New != 0 || strings.Contains(logs.String(), DiscoveryMarker) {
		t.Fatal("event already saved by another run triggered a duplicate alert")
	}
	if !repository.state.LastSuccessfulCheck.Equal(checkTime) {
		t.Fatal("check did not finish after reloading the competing write")
	}
}
