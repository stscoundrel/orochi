package domain

import "time"

type Event struct {
	ID              string    `json:"id"`
	GUID            string    `json:"guid"`
	Name            string    `json:"name"`
	City            string    `json:"city"`
	Location        string    `json:"location"`
	SpecialField    string    `json:"special_field"`
	Status          string    `json:"status"`
	StartTime       time.Time `json:"start_time"`
	EndTime         time.Time `json:"end_time"`
	URL             string    `json:"url"`
	RegistrationURL string    `json:"registration_url"`
	FirstSeen       time.Time `json:"first_seen"`
}
