package mpk

import (
	"strings"
	"time"
)

// The feed uses Finnish month names and explicit offsets. The *local fields
// incorrectly label local wall times as UTC, so they are not used.
func parseDate(value string) (time.Time, error) {
	months := map[string]string{
		"tammik.":  "Jan",
		"helmik.":  "Feb",
		"maalisk.": "Mar",
		"huhtik.":  "Apr",
		"toukok.":  "May",
		"kesäk.":   "Jun",
		"heinäk.":  "Jul",
		"elok.":    "Aug",
		"syysk.":   "Sep",
		"lokak.":   "Oct",
		"marrask.": "Nov",
		"jouluk.":  "Dec",
	}
	value = strings.TrimSpace(value)
	if comma := strings.Index(value, ","); comma >= 0 {
		value = strings.TrimSpace(value[comma+1:])
	}
	for finnish, english := range months {
		value = strings.ReplaceAll(value, finnish, english)
	}
	parsedTime, err := time.Parse("02 Jan 2006 15.04.05 -0700", value)
	if err != nil {
		return time.Time{}, err
	}
	return parsedTime.UTC(), nil
}
