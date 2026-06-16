#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

log()  { echo -e "\033[0;36m[INFO]\033[0m $1"; }
ok()   { echo -e "\033[0;32m[OK]\033[0m   $1"; }
err()  { echo -e "\033[0;31m[ERR]\033[0m  $1"; }

# ---- find AppHost root ----
# walk up to find aspire.config.json or apphost.cs
APPROOT="$SCRIPT_DIR"
while [ "$APPROOT" != "/" ]; do
  if [ -f "$APPROOT/aspire.config.json" ] || [ -f "$APPROOT/apphost.cs" ]; then
    break
  fi
  APPROOT="$(dirname "$APPROOT")"
done

if [ "$APPROOT" = "/" ]; then
  err "Could not find Aspire AppHost (aspire.config.json or apphost.cs) in parent directories."
  exit 1
fi

log "AppHost root: $APPROOT"

# ---- discover Aspire endpoints ----
log "Discovering Aspire endpoints..."

DESCRIBE=$(cd "$APPROOT" && aspire describe --format Json --non-interactive 2>/dev/null || true)

if [ -z "$DESCRIBE" ]; then
    err "No running AppHost found. Start it from $APPROOT with: aspire start"
    exit 1
fi

EXTRACT=$(node -e "
const data = JSON.parse(process.argv[1]);
let frontend = '', backend = '';
for (const r of data.resources) {
  if (r.displayName === 'frontend' && r.state === 'Running') {
    for (const u of r.urls) {
      if (u.name === 'http') { frontend = u.url; break; }
    }
  }
  if (r.displayName === 'backend' && r.state === 'Running') {
    for (const u of r.urls) {
      if (u.name === 'http') { backend = u.url; break; }
    }
  }
}
if (!frontend || !backend) process.exit(1);
console.log(frontend);
console.log(backend);
" "$DESCRIBE" 2>/dev/null || true)

FRONTEND_URL=$(echo "$EXTRACT" | sed -n '1p')
BACKEND_URL=$(echo "$EXTRACT" | sed -n '2p')

if [ -z "$FRONTEND_URL" ] || [ -z "$BACKEND_URL" ]; then
    err "Could not discover frontend or backend endpoints from running AppHost."
    err "Ensure 'aspire start' is running and all resources are healthy."
    exit 1
fi

ok "Frontend: $FRONTEND_URL"
ok "Backend:  $BACKEND_URL"

# ---- run Playwright ----
log "Running Playwright tests..."
export E2E_API_URL="$BACKEND_URL"
export PLAYWRIGHT_BASE_URL="$FRONTEND_URL"

npx playwright test "$@"
