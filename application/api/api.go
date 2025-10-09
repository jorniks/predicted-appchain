package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/0xAtelerix/sdk/gosdk/rpc"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/example/application"
)

type CustomRPC struct {
	rpcServer *rpc.StandardRPCServer
	db        kv.RoDB
}

func NewCustomRPC(rpcServer *rpc.StandardRPCServer, db kv.RoDB) *CustomRPC {
	return &CustomRPC{
		rpcServer: rpcServer,
		db:        db,
	}
}

func (c *CustomRPC) AddRPCMethods() {
	c.rpcServer.AddMethod("getEvent", c.GetEvent)
	c.rpcServer.AddMethod("listEvents", c.ListEvents)
	c.rpcServer.AddMethod("syncEvents", c.SyncEvents)
}

// ----------------- New: Event RPC handlers -----------------

type GetEventRequest struct {
	EventID int64 `json:"eventId"`
}

// GetEvent returns single event by id
func (c *CustomRPC) GetEvent(ctx context.Context, params []any) (any, error) {
	if len(params) == 0 {
		return nil, application.ErrMissingParameters
	}

	paramBytes, err := json.Marshal(params[0])
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parameter: %w", err)
	}

	var req GetEventRequest
	if err := json.Unmarshal(paramBytes, &req); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if c.db == nil {
		return nil, application.ErrDatabaseNotAvailable
	}

	tx, err := c.db.BeginRo(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin ro: %w", err)
	}
	defer tx.Rollback()

	ev, err := application.GetEvent(tx, req.EventID)
	if err != nil {
		return nil, err
	}
	return ev, nil
}

// ListEvents returns all stored events
func (c *CustomRPC) ListEvents(ctx context.Context, params []any) (any, error) {
	if c.db == nil {
		return nil, application.ErrDatabaseNotAvailable
	}

	tx, err := c.db.BeginRo(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin ro: %w", err)
	}
	defer tx.Rollback()

	events, err := application.ListEvents(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	return events, nil
}

// SyncEvents fetches events from external API and returns sync status
func (c *CustomRPC) SyncEvents(ctx context.Context, params []any) (any, error) {
	// Define response structure
	type SyncResponse struct {
		Success      bool   `json:"success"`
		Message      string `json:"message,omitempty"`
		TotalFromAPI int    `json:"totalFromAPI,omitempty"`
		TotalSynced  int    `json:"totalSynced,omitempty"`
		NotSynced    int    `json:"notSynced,omitempty"`
	}

	// Fetch events from external API
	resp, err := http.Get("https://predicted-provers.replit.app/api/blockchain/concluded-events")
	if err != nil {
		return false, fmt.Errorf("failed to fetch events: %w", err)
	}
	defer resp.Body.Close()

	// Parse response structure matching the exact API response format
	var apiResponse struct {
		Success bool              `json:"success"`
		Count   int              `json:"count"`
		Events  []*application.Event `json:"events"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	if !apiResponse.Success {
		return false, fmt.Errorf("API returned failure status")
	}

	// Get existing event IDs to avoid duplicates
	tx, err := c.db.BeginRo(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to begin read transaction: %w", err)
	}
	existingEvents, err := application.ListEvents(ctx, tx)
	tx.Rollback()
	if err != nil {
		return false, fmt.Errorf("failed to list existing events: %w", err)
	}

	// Create map of existing event IDs for quick lookup
	existingEventIDs := make(map[int64]bool)
	for _, event := range existingEvents {
		existingEventIDs[event.EventID] = true
	}

	// Filter out duplicates
	var newEvents []*application.Event
	for _, event := range apiResponse.Events {
		if !existingEventIDs[event.EventID] {
			newEvents = append(newEvents, event)
		}
	}

	// If no new events to add, return early with status message
	if len(newEvents) == 0 {
		return SyncResponse{
			Success: true,
			Message: "Events not synced because no new event was detected",
			TotalFromAPI: len(apiResponse.Events),
			NotSynced: 0,
		}, nil
	}

	// Store new events in a single write transaction
	rwDB, ok := c.db.(kv.RwDB)
	if !ok {
		return false, fmt.Errorf("database does not support write operations")
	}

	err = rwDB.Update(ctx, func(tx kv.RwTx) error {
		for _, event := range newEvents {
			if err := application.PutEvent(tx, event); err != nil {
				return fmt.Errorf("failed to store event: %w", err)
			}
		}
		return nil
	})

	if err != nil {
		return false, fmt.Errorf("failed to sync events: %w", err)
	}

	// Return successful sync response with statistics
	return SyncResponse{
		Success: true,
		TotalFromAPI: len(apiResponse.Events),
		TotalSynced: len(newEvents),
		NotSynced: len(apiResponse.Events) - len(newEvents),
	}, nil
}
