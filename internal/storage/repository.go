package storage

import (
	"context"
	"errors"
	"time"

	"github.com/stscoundrel/orochi/internal/domain"
)

var ErrConflict = errors.New("state generation conflict")

const Version = 1

type State struct {
	Version             int                     `json:"version"`
	Initialized         bool                    `json:"initialized"`
	LastSuccessfulCheck time.Time               `json:"last_successful_check"`
	Events              map[string]domain.Event `json:"events"`
}

func EmptyState() State {
	return State{
		Version: Version,
		Events:  make(map[string]domain.Event),
	}
}

type Repository interface {
	Load(context.Context) (State, int64, error)
	Save(context.Context, State, int64) error
}
