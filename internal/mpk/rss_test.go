package mpk

import (
	"os"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("testdata/events.xml")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestParse(t *testing.T) {
	events, err := Parse(strings.NewReader(fixture(t)))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events: %d", len(events))
	}
	e := events[0]
	if e.ID != "mpk:112615111274" || e.Name != "Shooting & practice" || e.City != "Helsinki" || e.GUID != "opaque%2bID" {
		t.Fatalf("mapping: %+v", e)
	}
	if e.StartTime.Format(time.RFC3339) != "2026-11-21T07:00:00Z" {
		t.Fatalf("offset ignored: %v", e.StartTime)
	}
	if e.RegistrationURL != e.URL || !strings.Contains(e.URL, "&lang=fi") {
		t.Fatalf("link fallback: %+v", e)
	}
}

func TestInvalidSnapshots(t *testing.T) {
	base := fixture(t)
	cases := []string{
		"<html/>", "<rss/>", "<rss><channel>",
		strings.Replace(base, "<uniqueid> 112615111274 </uniqueid>", "", 1),
		strings.Replace(base, "la, 21 marrask. 2026 09.00.00 +0200", "invalid", 1),
		strings.Replace(base, "la, 21 marrask. 2026 12.00.00 +0200", "la, 20 marrask. 2026 12.00.00 +0200", 1),
		base + "<rss/>", base + "garbage",
		"<rss><channel><item/></channel></rss>",
	}
	for _, input := range cases {
		if _, err := Parse(strings.NewReader(input)); err == nil {
			t.Errorf("accepted invalid snapshot: %.100s", input)
		}
	}
	events, err := Parse(strings.NewReader("<rss><channel/></rss>"))
	if err != nil || len(events) != 0 {
		t.Fatalf("empty valid snapshot: %v", err)
	}
}

func TestFinnishMonthsAndOffsets(t *testing.T) {
	months := []string{"tammik.", "helmik.", "maalisk.", "huhtik.", "toukok.", "kesäk.", "heinäk.", "elok.", "syysk.", "lokak.", "marrask.", "jouluk."}
	for n, month := range months {
		date, err := parseDate("ma, 01 " + month + " 2026 09.00.00 +0300")
		if err != nil {
			t.Fatal(err)
		}
		if date.Month() != time.Month(n+1) || date.Hour() != 6 {
			t.Fatalf("date: %v", date)
		}
	}
}
