# Bond Platform - Project Status

## What Works (2026-06-01)

All core API endpoints are functional and tested end-to-end:

### API Endpoints

| Endpoint | Status | Notes |
|----------|--------|-------|
| `GET /api/v1/factory` | ✓ | Returns factory contract with admin, instruments, templateId |
| `POST /api/v1/mint` | ✓ | Mints BOND holdings for a party |
| `GET /api/v1/holdings?party=` | ✓ | Lists holdings (SimpleHolding + LockedSimpleHolding) |
| `POST /api/v1/transfer` | ✓ | Creates transfer instruction between parties |
| `GET /api/v1/transfer-instructions?party=` | ✓ | Lists pending transfer instructions |
| `POST /api/v1/transfer/accept` | ✓ | Receiver accepts a transfer |
| `POST /api/v1/transfer/reject` | ✓ | Receiver rejects a transfer |
| `POST /api/v1/transfer/withdraw` | ✓ | Sender withdraws a pending transfer |
| `POST /api/v1/burn` | ✓ | Owner burns a holding |

### Cross-Participant Transfers

Transferring BOND tokens between parties on different participants works:
- **Alice** (participant1) → **Bob** (participant2)
- Bob is known on participant1 (`isLocal: false`) through domain topology sharing
- Accept/Reject are exercised by Bob on participant2
- Withdraw is exercised by Alice on participant1

### Verified Flows

```
Mint → Transfer → Accept          → Holdings correct
Mint → Transfer → Withdraw         → Holdings correct
Mint → Transfer → Reject           → Holdings correct
Mint → Transfer (after withdraw)   → Works with fresh holdings
Burn (owner only)                  → CONTRACT_NOT_FOUND for wrong party
```

## Key Decisions

1. **Interface templateId for interface choices**: JSON API V2 ExerciseCommand does not support `choiceInterfaceId`. Interface choices (`TransferFactory_Transfer`, `TransferInstruction_Accept`/`Reject`/`Withdraw`) use the interface's templateId as `templateId`.

2. **Full package hash in template constants**: Interface template constants use the full hex package hash (`55ba4deb...`) instead of the short dash-separated name (`splice-api-token-transfer-instruction-v1`). Regular template constants use `#package-name:Module:Entity` format.

3. **Admin in actAs for TransferFactory**: Factory contract's only signatory is admin. To see the contract through the TransferFactory interface, admin must be included in `actAs`. The choice controller (`transfer.sender`) must also be in `actAs`.

4. **WildcardFilter + Go-side filtering**: `ActiveContracts` uses a `WildcardFilter` to return all contracts, then `ExtractCreatedEvents` filters by matching the module:entity portion of the template ID (`templateIDTail` comparison).

5. **Holding CID from request**: Transfer accepts optional `holdingCids` field. If provided, only those specific holdings are used. Otherwise, unlocked holdings are auto-selected.

## Architecture

```
Frontend (nginx:3000) → Backend (Go:8080) → Canton JSON API V2
                                                ├── participant1:5013 (admin, alice, executor)
                                                ├── participant2:5023 (bob)
                                                └── participant3:5033 (charlie)
```

## Relevant Files

- `backend/internal/api/handlers.go`: HTTP handlers, choice-building, actAs logic
- `backend/internal/ledger/client.go`: JSON API client, template/choice constants, `ExerciseCommand` struct
- `backend/internal/ledger/client.go` → `templateIDMatches()`: Template ID format matching
- `bond-contract/daml/SimpleToken/Rules.daml`: `TransferFactory_Transfer`, `archiveAndSumInputs`
- `bond-contract/daml/SimpleToken/Holding.daml`: `SimpleHolding`, `LockedSimpleHolding` templates
- `bond-contract/daml/SimpleToken/TransferInstruction.daml`: `SimpleTransferInstruction` template
- `configs/participant1-bootstrap.sc`: Bootstrap script (parties, factory creation)
- `configs/participant2-bootstrap.sc`: Bootstrap script (parties)

## Constraints

- Building: Go binary built on host (`go 1.26`) and `docker cp` into container (no Go in container)
- Docker: Full stack runs via `docker compose up -d --build`
- Bootstrap: Runs on participant startup via canton console scripts
- Party topology: Shared automatically through the domain between participants
