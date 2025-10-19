package main

import (
	"context"
	"log"

	"github.com/0xAtelerix/example/application"
	"github.com/0xAtelerix/example/application/api"
	"github.com/0xAtelerix/sdk/gosdk/rpc"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
)

func syncEvents() error {
	// Initialize the database
	appchainDB, err := mdbx.NewMDBX(mdbxlog.New()).
		Path("/data/appchain-db").
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return application.Tables()
		}).
		Open()
	if err != nil {
		return err
	}
	defer appchainDB.Close()

	// Create RPC server (needed for the sync implementation)
	rpcServer := rpc.NewStandardRPCServer(nil)
	customRPC := api.NewCustomRPC(rpcServer, appchainDB)

	// Call the existing syncEvents implementation
	result, err := customRPC.SyncEvents(context.Background(), nil)
	if err != nil {
		return err
	}

	if r, ok := result.(map[string]interface{}); ok {
		if r["success"].(bool) {
			if msg, ok := r["message"].(string); ok && msg != "" {
				log.Printf("Sync check complete: %s", msg)
			} else {
				log.Printf("Sync successful - Total API events: %v, Synced: %v, Already exists: %v",
					r["totalFromAPI"], r["totalSynced"], r["notSynced"])
			}
		}
	}

	return nil
}