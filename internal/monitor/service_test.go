package monitor

import (
	"bytes"
	"context"
	"errors"
	"log"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/stscoundrel/orochi/internal/domain"
	"github.com/stscoundrel/orochi/internal/storage"
)

type fakeSource struct {
	events []domain.Event
	err    error
}

func (s *fakeSource) Fetch(context.Context, time.Time) ([]domain.Event, error) {
	return s.events, s.err
}

type fakeRepo struct {
	state      storage.State
	loadErr    error
	saveErr    error
	saves      int
	conflicts  int
	onConflict func(*fakeRepo)
}

func (r *fakeRepo) Load(context.Context) (storage.State, int64, error) {
	state := r.state
	state.Events = maps.Clone(r.state.Events)
	return state, 1, r.loadErr
}

func (r *fakeRepo) Save(_ context.Context, s storage.State, _ int64) error {
	r.saves++
	if r.conflicts > 0 {
		r.conflicts--
		if r.onConflict != nil {
			r.onConflict(r)
		}
		return storage.ErrConflict
	}
	if r.saveErr != nil {
		return r.saveErr
	}
	r.state = s
	return nil
}

var testNow = time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)

func event(id string) domain.Event {
	return domain.Event{
		ID:           id,
		City:         "Helsinki",
		SpecialField: "Ammunta",
		StartTime:    testNow.Add(time.Hour),
		EndTime:      testNow.Add(2 * time.Hour),
	}
}

func setup() (*Service, *fakeSource, *fakeRepo, *bytes.Buffer) {
	source := &fakeSource{}
	repo := &fakeRepo{state: storage.EmptyState()}
	logs := &bytes.Buffer{}
	service := &Service{
		Source:     source,
		Repository: repo,
		Logger:     log.New(logs, "", 0),
		Now:        func() time.Time { return testNow },
	}
	return service, source, repo, logs
}

func TestLifecycle(t *testing.T) {
	svc, source, repo, logs := setup()
	ctx := context.Background()
	source.events = []domain.Event{event("mpk:1"), event("mpk:1")}
	result, err := svc.Run(ctx)
	if err != nil || !result.Baseline || result.New != 0 || result.Matched != 1 {
		t.Fatalf("baseline: %+v %v", result, err)
	}
	if strings.Contains(logs.String(), DiscoveryMarker) {
		t.Fatal("baseline alerted")
	}
	// Move time forward so this also detects accidental changes to FirstSeen.
	svc.Now = func() time.Time { return testNow.Add(time.Minute) }
	source.events = append(source.events, event("mpk:2"))
	result, err = svc.Run(ctx)
	if err != nil || result.New != 1 || result.Baseline {
		t.Fatalf("new: %+v %v", result, err)
	}
	if strings.Count(logs.String(), DiscoveryMarker) != 1 {
		t.Fatal("expected one marker")
	}
	source.events[0].Name = "Changed"
	logs.Reset()
	result, err = svc.Run(ctx)
	if err != nil || result.New != 0 || strings.Contains(logs.String(), DiscoveryMarker) {
		t.Fatalf("metadata alerted: %v", err)
	}
	source.events = nil
	if _, err := svc.Run(ctx); err != nil {
		t.Fatal(err)
	}
	source.events = []domain.Event{event("mpk:1")}
	result, err = svc.Run(ctx)
	if err != nil || result.New != 0 || len(repo.state.Events) != 2 {
		t.Fatalf("returning event: %+v %v", result, err)
	}
	if !repo.state.Events["mpk:2"].FirstSeen.Equal(testNow.Add(time.Minute)) {
		t.Fatal("new event first seen time is incorrect")
	}
	if !repo.state.Events["mpk:1"].FirstSeen.Equal(testNow) {
		t.Fatal("first seen lost")
	}
}

func TestEmptyBaselineAndFilters(t *testing.T) {
	svc, source, _, _ := setup()
	ctx := context.Background()
	result, err := svc.Run(ctx)
	if err != nil || !result.Baseline {
		t.Fatal("empty baseline failed")
	}
	good := event("good")
	good.City = " HELSINKI "
	good.SpecialField = " ammunta "
	good.Location = "Heslinki, Santahamina"
	other := event("other")
	other.City = "Upinniemi"
	cancelled := event("cancelled")
	cancelled.Status = "Peruutettu"
	past := event("past")
	past.EndTime = testNow.Add(-time.Second)
	sport := event("sport")
	sport.SpecialField = "Other"
	source.events = []domain.Event{good, other, cancelled, past, sport}
	result, err = svc.Run(ctx)
	if err != nil || result.Matched != 1 || result.New != 1 {
		t.Fatalf("filter: %+v %v", result, err)
	}
}

func TestFailuresDoNotAlert(t *testing.T) {
	for _, kind := range []string{"fetch", "load", "save", "conflict"} {
		t.Run(kind, func(t *testing.T) {
			svc, source, repo, logs := setup()
			repo.state.Initialized = true
			source.events = []domain.Event{event("new")}
			failure := errors.New("failure")
			switch kind {
			case "fetch":
				source.err = failure
			case "load":
				repo.loadErr = failure
			case "save":
				repo.saveErr = failure
			case "conflict":
				repo.conflicts = 10
			}
			if _, err := svc.Run(context.Background()); err == nil {
				t.Fatal("expected error")
			}
			if strings.Contains(logs.String(), DiscoveryMarker) {
				t.Fatal("failure alerted")
			}
			if len(repo.state.Events) != 0 {
				t.Fatal("failed run changed state")
			}
			if kind == "conflict" && repo.saves != 3 {
				t.Fatalf("retries: %d", repo.saves)
			}
		})
	}
}

func TestConflictRecomputesAgainstWinner(t *testing.T) {
	svc, source, repo, logs := setup()
	repo.state.Initialized = true
	source.events = []domain.Event{event("new")}
	repo.conflicts = 1
	repo.onConflict = func(repo *fakeRepo) {
		newEvent := event("new")
		newEvent.FirstSeen = testNow
		repo.state.Events[newEvent.ID] = newEvent
	}
	result, err := svc.Run(context.Background())
	if err != nil || result.New != 0 || repo.saves != 2 || strings.Contains(logs.String(), DiscoveryMarker) {
		t.Fatalf("conflict: %+v %v", result, err)
	}
}
