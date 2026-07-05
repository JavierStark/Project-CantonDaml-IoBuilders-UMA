# Canton Bond Platform

A Dockerized Canton network with a bond token contract, Go backend API, web frontend, and real-time WebSocket listener.

## Architecture

```
                    ┌─────────────────────────┐
                    │   Canton Domain "da"     │
                    │  ┌─────────┐ ┌────────┐ │
                    │  │sequencer│ │mediator│ │
                    │  │  :5001  │ │ :5202  │ │
                    │  └────┬────┘ └───┬────┘ │
                    │       └────┬─────┘      │
                    │   ┌────────▼──────┐     │
                    │   │  synchronizer │     │
                    │   └────────┬──────┘     │
                    └────────────┼────────────┘
                                 │
        ┌────────────────────────┼────────────────────────────┐
        │                        │                            │
┌───────▼──────┐         ┌──────▼──────┐          ┌──────────▼──────┐
│ participant1 │         │ participant2│          │  participant3   │
│  admin,      │         │  bob        │          │  charlie        │
│  alice,      │         │  gRPC: 5021 │          │  gRPC: 5031     │
│  executor    │         │  HTTP: 5023 │          │  HTTP: 5033     │
│  gRPC: 5011  │         └──────┬──────┘          └──────┬──────────┘
│  HTTP: 5013  │                │                        │
└──────┬───────┘                │                        │
       │          ┌─────────────┼────────────────────────┘
       │          │             │
       │   ┌──────▼─────────────▼──────┐
       │   │    Go Backend :8080       │──► REST API /api/v1
       │   │  (gRPC + JSON API fallback)│
       │   └─────────────┬─────────────┘
       │                 │
       │   ┌─────────────▼─────────────┐
       │   │  Listener :8081           │──► WebSocket /ws/bonds
       │   │  (gRPC stream → WS bridge)│
       │   └───────────────────────────┘
       │
       └─── (gRPC:5011, HTTP:5013)
```

## Prerequisites

- [Docker](https://docs.docker.com/engine/install/) + Docker Compose
- At least 6 GB RAM available for Docker
- Pre-pulled Canton images: `europe-docker.pkg.dev/da-images/public/docker/canton-{base,sequencer,mediator,participant}:3.4.11`

## Quick Start

```bash
./start.sh
```

This single script:

1. Checks that Docker and Docker Compose are installed
2. Verifies the bond contract DAR exists (builds it with `dpm`/`daml` if missing)
3. Builds all Docker images (backend, listener, frontend)
4. Starts all 9 services: sequencer, mediator, synchronizer, 3 participants, backend, listener, frontend, OTEL collector
5. Waits for all containers to be up and healthy
6. Initializes the factory contract
7. Verifies key API endpoints with a mint + holdings check

Once complete, open http://localhost:3000 in your browser.

### Manual setup (alternative)

If you prefer to build and start manually:

```bash
# Build DAR
cd bond-contract && dpm build && cp .daml/dist/simple-token-0.1.0.dar ../dars/ && cd ..

# Start all services
docker compose up -d --build

# Wait for participants to be healthy, then initialize factory
curl -s -X POST http://localhost:8080/api/v1/factory | jq .
```

### Memory requirements

Three Canton participants each run a JVM. On machines with limited RAM, the containers may be killed (exit code 137 = OOM). The `docker-compose.yml` already includes:

```yaml
participantN:
  environment:
    - JAVA_TOOL_OPTIONS=-Xmx768m
  mem_limit: 1536m
```

If participants still crash, try reducing to `-Xmx512m` / `mem_limit: 1024m` or increase Docker's memory allocation in Docker Desktop settings.

## Project Structure

```
├── README.md
├── start.sh                       # One-command startup script
├── docker-compose.yml             # 9 services: sequencer, mediator, sync, 3 participants,
│                                  #   backend, listener, frontend, otel-collector
├── bond-contract/                 # Bond token DAML contract
│   ├── daml.yaml
│   └── daml/SimpleToken/
│       ├── Holding.daml           # SimpleHolding + LockedSimpleHolding
│       ├── Rules.daml             # SimpleTokenRules factory (Mint, Transfer, Allocation)
│       ├── TransferInstruction.daml
│       ├── Allocation.daml
│       └── ContextUtils.daml
├── dars/                          # Pre-built DAR files
│   ├── splice-api-token-*.dar     # CIP-056 interface DARs (5 files)
│   └── simple-token-0.1.0.dar    # Built bond contract DAR
├── configs/                       # Canton bootstrap and collector configuration
│   ├── shared-bootstrap.sc        # Shared init helpers (keys, topology, connectToSynchronizer)
│   ├── sequencer-bootstrap.sc
│   ├── mediator-bootstrap.sc
│   ├── synchronizer-bootstrap.sc
│   ├── synchronizer-remote.conf
│   ├── participant1-bootstrap.sc  # Init + DAR upload + parties (admin, alice, executor)
│   ├── participant2-bootstrap.sc  # Init + DAR upload + parties (bob)
│   ├── participant3-bootstrap.sc  # Init + DAR upload + parties (charlie)
│   └── otel-collector.yaml        # OpenTelemetry collector config
├── backend/                       # Go REST API server
│   ├── Dockerfile
│   ├── go.mod / go.sum
│   ├── main.go                    # Entrypoint + OTel init
│   ├── api.go                     # All HTTP handlers + routing
│   └── config.go                  # Environment-based config
├── pkg/cantonledger/              # Shared Go package: Canton Ledger API client
│   ├── interface.go               # LedgerClient interface
│   ├── client.go                  # HTTP JSON API v2 client
│   ├── grpc_client.go             # gRPC client (StateService reads + HTTP writes)
│   ├── commands.go                # Command building + submit-and-wait
│   ├── events.go                  # CreatedEvent parsing
│   ├── templates.go               # Template/choice ID constants
│   ├── parties.go                 # Party operations
│   ├── daml.go                    # Daml decimal/instrument helpers
│   ├── stream.go                  # gRPC update stream (listener)
│   └── proto/                     # Generated protobuf code
├── listener/                      # Real-time event listener
│   ├── Dockerfile
│   └── main.go                    # gRPC stream → WebSocket bridge
├── frontend/                      # Vite SPA web frontend
│   ├── Dockerfile
│   ├── package.json
│   ├── vite.config.js             # Dev server + proxy to backend
│   ├── index.html
│   ├── app.js                     # API client + UI logic + WebSocket
│   └── style.css
├── e2e/                           # Playwright end-to-end tests
├── docs/                          # Extended documentation
│   └── backend-api.md             # Per-endpoint API reference
├── Canton.Aspire.Hosting/         # .NET Aspire hosting integration (alternative launcher)
└── migracion_gRPC.md              # gRPC migration guide
```

## Party Distribution

| Participant | Parties | JSON API | gRPC |
|---|---|---|---|
| participant1 | admin, alice, executor | http://localhost:5013 | localhost:5011 |
| participant2 | bob | http://localhost:5023 | localhost:5021 |
| participant3 | charlie | http://localhost:5033 | localhost:5031 |

## API Endpoints

The Go backend exposes a REST API at `http://localhost:8080/api/v1/`.

**Transport**: The backend uses gRPC for reads (`GetLedgerEnd`, `GetActiveContracts`) and HTTP JSON API v2 for writes (`submit-and-wait`) and party management. Set `LEDGER_TRANSPORT=http` to fall back to full HTTP mode.

| Method | Path | Description |
|---|---|---|
| GET | /api/v1/health | Health check |
| GET | /api/v1/parties | List all parties across participants |
| POST | /api/v1/parties | Allocate a new party |
| GET | /api/v1/holdings?party=X | List holdings for a party |
| POST | /api/v1/mint | Mint a new bond |
| POST | /api/v1/transfer | Initiate a two-step transfer |
| POST | /api/v1/transfer/accept | Accept a transfer instruction |
| POST | /api/v1/transfer/reject | Reject a transfer instruction |
| POST | /api/v1/transfer/withdraw | Withdraw a transfer instruction |
| POST | /api/v1/self-transfer | Merge holdings (sender == receiver) |
| POST | /api/v1/burn | Burn a holding |
| GET | /api/v1/transfer-instructions?party=X | List pending transfers |
| GET | /api/v1/allocations?party=X | List allocations for a party |
| POST | /api/v1/allocations | Create a new DvP allocation |
| POST | /api/v1/allocations/execute | Execute an allocation (by executor) |
| POST | /api/v1/allocations/cancel | Cancel an allocation |
| POST | /api/v1/allocations/withdraw | Withdraw an allocation before deadline |
| GET/POST | /api/v1/factory | Get or create the SimpleTokenRules factory |

## API Examples

### Mint a bond

```bash
curl -X POST http://localhost:8080/api/v1/mint \
  -H "Content-Type: application/json" \
  -d '{
    "admin": "admin",
    "owner": "alice",
    "amount": 1000.00,
    "couponRate": 5.0,
    "maturityDate": "2028-12-31",
    "description": "Corporate Bond A"
  }'
```

### List holdings

```bash
curl "http://localhost:8080/api/v1/holdings?party=alice"
```

### Transfer a bond (two-step)

```bash
curl -X POST http://localhost:8080/api/v1/transfer \
  -H "Content-Type: application/json" \
  -d '{"sender": "alice", "receiver": "bob", "amount": 500.00}'
```

### Accept a transfer

```bash
curl -X POST http://localhost:8080/api/v1/transfer/accept \
  -H "Content-Type: application/json" \
  -d '{"party": "bob", "contractId": "<contract-id>"}'
```

### Create a DvP allocation

```bash
curl -X POST http://localhost:8080/api/v1/allocations \
  -H "Content-Type: application/json" \
  -d '{
    "sender": "alice",
    "receiver": "bob",
    "executor": "executor",
    "amount": 100,
    "allocateBefore": "2028-12-31T00:00:00Z",
    "settleBefore": "2029-01-02T00:00:00Z",
    "settlementRef": "dvp-2028-0001",
    "transferLegId": "leg-1"
  }'
```

### Execute an allocation (DvP settlement)

```bash
curl -X POST http://localhost:8080/api/v1/allocations/execute \
  -H "Content-Type: application/json" \
  -d '{"party": "executor", "contractId": "<allocation-contract-id>"}'
```

## Frontend

Open http://localhost:3000 in your browser.

The frontend is a Vite SPA running in Docker, proxying `/api` requests to the backend at `http://backend:8080`.

**Pages:**
- **Dashboard** — overview of all bonds, parties, pending transfers, and allocations
- **Mint** — create new bonds
- **Holdings** — view and filter bond holdings by party
- **Transfer** — initiate two-step transfers between parties
- **Pending** — accept, reject, or withdraw pending transfer instructions
- **Burn** — burn bonds (by owner or admin)
- **Parties** — view and create parties
- **Allocations** — create, list, execute, cancel, and withdraw DvP allocations

## Ledger Listener & WebSocket

The listener subscribes to Canton's gRPC `UpdateService.GetUpdates` stream and broadcasts `created`/`archived` events to the frontend via WebSocket at `ws://localhost:8081/ws/bonds`. The frontend uses these events to auto-refresh views in real time.

```bash
# Watch real-time events
docker logs -f bond-listener
```

The listener connects only to participant1 (the primary node). Events originating on participant2/3 that are visible through domain topology sharing on participant1 are also streamed.

## Bond Contract

The bond token contract implements CIP-056 token interfaces with:

| Template | Purpose |
|---|---|
| **SimpleTokenRules** | Factory contract. Signatory: admin. Observers: all parties. Mint, transfer, and allocation entry point. |
| **SimpleHolding** | Unlocked holding of a bond/cash instrument. Signatory: admin, owner. |
| **LockedSimpleHolding** | Escrowed holding during transfer/allocation. Signatory: admin, owner. Lock expires at deadline. |
| **SimpleTransferInstruction** | Pending two-step transfer (accept/reject/withdraw). |
| **SimpleAllocation** | Funded DvP allocation. Settled by executor via `ExecuteTransfer`. |

### DvP Workflow

The delivery-vs-payment settlement uses the `SimpleAllocation` template with a neutral executor who controls the final delivery. Below is the complete sequence:

```
SENDER (alice)            FACTORY (SimpleTokenRules)       RECEIVER (bob)        EXECUTOR
    │                            │                             │                    │
    │  1. POST /allocations      │                             │                    │
    │  ─────────────────────────▶│                             │                    │
    │  {sender, receiver,        │                             │                    │
    │   executor, amount,        │                             │                    │
    │   allocateBefore,          │                             │                    │
    │   settleBefore,            │                             │                    │
    │   settlementRef}           │                             │                    │
    │                            │                             │                    │
    │                    Choice: AllocationFactory_Allocate    │                    │
    │                            │                             │                    │
    │                    ┌───────┴───────┐                     │                    │
    │                    │ archive input │                     │                    │
    │                    │   holdings    │                     │                    │
    │                    └───────┬───────┘                     │                    │
    │                            │                             │                    │
    │              ┌─────────────┼─────────────┐               │                    │
    │              ▼             ▼             ▼               │                    │
    │      ┌──────────────┐ ┌───────────┐ ┌─────────┐          │                    │
    │      │ LockedSimple │ │SimpleAlloc│ │ change  │          │                    │
    │      │  Holding     │ │  ation    │ │ holding │          │                    │
    │      │ (escrow)     │ │  (active) │ │ (if any)│          │                    │
    │      │ owner:alice  │ │           │ │→ sender │          │                    │
    │      │ lock:admin   │ │ executor  │ └─────────┘          │                    │
    │      │ expires:     │ │ receiver  │                      │                    │
    │      │ settleBefore │ │ deadlines │                      │                    │
    │      └──────┬───────┘ └─────┬─────┘                      │                    │
    │             │ references    │                            │                    │
    │             └───────────────┘                            │                    │
    │                                                          │                    │
    │  [assets locked in escrow. Sender can Withdraw           │                    │
    │   before allocateBefore. Executor can Cancel.]           │                    │
    │                                                          │                    │
    │                                           2. POST /allocations/execute        │
    │                                           ┌───────────────────────────────────▶│
    │                                           │            {party:executor,        │
    │                                           │             contractId}            │
    │                                           │                                    │
    │                                           │          Choice:                   │
    │                                           │    Allocation_ExecuteTransfer      │
    │                                           │                                    │
    │                                           │      ┌─────────────────┐           │
    │                                           │      │ archive Locked  │           │
    │                                           │      │ SimpleHolding   │           │
    │                                           │      └────────┬────────┘           │
    │                                           │               │                    │
    │                                           │      ┌────────▼────────┐           │
    │                                           │      │ SimpleHolding   │           │
    │                        ┌──────────────────┼──────│ owner: bob      │           │
    │                        │                  │      │ amount: 100     │──────▶ bob owns
    │                        │ (change if any)  │      └─────────────────┘       the assets
    │                        ▼                  │
    │              ┌──────────────┐             │
    │              │ SimpleHolding│             │
    │              │ owner: alice │             │
    │              └──────────────┘             │
    │                                           │
    │  [SETTLEMENT COMPLETE]                    │
    │                                           │
    │                                           │
    │        ─ ─ ─ ─ Alternative paths ─ ─ ─ ─ ─
    │
    │  Sender Withdraw (before allocateBefore):
    │    POST /allocations/withdraw
    │    → archives LockedSimpleHolding
    │    → returns SimpleHolding to sender
    │
    │  Executor Cancel (any time):
    │    POST /allocations/cancel
    │    → archives LockedSimpleHolding
    │    → returns SimpleHolding to sender
    │
```

**Step-by-step:**

1. **Allocate** — sender locks assets via `AllocationFactory_Allocate` on the factory. Input `SimpleHolding` contracts are archived. A `LockedSimpleHolding` (escrow) and `SimpleAllocation` (funded, ready for settlement) are created. Excess is returned as change.

2. **Execute** — the neutral executor calls `Allocation_ExecuteTransfer`. The locked holding is archived and a new `SimpleHolding` is created with `owner = receiver`. Settlement is final and atomic.

3. **Cancel** — executor cancels the allocation at any time. Locked funds are returned to sender.

4. **Withdraw** — sender withdraws their own allocation, but only before the `allocateBefore` deadline.

## Transport Modes

The backend supports two ledger transport modes, configured via `LEDGER_TRANSPORT`:

| Mode | Reads | Writes / Party management |
|---|---|---|
| `grpc` (default) | gRPC `StateService` | HTTP JSON API v2 |
| `http` | HTTP JSON API v2 | HTTP JSON API v2 |

To switch to HTTP-only mode:

```bash
LEDGER_TRANSPORT=http docker compose up -d --build backend
```

## Stopping

```bash
docker compose down
```
