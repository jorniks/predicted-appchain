package application

import "github.com/ledgerwatch/erigon-lib/kv"

const (
	EventsBucket   = "appevents"   // event:<id> -> json
)

func Tables() kv.TableCfg {
	return kv.TableCfg{
		EventsBucket:   {},
	}
}
