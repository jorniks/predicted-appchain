package api

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/0xAtelerix/sdk/gosdk/rpc"
	"github.com/0xAtelerix/sdk/gosdk/txpool"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/example/application"
)

// Integration test: start RPC server, send transaction, get transaction by hash
func TestDefaultRPC_Integration_SendAndGetTransaction(t *testing.T) {
	localDB, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(t.TempDir()).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return txpool.Tables()
		}).
		Open()
	require.NoError(t, err)

	defer localDB.Close()

	txPool := txpool.NewTxPool[application.Transaction[application.Receipt], application.Receipt](
		localDB,
	)

	rpcServer := rpc.NewStandardRPCServer(nil)
	rpc.AddStandardMethods(rpcServer, nil, txPool)

	rpcAddress := "http://127.0.0.1:18545/rpc"

	errServer := make(chan error, 1)
	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		wg.Done()

		errServer <- rpcServer.StartHTTPServer(t.Context(), ":18545")
	}()

	select {
	case serverErr := <-errServer:
		if serverErr != nil {
			t.Fatalf("Failed to start HTTP server: %v", serverErr)
		}
	default:
		// continue
		wg.Wait()
		time.Sleep(100 * time.Millisecond)
	}

	txHash := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	// Send transaction via JSON-RPC (include hash)
	jsonReq := `{"jsonrpc":"2.0","method":"sendTransaction","params":[{"sender":"alice","token":"USDT","amount":"1234","hash":"` + txHash + `"}],"id":1}`
	resp, err := sendJSONRPCRequest(rpcAddress, jsonReq)
	require.NoError(t, err)
	require.Contains(t, resp, "result")

	jsonReqGet := `{"jsonrpc":"2.0","method":"getTransactionByHash","params":["` + txHash + `"],"id":2}`
	respGet, err := sendJSONRPCRequest(rpcAddress, jsonReqGet)
	require.NoError(t, err)
	require.Contains(t, respGet, "result")

	require.Contains(t, respGet, "alice")
	require.Contains(t, respGet, "USDT")
	require.Contains(t, respGet, "1234")
}

func TestDefaultRPC_MethodRegistration(t *testing.T) {
	// Create local DB for txpool
	localDB, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(t.TempDir()).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return txpool.Tables()
		}).
		Open()
	require.NoError(t, err)

	defer localDB.Close()

	// Create txpool
	txPool := txpool.NewTxPool[application.Transaction[application.Receipt], application.Receipt](
		localDB,
	)

	// Create RPC server and add standard methods
	rpcServer := rpc.NewStandardRPCServer(nil)

	// Test that AddStandardMethods doesn't panic (even with minimal setup)
	require.NotPanics(t, func() {
		rpc.AddStandardMethods(rpcServer, nil, txPool)
	})
}

// Helper: send JSON-RPC request to local server
func sendJSONRPCRequest(rpcAddress string, jsonReq string) (string, error) {
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		rpcAddress,
		bytes.NewBufferString(jsonReq),
	)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
