package mpk

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stscoundrel/orochi/internal/domain"
)

func readFeedFixture(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("testdata/events.xml")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestParseMPKFeed(t *testing.T) {
	events, err := Parse(strings.NewReader(readFeedFixture(t)))

	if err != nil {
		t.Fatal(err)
	}
	expected := []domain.Event{
		{
			ID:           "mpk:112615111274",
			GUID:         "opaque%2bID",
			Name:         "Kivääri & pistooli",
			City:         "Helsinki",
			Location:     "Helsinki, Santahamina",
			SpecialField: "Ammunta",
			Status:       "Ilmo auki",
			// Use the explicit +0200 offset, not the feed's misleading *local value.
			StartTime:       time.Date(2026, 11, 21, 7, 0, 0, 0, time.UTC),
			EndTime:         time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC),
			URL:             "https://example.test/calendar?id=opaque%2bID&lang=fi",
			RegistrationURL: "https://example.test/signup?id=112615111274&lang=fi",
		},
		{
			ID:           "mpk:112715111275",
			GUID:         "announced-event",
			Name:         "Kesän ammuntaharjoitus",
			City:         "Helsinki",
			Location:     "Helsinki",
			SpecialField: "Ammunta",
			Status:       "Julkaistu",
			// Summer time has a different offset; the calendar link is the fallback.
			StartTime:       time.Date(2027, 6, 1, 6, 0, 0, 0, time.UTC),
			EndTime:         time.Date(2027, 6, 1, 9, 0, 0, 0, time.UTC),
			URL:             "https://example.test/calendar?id=announced-event",
			RegistrationURL: "https://example.test/calendar?id=announced-event",
		},
	}
	if !reflect.DeepEqual(events, expected) {
		t.Fatalf("parsed events = %+v, want %+v", events, expected)
	}
}

func TestInvalidFeedReturnsNoPartialEvents(t *testing.T) {
	feed := readFeedFixture(t)
	for _, scenario := range []struct {
		name string
		body string
	}{
		{name: "upstream returned HTML", body: "<html>Service unavailable</html>"},
		{name: "truncated XML", body: "<rss><channel><item>"},
		{name: "missing channel", body: "<rss/>"},
		{name: "second event has no ID", body: strings.Replace(feed, "<uniqueid>112715111275</uniqueid>", "", 1)},
		{name: "second event has an invalid date", body: strings.Replace(feed, "ti, 01 kesäk. 2027 09.00.00 +0300", "invalid", 1)},
		{name: "event ends before it starts", body: strings.Replace(feed, "la, 21 marrask. 2026 12.00.00 +0200", "la, 20 marrask. 2026 12.00.00 +0200", 1)},
		{name: "unexpected second document", body: feed + "<rss/>"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			events, err := Parse(strings.NewReader(scenario.body))

			if err == nil {
				t.Fatal("invalid feed was accepted")
			}
			if len(events) != 0 {
				t.Fatalf("invalid feed returned %d partial events", len(events))
			}
		})
	}
}

func TestEmptyFeedIsValid(t *testing.T) {
	events, err := Parse(strings.NewReader("<rss><channel/></rss>"))

	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("empty feed returned %d events", len(events))
	}
}
