package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ledgerwatch/erigon-lib/kv"
)

// EventOption represents a single option for an event
type EventOption struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	IsWinner       bool    `json:"isWinner"`
	VoteCount      int     `json:"voteCount"`
	VotePercentage float64 `json:"votePercentage"`
}

// ConsensusMetrics describes consensus-related info for an event
type ConsensusMetrics struct {
	TotalProvers       int     `json:"totalProvers"`
	ParticipationCount int     `json:"participationCount"`
	ParticipationRate  float64 `json:"participationRate"`
	WinningOptionId    int64   `json:"winningOptionId"`
	WinningOptionName  string  `json:"winningOptionName"`
	WinningOptionVotes int     `json:"winningOptionVotes"`
	ConsensusRate      float64 `json:"consensusRate"`
}

// TimingInfo contains time-related information about an event
type TimingInfo struct {
	TargetDate                    string `json:"targetDate"`
	ClosedAt                      string `json:"closedAt"`
	DurationMinutes              int    `json:"durationMinutes"`
	AverageResponseTimeSeconds   int    `json:"averageResponseTimeSeconds"`
}

// RewardsInfo contains reward-related information
type RewardsInfo struct {
	TotalDistributed float64 `json:"totalDistributed"`
	CorrectProvers   int     `json:"correctProvers"`
}

// ProvenanceInfo contains information about the truth source
type ProvenanceInfo struct {
	SourcesOfTruth    []string `json:"sourcesOfTruth"`
	SourceType        string   `json:"sourceType"`
	OriginalSourceUrl string   `json:"originalSourceUrl,omitempty"`
}

// VerificationInfo contains cryptographic verification details
type VerificationInfo struct {
	Signature     string `json:"signature"`
	SignerAddress string `json:"signerAddress"`
	MessageHash   string `json:"messageHash"`
	SignedAt      string `json:"signedAt"`
	Algorithm     string `json:"algorithm"`
	Standard      string `json:"standard"`
}

// Event is the structure matching the JSON returned by the API
type Event struct {
	APIVersion       string           `json:"apiVersion"`
	EventID          int64            `json:"eventId"`
	EventName        string           `json:"eventName"`
	Description      string           `json:"description"`
	Status           string           `json:"status"`
	Timing           TimingInfo       `json:"timing"`
	Options          [2]EventOption   `json:"options"`
	Consensus        ConsensusMetrics `json:"consensus"`
	Rewards          RewardsInfo      `json:"rewards"`
	Provenance       ProvenanceInfo   `json:"provenance"`
	Verification     VerificationInfo `json:"verification"`
}

// PutEvent stores an event into the EventsBucket.
// key format: "event:<eventId>"
func PutEvent(tx kv.RwTx, e *Event) error {
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
func GetEvent(tx kv.Tx, id int64) (*Event, error) {
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
