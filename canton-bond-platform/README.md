# Canton Bond Platform

A Canton network with a bond token contract, Go backend API, web frontend, and Aspire Dashboard observability.

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
- [Aspire CLI](https://aspire.dev/) 13.4 or newer
- Pre-pulled Canton images: `europe-docker.pkg.dev/da-images/public/docker/canton-{base,sequencer,mediator,participant}:3.4.11`
- [Daml SDK](https://docs.daml.com/) or `dpm` (to build the bond DAR)

## Quick Start

### 1. Build the bond contract DAR

```bash
cd bond-contract && dpm build
cp .daml/dist/simple-token-0.1.0.dar ../dars/
cd ..
```

### 2. Start everything with Aspire

```bash
./start-aspire.sh
```

The script starts the AppHost in the foreground and Aspire prints the Dashboard
URL plus the OTLP/gRPC endpoint it assigned for that run.

Wait for the Canton participants to finish bootstrap, then verify from the Aspire Dashboard
Resources view. If you use the Docker Compose fallback instead, run:

```bash
docker compose up -d
```

The Compose fallback is only for running the application stack without Aspire.
It does not start Grafana, Prometheus, Tempo, or the Aspire Dashboard.

Do not call the ledger-backed API endpoints while the participants still show
`health: starting`. On a clean in-memory startup, Canton may need 1-2 minutes to
initialize identities, connect the synchronizer, expose the Ledger API gRPC
ports, and create the parties.

### 3. Use the Aspire endpoints

Aspire publishes backend and frontend endpoints dynamically. Open the Aspire
Dashboard URL printed by `./start-aspire.sh`, go to `Resources`, and copy the
endpoint for `frontend` or `backend`.

For shell checks, set the backend endpoint once:

```bash
export BACKEND_URL=http://127.0.0.1:<backend-port>
curl -s "$BACKEND_URL/api/v1/health" | jq .
```

The factory contract (`SimpleTokenRules`) is auto-created by the `factory-init`
resource during startup. You can also query it explicitly:

```bash
curl -s "$BACKEND_URL/api/v1/factory" | jq .
```

Or open the frontend endpoint from Aspire and click around. The frontend uses
the backend endpoint injected by Aspire.

### 4. Verify gRPC transport

The public REST API stays the same, but the backend uses the native Canton
Ledger API gRPC by default.

Check the `backend` logs from the Aspire Dashboard, or from the terminal where
`./start-aspire.sh` is running. Expected lines:

```text
ledger transport: grpc
participant participant1 -> localhost:5011 (grpc), fallback http http://localhost:5013
participant participant2 -> localhost:5021 (grpc), fallback http http://localhost:5023
participant participant3 -> localhost:5031 (grpc), fallback http http://localhost:5033
```

Check that the listener has resolved the `admin` party and opened the gRPC
stream by opening the `listener` logs in Aspire.

Expected lines:

```text
¡ÉXITO! ID resuelto: admin -> admin::...
Bond Listener Iniciado con gRPC
Stream gRPC abierto. Esperando eventos para enviar a los WebSockets...
```

Then verify the REST endpoints:

```bash
curl -s "$BACKEND_URL/api/v1/health"
curl -s "$BACKEND_URL/api/v1/parties"
curl -s "$BACKEND_URL/api/v1/factory"
```

If `/api/v1/parties` returns `[]` or `/api/v1/factory` returns
`failed to query ledger end`, wait until all participants are healthy and the
listener has resolved `admin`, then retry. See
[`migracion_gRPC.md`](./migracion_gRPC.md) for the full gRPC verification flow,
fallback mode, and troubleshooting.

## Observability: Aspire Dashboard

Aspire is the local observability entry point:

- Canton nodes export OTLP traces to the local `otel-collector`.
- Canton nodes expose internal Prometheus metrics on `:10013`; the `otel-collector` scrapes those metrics and exports them to Aspire as OTLP metrics.
- The Go backend exports OTLP traces and HTTP metrics to the same local collector.
- Aspire shows resource state, logs, traces, trace details, backend metrics, and Canton internal metrics without running Grafana, Tempo, or Prometheus.

Start or refresh the Aspire AppHost:

```bash
./start-aspire.sh
```

Then open the Dashboard URL printed by Aspire. Keep the terminal running; stop
the AppHost with `Ctrl+C`.

Generate backend and Canton activity:

```bash
export BACKEND_URL=http://127.0.0.1:<backend-port>

for i in $(seq 1 20); do
  curl -s "$BACKEND_URL/api/v1/health" >/dev/null
  curl -s "$BACKEND_URL/api/v1/factory" >/dev/null
done
```

Expected result: Aspire shows `backend` spans and Canton spans from
`sequencer1`, `mediator1`, `synchronizer`, and the participants.

In the `Metrics` view, filter by `canton` or by a Canton resource and look for
internal Canton metrics such as:

- `daml_cache_hits`
- `daml_cache_size`
- `daml_db_commit_duration_seconds`
- `daml_db_storage_general_executor_load`
- `daml_grpc_server_duration_seconds`

For backend metrics, filter by `backend` and look for:

- `http.server.request.count`
- `http.server.request.duration`

If traces are missing, check the Canton resource environment in Aspire. It
should contain `canton.monitoring.tracing.tracer.exporter.address =
 "otel-collector"` and `canton.monitoring.tracing.tracer.exporter.port =
4317`.
## Project Structure

```
├── README.md
├── docker-compose.yml            # Docker Compose fallback without Aspire observability
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
├── configs/                      # Canton bootstrap and collector configuration
│   ├── shared-bootstrap.sc       # Shared init + DAR upload helpers
│   ├── sequencer-bootstrap.sc
│   ├── mediator-bootstrap.sc
│   ├── synchronizer-bootstrap.sc
│   ├── synchronizer-remote.conf
│   ├── participant1-bootstrap.sc # Init + DAR upload + parties (admin, alice, executor)
│   ├── participant2-bootstrap.sc # Init + DAR upload + parties (bob)
│   ├── participant3-bootstrap.sc # Init + DAR upload + parties (charlie)
│   └── otel-collector.yaml       # OpenTelemetry collector config
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
└── frontend/                     # Static web frontend (Vite SPA, served via Aspire YARP proxy)
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

The Go backend exposes a REST API at `/api/v1`. When running with Aspire, copy
the `backend` endpoint from the Dashboard `Resources` view and use it as
`BACKEND_URL`. In Docker Compose fallback mode, the default is
`http://localhost:8080/api/v1/`.

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
| GET | /allocations?party=X | List allocations for a party |
| POST | /allocations | Create a new allocation |
| POST | /allocations/execute | Execute an allocation |
| POST | /allocations/cancel | Cancel an allocation |
| POST | /allocations/withdraw | Withdraw an allocation |
| GET | /factory | Get or create the SimpleTokenRules factory |

## API Examples

### Mint a bond

```bash
curl -X POST "$BACKEND_URL/api/v1/mint" \
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
curl "$BACKEND_URL/api/v1/holdings?party=alice"
```

### Transfer a bond

```bash
curl -X POST "$BACKEND_URL/api/v1/transfer" \
  -H "Content-Type: application/json" \
  -d '{
    "sender": "alice",
    "receiver": "bob",
    "amount": 500.00
  }'
```

### Accept a transfer

```bash
curl -X POST "$BACKEND_URL/api/v1/transfer/accept" \
  -H "Content-Type: application/json" \
  -d '{
    "party": "bob",
    "contractId": "<contract-id>"
  }'
```

### Burn a bond

```bash
curl -X POST "$BACKEND_URL/api/v1/burn" \
  -H "Content-Type: application/json" \
  -d '{
    "party": "alice",
    "contractId": "<contract-id>"
  }'
```

### Create an allocation

```bash
curl -X POST "$BACKEND_URL/api/v1/allocations" \
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

### Execute an allocation

```bash
curl -X POST "$BACKEND_URL/api/v1/allocations/execute" \
  -H "Content-Type: application/json" \
  -d '{
    "party": "executor",
    "contractId": "<allocation-contract-id>"
  }'
```

### Cancel an allocation

```bash
curl -X POST "$BACKEND_URL/api/v1/allocations/cancel" \
  -H "Content-Type: application/json" \
  -d '{
    "party": "executor",
    "contractId": "<allocation-contract-id>"
  }'
```

### Withdraw an allocation

```bash
curl -X POST "$BACKEND_URL/api/v1/allocations/withdraw" \
  -H "Content-Type: application/json" \
  -d '{
    "party": "alice",
    "contractId": "<allocation-contract-id>"
  }'
```

## Frontend

Open the `frontend` endpoint from the Aspire Dashboard.

The frontend provides:
- **Dashboard** — overview of all bonds and parties
- **Mint** — create new bonds
- **Holdings** — view and filter bond holdings
- **Transfer** — initiate two-step transfers
- **Pending** — accept, reject, or withdraw pending transfers
- **Burn** — burn bonds (owner or admin)
- **Parties** — view and create parties
- **Allocations** — create, list, execute, cancel, and withdraw DvP allocations

## Ledger Listener

The listener is a separate service that polls the Canton JSON API to detect `created` and `archived` events for the bond templates and logs them as JSON lines.

Environment variables (Docker defaults in `docker-compose.yml`):
- `LISTENER_PARTICIPANT_URL` (default `http://participant1:5013`)
- `LISTENER_USER_ID` (default `ledger-api-user`)
- `LISTENER_POLL_INTERVAL` (default `2s`)
- `LISTENER_REQUEST_TIMEOUT` (default `30s`)
- `LISTENER_EMIT_INITIAL` (default `false`)
- `LISTENER_TEMPLATES` (comma-separated list; defaults to bond templates)

When running with Aspire, open the `listener` resource logs in the Dashboard.
In Docker Compose fallback mode, use `docker logs -f bond-listener`.

### Verify the listener

1) Confirm the service is running:
```bash
docker ps --format "table {{.Names}}\t{{.Status}}"
```

2) Generate events and watch logs:
```bash
curl -s "$BACKEND_URL/api/v1/factory" | jq .
curl -X POST "$BACKEND_URL/api/v1/mint" \
  -H "Content-Type: application/json" \
  -d '{
    "admin": "admin",
    "owner": "alice",
    "amount": 1000,
    "couponRate": 5.0,
    "maturityDate": "2028-12-31",
    "description": "Test Bond"
  }'
docker logs -f bond-listener
```

## Bond Contract

The bond token contract implements the CIP-056 token standard with:

- **SimpleTokenRules** — Factory contract for minting bonds, managing transfers, and allocations
- **SimpleHolding** — A bond holding with amount, coupon rate, maturity date, and description
- **LockedSimpleHolding** — Locked holding during two-step transfer
- **SimpleTransferInstruction** — Pending transfer (accept/reject/withdraw)
- **SimpleAllocation** — DvP allocation supporting execute, cancel, and withdraw workflows

## Stopping

```bash
Ctrl+C
```

For Docker Compose fallback mode:

```bash
docker compose down
```
