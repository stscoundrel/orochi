package mpk

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/stscoundrel/orochi/internal/domain"
)

// Parse validates the entire snapshot before returning any events.
func Parse(r io.Reader) ([]domain.Event, error) {
	decoder := xml.NewDecoder(r)
	var rssFeed feed
	if err := decoder.Decode(&rssFeed); err != nil {
		return nil, fmt.Errorf("decode RSS: %w", err)
	}
	if rssFeed.Channel == nil {
		return nil, fmt.Errorf("RSS channel missing")
	}
	// Reject concatenated documents and garbage after the RSS root.
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("RSS trailing data: %w", err)
		}
		if data, ok := token.(xml.CharData); ok && strings.TrimSpace(string(data)) == "" {
			continue
		}
		if _, ok := token.(xml.Comment); ok {
			continue
		}
		return nil, fmt.Errorf("unexpected content after RSS root")
	}
	return mapToDomainEvents(rssFeed.Channel.Items)
}

func mapToDomainEvents(items []item) ([]domain.Event, error) {
	events := make([]domain.Event, 0, len(items))
	for index, rssItem := range items {
		if rssItem.Event == nil {
			return nil, fmt.Errorf("item %d: event missing", index)
		}
		raw := rssItem.Event
		id := strings.TrimSpace(raw.ID)
		if id == "" {
			return nil, fmt.Errorf("item %d: uniqueid missing", index)
		}
		start, err := parseDate(raw.Start)
		if err != nil {
			return nil, fmt.Errorf("event %s start: %w", id, err)
		}
		end, err := parseDate(raw.End)
		if err != nil {
			return nil, fmt.Errorf("event %s end: %w", id, err)
		}
		if end.Before(start) {
			return nil, fmt.Errorf("event %s ends before it starts", id)
		}
		name := strings.TrimSpace(raw.Name)
		if name == "" {
			name = strings.TrimSpace(rssItem.Title)
		}
		link := strings.TrimSpace(rssItem.Link)
		registration := strings.TrimSpace(raw.Registration.Link)
		if registration == "" {
			registration = link
		}
		events = append(events, domain.Event{
			ID:              "mpk:" + id,
			GUID:            strings.TrimSpace(rssItem.GUID),
			Name:            name,
			City:            strings.TrimSpace(raw.City),
			Location:        strings.TrimSpace(raw.Location),
			SpecialField:    strings.TrimSpace(raw.SpecialField),
			Status:          strings.TrimSpace(raw.Status),
			StartTime:       start,
			EndTime:         end,
			URL:             link,
			RegistrationURL: registration,
		})
	}
	return events, nil
}
