// Are we Helsinki yet? Checks whether the RSS API respects its Helsinki location filter.
// Run from the repository root:
//
//	go run ./scripts/are-we-helsinki-yet
//
// Prints event counts and any other cities returned. An empty feed or a failed
// request is inconclusive. Exit codes: 0 for all Helsinki, 1 for mismatches,
// 2 for an inconclusive check. go run reports a nonzero program exit as an error.
package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/stscoundrel/orochi/internal/mpk"
)

func main() {
	fmt.Println("Are we Helsinki yet?")

	events, err := mpk.NewClient().Fetch(context.Background(), time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not check the feed:", err)
		os.Exit(2)
	}
	if len(events) == 0 {
		fmt.Println("No events returned. We cannot tell yet.")
		os.Exit(2)
	}

	otherCities := make(map[string]int)
	helsinki := 0
	for _, event := range events {
		city := strings.TrimSpace(event.City)
		if strings.EqualFold(city, "Helsinki") {
			helsinki++
			continue
		}
		if city == "" {
			city = "(missing city)"
		}
		otherCities[city]++
	}

	if helsinki == len(events) {
		fmt.Printf("Yes! All %d events are Helsinki based. Location filtering appears to work.\n", len(events))
		return
	}

	fmt.Printf("Not yet. Checked %d events: %d Helsinki, %d elsewhere or missing a city.\n",
		len(events), helsinki, len(events)-helsinki)
	cities := make([]string, 0, len(otherCities))
	for city := range otherCities {
		cities = append(cities, city)
	}
	sort.Strings(cities)
	for _, city := range cities {
		fmt.Printf("  %s: %d\n", city, otherCities[city])
	}
	os.Exit(1)
}
