package application

import (
	"context"

	"github.com/rs/zerolog/log"
)

// GetDefaultGenesisAccounts retained for documentation / future use but not used by runtime.
// If you want full genesis seeding in the future, reintroduce logic here.
type GenesisAccount struct {
	Address string
	Token   string
	Balance uint64
}

func GetDefaultGenesisAccounts() []GenesisAccount {
	// Empty / placeholder. Keeping the function avoids breaking external references
	// but the runtime does not seed balances anymore.
	return []GenesisAccount{}
}

// InitializeGenesis is intentionally a no-op in this fork.
// The original template seeded token balances into an AccountsBucket.
// We disabled that behaviour because this appchain stores events only.
func InitializeGenesis(ctx context.Context, db interface{}) error {
	log.Info().Msg("Genesis seeding disabled: no account balances will be populated")
	return nil
}