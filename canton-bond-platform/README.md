# Canton Bond Platform

A Dockerized Canton network with a bond token contract, Go backend API, and web frontend.

## Architecture

```
                    ┌──────────────────┐
                    │   synchronizer   │
                    └────────┬─────────┘
                             │
            ┌────────────────┼──────────────────────────────────┐
            │                │                                  │
    ┌───────▼──────┐  ┌─────▼──────┐  ┌──────▼──────────┐
    │  sequencer1   │  │ mediator1  │  │  participant1   │  admin, alice, executor
    │  public:5001  │  │ admin:5202 │  │  http-json:5013 │
    └───────────────┘  └────────────┘  └─────────────────┘
                                       ┌─────────────────┐
                                       │  participant2   │  bob
                                       │  http-json:5023 │
                                       └─────────────────┘
                                       ┌─────────────────┐
                                       │  participant3   │  charlie
                                       │  http-json:5033 │
                                       └─────────────────┘
                                              │
                                    ┌─────────▼─────────┐
                                    │   Go Backend :8080 │
                                    └─────────┬─────────┘
                                              │
                                    ┌─────────▼─────────┐
                                    │  Frontend :3000   │
                                    └───────────────────┘
```

## Prerequisites

- [Docker](https://docs.docker.com/engine/install/) + [Docker Compose](https://docs.docker.com/compose/install/)
- Pre-pulled Canton images: `europe-docker.pkg.dev/da-images/public/docker/canton-{base,sequencer,mediator,participant}:3.4.8`
- [Daml SDK](https://docs.daml.com/) or `dpm` (to build the bond DAR)

## Quick Start

### 1. Build the bond contract DAR

```bash
cd bond-contract && dpm build
cp .daml/dist/simple-token-0.1.0.dar ../dars/
cd ..
```

### 2. Start everything

```bash
docker compose up -d
```

Wait ~60 seconds for bootstrap completion, then verify:

```bash
docker ps --format "table {{.Names}}\t{{.Status}}"
```

Expected output:
```
sequencer1         Up X minutes
mediator1          Up X minutes
synchronizer       Up X minutes
participant1       Up X minutes (healthy)
participant2       Up X minutes (healthy)
participant3       Up X minutes (healthy)
bond-backend       Up X minutes
bond-frontend      Up X minutes
```

### 3. Initialize the factory contract

The factory contract (SimpleTokenRules) is auto-created when first needed. Trigger it:

```bash
curl -s http://localhost:8080/api/v1/factory | jq .
```

Or open http://localhost:3000 in your browser and click around — the frontend will initialize everything.

## Project Structure

```
├── README.md
├── docker-compose.yml            # 8 services: sequencer, mediator, sync, 3 participants, backend, frontend
├── bond-contract/                # Bond token DAML contract (copy from canton-token-template)
│   ├── daml.yaml
│   └── daml/SimpleToken/
│       ├── Holding.daml          # SimpleHolding + LockedSimpleHolding
│       ├── Rules.daml            # SimpleTokenRules factory (Mint, TransferFactory, AllocationFactory)
│       ├── TransferInstruction.daml
│       ├── Allocation.daml
│       └── ContextUtils.daml
├── dars/                         # Pre-built DAR files
│   ├── splice-api-token-*.dar    # CIP-056 interface DARs (5 files)
│   └── simple-token-0.1.0.dar   # Built bond contract DAR
├── configs/                      # Canton bootstrap configuration
│   ├── shared-bootstrap.sc       # Shared init + DAR upload helpers
│   ├── sequencer-bootstrap.sc
│   ├── mediator-bootstrap.sc
│   ├── synchronizer-bootstrap.sc
│   ├── synchronizer-remote.conf
│   ├── participant1-bootstrap.sc # Init + DAR upload + parties (admin, alice, executor)
│   ├── participant2-bootstrap.sc # Init + DAR upload + parties (bob)
│   └── participant3-bootstrap.sc # Init + DAR upload + parties (charlie)
├── backend/                      # Go REST API server
│   ├── Dockerfile
│   ├── go.mod
│   ├── cmd/server/main.go
│   └── internal/
│       ├── config/config.go      # Environment-based configuration
│       ├── ledger/client.go      # Canton JSON Ledger API v2 client
│       └── api/
│           ├── server.go         # Router, middleware, CORS
│           └── handlers.go       # All API handlers
└── frontend/                     # Static web frontend
    ├── Dockerfile                # nginx
    ├── nginx.conf                # Reverse proxy /api/ -> backend
    └── html/
        ├── index.html            # SPA with all pages
        ├── app.js                # API client + UI logic
        └── style.css
```

## Party Distribution

| Participant | Parties | JSON API |
|---|---|---|
| participant1 | admin, alice, executor | http://localhost:5013 |
| participant2 | bob | http://localhost:5023 |
| participant3 | charlie | http://localhost:5033 |

## API Endpoints

The Go backend exposes a REST API at `http://localhost:8080/api/v1/`.

| Method | Path | Description |
|---|---|---|
| GET | /health | Health check |
| GET | /parties | List all parties across participants |
| POST | /parties | Allocate a new party |
| GET | /holdings?party=X | List holdings for a party |
| POST | /mint | Mint a new bond |
| POST | /transfer | Initiate a two-step transfer |
| POST | /transfer/accept | Accept a transfer instruction |
| POST | /transfer/reject | Reject a transfer instruction |
| POST | /transfer/withdraw | Withdraw a transfer instruction |
| POST | /self-transfer | Merge holdings (sender == receiver) |
| POST | /burn | Burn a holding |
| GET | /transfer-instructions?party=X | List pending transfers |
| GET | /factory | Get or create the SimpleTokenRules factory |

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
curl http://localhost:8080/api/v1/holdings?party=alice
```

### Transfer a bond

```bash
curl -X POST http://localhost:8080/api/v1/transfer \
  -H "Content-Type: application/json" \
  -d '{
    "sender": "alice",
    "receiver": "bob",
    "amount": 500.00
  }'
```

### Accept a transfer

```bash
curl -X POST http://localhost:8080/api/v1/transfer/accept \
  -H "Content-Type: application/json" \
  -d '{
    "party": "bob",
    "contractId": "<contract-id>"
  }'
```

### Burn a bond

```bash
curl -X POST http://localhost:8080/api/v1/burn \
  -H "Content-Type: application/json" \
  -d '{
    "party": "alice",
    "contractId": "<contract-id>"
  }'
```

## Frontend

Open http://localhost:3000 in your browser.

The frontend provides:
- **Dashboard** — overview of all bonds and parties
- **Mint** — create new bonds
- **Holdings** — view and filter bond holdings
- **Transfer** — initiate two-step transfers
- **Pending** — accept, reject, or withdraw pending transfers
- **Burn** — burn bonds (owner or admin)
- **Parties** — view and create parties

## Bond Contract

The bond token contract implements the CIP-056 token standard with:

- **SimpleTokenRules** — Factory contract for minting bonds, managing transfers, and allocations
- **SimpleHolding** — A bond holding with amount, coupon rate, maturity date, and description
- **LockedSimpleHolding** — Locked holding during two-step transfer
- **SimpleTransferInstruction** — Pending transfer (accept/reject/withdraw)
- **SimpleAllocation** — DvP settlement

## Stopping

```bash
docker compose down
```
