package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ledgerwatch/erigon-lib/kv"
)

// EventOption represents a single option for an event
type EventOption struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	IsWinner  bool   `json:"isWinner"`
	VoteCount int64  `json:"voteCount"`
}

// ConsensusMetrics describes consensus-related info for an event
type ConsensusMetrics struct {
	TotalProvers       int     `json:"totalProvers"`
	ParticipationCount int     `json:"participationCount"`
	ParticipationRate  float64 `json:"participationRate"`
	WinningOptionId    int64   `json:"winningOptionId"`
	WinningOptionName  string  `json:"winningOptionName"`
	ConsensusRate      float64 `json:"consensusRate"`
}

// Event is the structure matching the JSON returned by the Replit API
type Event struct {
	EventID          int64            `json:"eventId"`
	EventName        string           `json:"eventName"`
	Description      *string          `json:"description,omitempty"`
	TargetDate       string           `json:"targetDate"`
	ClosedAt         string           `json:"closedAt"`
	Status           string           `json:"status"`
	Options          []EventOption    `json:"options"`
	ConsensusMetrics ConsensusMetrics `json:"consensusMetrics"`
	SourcesOfTruth   interface{}      `json:"sourcesOfTruth,omitempty"`
}

// PutEvent stores an event into the EventsBucket.
// key format: "event:<eventId>"
func PutEvent(ctx context.Context, tx kv.RwTx, e *Event) error {
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	key := []byte(fmt.Sprintf("event:%d", e.EventID))
	if err := tx.Put(EventsBucket, key, data); err != nil {
		return fmt.Errorf("put event: %w", err)
	}
	return nil
}

// GetEvent reads a single event by ID from a read-only tx
func GetEvent(ctx context.Context, tx kv.Tx, id int64) (*Event, error) {
	key := []byte(fmt.Sprintf("event:%d", id))
	data, err := tx.GetOne(EventsBucket, key)
	if err != nil {
		return nil, fmt.Errorf("db get: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("event %d not found", id)
	}
	var ev Event
	if err := json.Unmarshal(data, &ev); err != nil {
		return nil, fmt.Errorf("unmarshal event: %w", err)
	}
	return &ev, nil
}

// ListEvents enumerates all events present in EventsBucket. It is read-only.
func ListEvents(ctx context.Context, tx kv.Tx) ([]Event, error) {
	cur, err := tx.Cursor(EventsBucket)
	if err != nil {
		return nil, fmt.Errorf("cursor open: %w", err)
	}
	defer cur.Close()

	var out []Event
	for k, v, err := cur.First(); k != nil && err == nil; k, v, err = cur.Next() {
		var ev Event
		if unmarshalErr := json.Unmarshal(v, &ev); unmarshalErr == nil {
			out = append(out, ev)
		}
	}
	return out, nil
}
