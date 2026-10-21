package monitor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/stscoundrel/orochi/internal/domain"
	"github.com/stscoundrel/orochi/internal/storage"
)

const DiscoveryMarker = "Orochi found new events"
const maxSaveAttempts = 3

type Source interface {
	Fetch(context.Context, time.Time) ([]domain.Event, error)
}

type Service struct {
	Source     Source
	Repository storage.Repository
	Logger     *log.Logger
	Now        func() time.Time
}

type Summary struct {
	Fetched  int  `json:"fetched"`
	Matched  int  `json:"matched"`
	New      int  `json:"new"`
	Baseline bool `json:"baseline"`
}

func (s *Service) Run(ctx context.Context) (Summary, error) {
	now := s.Now().UTC()
	events, err := s.Source.Fetch(ctx, now)
	if err != nil {
		return Summary{}, fmt.Errorf("fetch events: %w", err)
	}

	matchingEvents := filterEvents(events, now)
	summary := Summary{
		Fetched: len(events),
		Matched: len(matchingEvents),
	}

	for attempt := 0; attempt < maxSaveAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return Summary{}, err
		}

		state, generation, err := s.Repository.Load(ctx)
		if err != nil {
			return Summary{}, fmt.Errorf("load event state: %w", err)
		}

		baseline := !state.Initialized
		var newEvents []domain.Event
		for _, event := range matchingEvents {
			previousEvent, exists := state.Events[event.ID]
			if exists {
				event.FirstSeen = previousEvent.FirstSeen
			} else {
				event.FirstSeen = now
			}
			state.Events[event.ID] = event

			if !exists && !baseline {
				newEvents = append(newEvents, event)
			}
		}

		state.Initialized = true
		state.LastSuccessfulCheck = now

		err = s.Repository.Save(ctx, state, generation)
		if errors.Is(err, storage.ErrConflict) {
			// Another run changed the state. Reload before deciding which events are new.
			continue
		}
		if err != nil {
			return Summary{}, fmt.Errorf("save event state: %w", err)
		}

		summary.New = len(newEvents)
		summary.Baseline = baseline

		for _, event := range newEvents {
			s.Logger.Printf("New MPK event: %s, %s, %s, %s", event.Name,
				event.StartTime.Format(time.RFC3339), event.Location, event.RegistrationURL)
		}
		if len(newEvents) > 0 {
			s.Logger.Print(DiscoveryMarker)
		}
		s.Logger.Printf("Orochi check completed: %d fetched, %d matched, %d new, baseline=%t",
			summary.Fetched, summary.Matched, summary.New, summary.Baseline)

		return summary, nil
	}

	return Summary{}, fmt.Errorf("state contention after %d attempts: %w", maxSaveAttempts, storage.ErrConflict)
}

func filterEvents(events []domain.Event, now time.Time) []domain.Event {
	var matchingEvents []domain.Event
	seenIDs := make(map[string]bool)

	for _, event := range events {
		if !matches(event, now) || seenIDs[event.ID] {
			continue
		}
		seenIDs[event.ID] = true
		matchingEvents = append(matchingEvents, event)
	}

	return matchingEvents
}

func matches(event domain.Event, now time.Time) bool {
	if !strings.EqualFold(strings.TrimSpace(event.City), "Helsinki") {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(event.SpecialField), "Ammunta") {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(event.Status), "Peruutettu") {
		return false
	}
	return !event.EndTime.Before(now)
}
