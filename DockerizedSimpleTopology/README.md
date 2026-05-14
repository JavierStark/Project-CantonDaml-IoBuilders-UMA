# Canton Simple Topology

A Dockerized Canton network with 2 participants, 1 sequencer, 1 mediator, and an auto-initialized synchronizer domain.

## Prerequisites

- [Docker](https://docs.docker.com/engine/install/) + [Docker Compose](https://docs.docker.com/compose/install/)
- Pre-pulled Canton images: `europe-docker.pkg.dev/da-images/public/docker/canton-{base,sequencer,mediator,participant}:3.4.8`

## Topology

```
                    ┌──────────────────┐
                    │   synchronizer   │  (creates domain "da")
                    └────────┬─────────┘
                             │
            ┌────────────────┼────────────────┐
            │                │                 │
    ┌───────▼──────┐  ┌─────▼──────┐  ┌──────▼──────────┐
    │  sequencer1   │  │ mediator1  │  │  participant1   │
    │  public:5001  │  │ admin:5202 │  │  ledger:5011    │
    │  admin:5002   │  │            │  │  admin:5012     │
    └───────────────┘  └────────────┘  │  http-json:5013 │
                                       └─────────────────┘
                                       ┌─────────────────┐
                                       │  participant2   │
                                       │  ledger:5021    │
                                       │  admin:5022     │
                                       │  http-json:5023 │
                                       └─────────────────┘
```

All nodes use **in-memory storage** — data resets on restart, but bootstrap scripts auto-initialize everything.

## Quick Start

```bash
cd dev/simple-topology
docker compose up -d
```

Wait ~60 seconds for bootstrap completion, then verify:

```bash
docker ps --format "table {{.Names}}\t{{.Status}}"
```

Expected output:
```
canton-postgres    Up X minutes (healthy)
sequencer1         Up X minutes
mediator1          Up X minutes
synchronizer       Up X minutes
participant1       Up X minutes (healthy)
participant2       Up X minutes (healthy)
```

## Testing with curl

### List parties on each participant

```bash
curl http://localhost:5013/v2/parties      # participant1
curl http://localhost:5023/v2/parties      # participant2
```

### List users

```bash
curl http://localhost:5013/v2/users
```

### Allocate a new party

```bash
curl -X POST -H "Content-Type: application/json" \
  -d '{"partyIdHint":"alice"}' \
  http://localhost:5013/v2/parties
```

## Shutdown

```bash
cd dev/simple-topology
docker compose down
```

## Project Structure

```
dev/simple-topology/
├── docker-compose.yml          # Service definitions
├── configs/
│   ├── sequencer-bootstrap.sc      # Sequencer identity initialization
│   ├── mediator-bootstrap.sc       # Mediator identity initialization
│   ├── synchronizer-bootstrap.sc   # Synchronizer domain creation
│   ├── participant1-bootstrap.sc   # Participant1 init + domain connect
│   ├── participant2-bootstrap.sc   # Participant2 init + domain connect
│   └── synchronizer-remote.conf    # Remote node config for synchronizer
```
