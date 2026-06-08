package cantonledger

import "context"

// LedgerClient is the small surface used by the REST backend.
// Implementations may use the HTTP JSON API or the native Ledger API gRPC.
type LedgerClient interface {
	LedgerEnd(ctx context.Context) (int64, error)
	ActiveContracts(ctx context.Context, offset int64, templates ...string) (ActiveContractsResponse, error)
	Parties(ctx context.Context) ([]PartyDetail, error)
	AllocateParty(ctx context.Context, hint string) (PartyDetail, error)
	SubmitCommand(ctx context.Context, commandID string, cmd Command, actAs []string) (int64, error)
}
