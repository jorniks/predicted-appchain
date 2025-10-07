package api

import (
	"context"
	"encoding/json"
	"fmt"

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

	ev, err := application.GetEvent(ctx, tx, req.EventID)
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
