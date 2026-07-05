# Bond Platform — Project Status

## What Works (2026-07)

All core API endpoints are functional and tested end-to-end with gRPC transport.

### API Endpoints

| Endpoint | Status | Notes |
|----------|--------|-------|
| `GET/POST /api/v1/factory` | OK | Returns/creates factory contract with admin, instruments, templateId |
| `POST /api/v1/mint` | OK | Mints BOND holdings for a party |
| `GET /api/v1/holdings?party=` | OK | Lists holdings (SimpleHolding + LockedSimpleHolding) |
| `POST /api/v1/transfer` | OK | Creates transfer instruction between parties |
| `GET /api/v1/transfer-instructions?party=` | OK | Lists pending transfer instructions |
| `POST /api/v1/transfer/accept` | OK | Receiver accepts a transfer |
| `POST /api/v1/transfer/reject` | OK | Receiver rejects a transfer |
| `POST /api/v1/transfer/withdraw` | OK | Sender withdraws a pending transfer |
| `POST /api/v1/self-transfer` | OK | Merge holdings (sender == receiver) |
| `POST /api/v1/burn` | OK | Owner or admin burns a holding |
| `GET /api/v1/allocations?party=` | OK | Lists DvP allocations |
| `POST /api/v1/allocations` | OK | Creates funded DvP allocation |
| `POST /api/v1/allocations/execute` | OK | Executor settles allocation, delivers assets |
| `POST /api/v1/allocations/cancel` | OK | Cancel allocation, return funds to sender |
| `POST /api/v1/allocations/withdraw` | OK | Withdraw allocation before allocateBefore |
| `GET /api/v1/parties` | OK | Lists all local parties across participants |
| `POST /api/v1/parties` | OK | Allocates a new party on a participant |

### Cross-Participant Operations

- **Alice** (participant1) to **Bob** (participant2) transfers work via domain topology sharing
- Factory contract on participant1 is visible on participant2/3 via `observers` field
- **DvP allocations** work with executor (participant1) settling deliveries to receivers on any participant
- Allocator and receiver can be on different participants; executor co-located with factory (participant1)

### Real-Time Events

- **Listener** subscribes to Canton gRPC `UpdateService.GetUpdates` on participant1
- Events broadcast via **WebSocket** at `ws://localhost:8081/ws/bonds`
- Frontend auto-refreshes dashboard, holdings, pending, and allocations on received events

### Verified Flows

```
Mint → Transfer → Accept                    → Holdings correct
Mint → Transfer → Withdraw                  → Holdings correct
Mint → Transfer → Reject                    → Holdings correct
Mint → Allocate → Execute (DvP)             → Holdings correct
Mint → Allocate → Cancel                    → Holdings correct
Mint → Allocate → Withdraw                  → Holdings correct
Mint → Self-Transfer (merge)                → Holdings correct
Burn (owner) / BurnByAdmin                  → CONTRACT_NOT_FOUND for wrong party
```

## Architecture

```
start.sh
  └── docker compose up -d
        ├── sequencer1 (Canton 3.4.11)
        ├── mediator1  (Canton 3.4.11)
        ├── synchronizer (Canton 3.4.11)
        ├── participant1 (admin, alice, executor) — gRPC:5011, HTTP:5013
        ├── participant2 (bob)                    — gRPC:5021, HTTP:5023
        ├── participant3 (charlie)                — gRPC:5031, HTTP:5033
        ├── bond-backend  (Go, :8080) — REST API, gRPC reads + HTTP writes
        ├── bond-listener (Go, :8081) — gRPC stream → WebSocket bridge
        ├── bond-frontend (Vite, :3000) — SPA, proxies /api to backend
        └── otel-collector (OTEL, :4317)

Backend transport: gRPC (default) for reads, HTTP JSON API v2 for writes.
Set LEDGER_TRANSPORT=http for full HTTP fallback.
```

## Key Decisions

1. **Hybrid gRPC + HTTP**: Backend uses gRPC `StateService` for `LedgerEnd`/`ActiveContracts` and HTTP JSON API v2 for `submit-and-wait` and party management. This avoids the complexity of building Daml values in protobuf for writes while keeping reads fast.

2. **Interface template IDs**: JSON API V2 `ExerciseCommand` uses interface template IDs for interface choices (`TransferFactory_Transfer`, `AllocationFactory_Allocate`, etc.). Package-hash-prefixed format for splice interface DARs, short format for simple-token DAR.

3. **WildcardFilter + Go-side filtering**: `ActiveContracts` uses a `WildcardFilter` to return all contracts. `ExtractCreatedEvents` filters by matching the module:entity portion of the template ID.

4. **Holding CID auto-selection**: Transfers accept optional `holdingCids`. If omitted, unlocked holdings are auto-selected to cover the amount.

5. **In-memory storage**: All Canton nodes use `storage.type = memory`. State is ephemeral and lost on restart. Suitable for development and testing only.

## Relevant Files

- `backend/api.go` — HTTP handlers, choice-building, actAs logic
- `backend/config.go` — Environment-based configuration, participant routing
- `pkg/cantonledger/grpc_client.go` — gRPC client (StateService reads + HTTP fallback writes)
- `pkg/cantonledger/client.go` — Full HTTP JSON API v2 client
- `pkg/cantonledger/commands.go` — Command building + submit-and-wait
- `pkg/cantonledger/templates.go` — Template/choice ID constants
- `listener/main.go` — gRPC update stream → WebSocket broadcaster
- `frontend/app.js` — SPA logic, REST calls, WebSocket consumer
- `bond-contract/daml/SimpleToken/Rules.daml` — Factory, Mint, Transfer, Allocation
- `bond-contract/daml/SimpleToken/Holding.daml` — SimpleHolding, LockedSimpleHolding
- `bond-contract/daml/SimpleToken/TransferInstruction.daml` — Pending transfer with accept/reject/withdraw
- `bond-contract/daml/SimpleToken/Allocation.daml` — DvP allocation with execute/cancel/withdraw
- `configs/participant1-bootstrap.sc` — Bootstrap: keys, DAR upload, parties, synchronizer connect
- `configs/shared-bootstrap.sc` — Shared helpers: topology init, key generation, synchronizer connection

## Constraints

- **Memory**: 3 Canton participant JVMs. `docker-compose.yml` sets `JAVA_TOOL_OPTIONS=-Xmx768m` and `mem_limit: 1536m` per participant.
- **Docker**: Full stack via `docker compose up -d` or `./start.sh`.
- **Bootstrap**: Canton console scripts run on participant startup. Synchronizer bootstraps first, then each participant uploads DARs, creates parties, and connects to the domain.
- **Party topology**: Shared automatically through the domain between participants.
- **gRPC insecure**: gRPC connections use `insecure.NewCredentials()` — safe for local Docker networks, not for production.
