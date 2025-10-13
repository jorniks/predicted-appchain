package application

import (
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/ledgerwatch/erigon-lib/kv"
)

// EventTransaction stores or updates an event in the EventsBucket
type Transaction[R Receipt] struct {
	Event  Event  `json:"event"`
	TxHash string `json:"hash"`
}

func (e *Transaction[R]) Unmarshal(b []byte) error {
	return json.Unmarshal(b, e)
}

func (e Transaction[R]) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

func (e Transaction[R]) Hash() [32]byte {
	txHash := strings.TrimPrefix(e.TxHash, "0x")

	hashBytes, err := hex.DecodeString(txHash)
	if err != nil {
		panic(err)
	}

	var h [32]byte
	copy(h[:], hashBytes)

	return h
}

func (e Transaction[R]) Process(
	dbTx kv.RwTx,
) (res R, txs []apptypes.ExternalTransaction, err error) {
	// Store the event into EventsBucket
	if err := PutEvent(dbTx, &e.Event); err != nil {
		return e.failedReceipt(err), nil, nil
	}

	return e.successReceipt(), []apptypes.ExternalTransaction{}, nil
}

func (e *Transaction[R]) failedReceipt(err error) R {
	return R{
		TxnHash:      e.Hash(),
		ErrorMessage: err.Error(),
		TxStatus:     apptypes.ReceiptFailed,
	}
}

func (e *Transaction[R]) successReceipt() R {
	return R{
		TxnHash:  e.Hash(),
		TxStatus: apptypes.ReceiptConfirmed,
	}
}
