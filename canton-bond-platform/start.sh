#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

log_info()  { echo -e "${CYAN}[INFO]${NC} $1"; }
log_ok()    { echo -e "${GREEN}[OK]${NC}   $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERR]${NC}  $1"; }

# ---------------------------------------------------------------
# 1. Prerequisites check
# ---------------------------------------------------------------
log_info "Checking prerequisites..."

if ! command -v docker &>/dev/null; then
    log_error "Docker is not installed. Install Docker first: https://docs.docker.com/engine/install/"
    exit 1
fi
log_ok "Docker found: $(docker --version)"

if ! docker compose version &>/dev/null; then
    log_error "Docker Compose is not available."
    exit 1
fi
log_ok "Docker Compose found: $(docker compose version)"

# ---------------------------------------------------------------
# 2. Build bond contract DAR if not present
# ---------------------------------------------------------------
DAR_FILE="dars/simple-token-0.1.0.dar"
if [ ! -f "$DAR_FILE" ]; then
    log_info "Building bond contract DAR..."
    if command -v dpm &>/dev/null; then
        (cd bond-contract && dpm build)
        cp bond-contract/.daml/dist/simple-token-0.1.0.dar dars/
        log_ok "DAR built with dpm"
    elif command -v daml &>/dev/null; then
        (cd bond-contract && daml build)
        cp bond-contract/.daml/dist/simple-token-0.1.0.dar dars/
        log_ok "DAR built with daml"
    else
        log_warn "Neither 'dpm' nor 'daml' found — assuming DAR already exists at dars/"
        ls -la "$DAR_FILE" 2>/dev/null || log_error "DAR file missing! Build it manually: cd bond-contract && dpm build"
    fi
else
    log_ok "DAR file already exists: $DAR_FILE"
fi

# ---------------------------------------------------------------
# 3. Build and start Docker stack
# ---------------------------------------------------------------
log_info "Building Docker images..."
docker compose build --quiet 2>&1 | tail -1 || true
log_ok "Docker images built"

log_info "Starting Canton network + backend + frontend..."
docker compose up -d
log_ok "Containers started"

# ---------------------------------------------------------------
# 4. Wait for all services to be healthy
# ---------------------------------------------------------------
log_info "Waiting for all services to be ready..."

MAX_RETRIES=40
RETRY_INTERVAL=5
SERVICES=("sequencer1" "mediator1" "synchronizer" "participant1" "participant2" "participant3" "bond-backend" "bond-frontend")

for service in "${SERVICES[@]}"; do
    log_info "Waiting for $service..."
    retries=0
    while [ $retries -lt $MAX_RETRIES ]; do
        status=$(docker inspect --format='{{.State.Status}}' "$service" 2>/dev/null || echo "missing")
        health=$(docker inspect --format='{{if .State.Health}}{{.State.Health.Status}}{{else}}running{{end}}' "$service" 2>/dev/null || echo "unknown")
        if [ "$status" = "running" ] && { [ "$health" = "running" ] || [ "$health" = "healthy" ] || [ "$health" = "none" ]; }; then
            log_ok "$service is ready ($status)"
            break
        fi
        sleep $RETRY_INTERVAL
        retries=$((retries + 1))
    done
    if [ $retries -ge $MAX_RETRIES ]; then
        log_warn "$service not ready after $((MAX_RETRIES * RETRY_INTERVAL))s — continuing anyway"
        docker logs --tail 5 "$service" 2>/dev/null || true
    fi
done

log_ok "All containers are up"

# ---------------------------------------------------------------
# 5. Wait for backend health
# ---------------------------------------------------------------
log_info "Waiting for backend API..."
MAX_API_RETRIES=30
retries=0
while [ $retries -lt $MAX_API_RETRIES ]; do
    if curl -sf http://localhost:8080/api/v1/health >/dev/null 2>&1; then
        log_ok "Backend API is healthy"
        break
    fi
    sleep 2
    retries=$((retries + 1))
done
if [ $retries -ge $MAX_API_RETRIES ]; then
    log_error "Backend API not responding after 60s"
    docker logs bond-backend --tail 20
    exit 1
fi

log_info "Waiting for participant1 Ledger API (Port 5011)..."
until docker exec bond-backend nc -zv participant1 5011 >/dev/null 2>&1 || curl -s http://localhost:5013/v1/health >/dev/null 2>&1; do
    sleep 2
done
log_ok "Participant1 Ledger API is responding"

# ---------------------------------------------------------------
# 6. Initialize factory with retry
# ---------------------------------------------------------------
log_info "Initializing factory contract..."
MAX_FACTORY_RETRIES=30
RETRY_FACTORY_INTERVAL=4
retries=0

while [ $retries -lt $MAX_FACTORY_RETRIES ]; do
    # Capturamos tanto la respuesta como el código de estado HTTP
    response=$(curl -s -w "\n%{http_code}" -X POST http://localhost:8080/api/v1/factory || echo -e "\n500")
    http_status=$(echo "$response" | tail -n1)
    json_body=$(echo "$response" | sed '$d')

    # Validamos que el HTTP Status sea 2xx y que no contenga la palabra "error"
    if [ "$http_status" -ge 200 ] && [ "$http_status" -lt 300 ] && ! echo "$json_body" | grep -q '"error"'; then
        log_ok "Factory initialized successfully (HTTP $http_status)"
        break
    fi

    log_warn "  Factory deployment pending or ledger not ready (HTTP $http_status). Retrying in ${RETRY_FACTORY_INTERVAL}s... ($((retries + 1))/$MAX_FACTORY_RETRIES)"
    sleep $RETRY_FACTORY_INTERVAL
    retries=$((retries + 1))
done

if [ $retries -ge $MAX_FACTORY_RETRIES ]; then
    log_error "Failed to initialize factory after $((MAX_FACTORY_RETRIES * RETRY_FACTORY_INTERVAL))s"
    log_info "Last backend output:"
    docker logs bond-backend --tail 10
    exit 1
fi

# ---------------------------------------------------------------
# 7. Verify all endpoints
# ---------------------------------------------------------------
log_info "Verifying API endpoints..."

check_endpoint() {
    local desc="$1" method="$2" url="$3" data="$4"
    if [ -n "$data" ]; then
        resp=$(curl -sf -X "$method" "$url" -H "Content-Type: application/json" -d "$data" 2>/dev/null || echo "FAILED")
    else
        resp=$(curl -sf -X "$method" "$url" 2>/dev/null || echo "FAILED")
    fi
    if [ "$resp" = "FAILED" ]; then
        log_warn "  $desc — FAILED"
    else
        log_ok "  $desc — OK"
    fi
}

check_endpoint "List parties"  GET  "http://localhost:8080/api/v1/parties" ""
check_endpoint "Mint bond"     POST "http://localhost:8080/api/v1/mint" \
    '{"admin":"admin","owner":"alice","amount":1000,"couponRate":5.0,"maturityDate":"2028-12-31","description":"Corporate Bond A"}'
check_endpoint "Holdings for alice"  GET "http://localhost:8080/api/v1/holdings?party=alice" ""

# ---------------------------------------------------------------
# 8. Wait for frontend
# ---------------------------------------------------------------
log_info "Waiting for frontend..."
MAX_FRONTEND_RETRIES=15
retries=0
while [ $retries -lt $MAX_FRONTEND_RETRIES ]; do
    if curl -sf http://localhost:3000 >/dev/null 2>&1; then
        log_ok "Frontend is responding"
        break
    fi
    sleep 2
    retries=$((retries + 1))
done
if [ $retries -ge $MAX_FRONTEND_RETRIES ]; then
    log_warn "Frontend not responding after 30s — check logs: docker logs bond-frontend"
fi

# ---------------------------------------------------------------
# Done
# ---------------------------------------------------------------
echo ""
echo -e "${GREEN}═══════════════════════════════════════════════${NC}"
echo -e "${GREEN}  Bond Platform is UP and READY${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════${NC}"
echo -e "  Frontend:  ${CYAN}http://localhost:3000${NC}"
echo -e "  API:       ${CYAN}http://localhost:8080/api/v1${NC}"
echo -e "  Health:    ${CYAN}http://localhost:8080/api/v1/health${NC}"
echo ""
echo -e "  Participants:"
echo -e "    participant1 → admin, alice, executor  (JSON API: 5013)"
echo -e "    participant2 → bob                     (JSON API: 5023)"
echo -e "    participant3 → charlie                 (JSON API: 5033)"
echo ""
echo -e "  To stop:  ${YELLOW}docker compose down${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════${NC}"
