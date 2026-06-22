#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

if [ -x "$HOME/.dotnet/dotnet" ]; then
    export DOTNET_ROOT="${DOTNET_ROOT:-$HOME/.dotnet}"
    export PATH="$HOME/.dotnet:$PATH"
fi

if [ -x "$HOME/.local/go/bin/go" ]; then
    export PATH="$HOME/.local/go/bin:$PATH"
fi

ASPIRE_BIN="${ASPIRE_BIN:-}"

if [ -z "$ASPIRE_BIN" ]; then
    if command -v aspire >/dev/null 2>&1; then
        ASPIRE_BIN="$(command -v aspire)"
    elif [ -x "$HOME/.aspire/bin/aspire" ]; then
        ASPIRE_BIN="$HOME/.aspire/bin/aspire"
    else
        echo "Aspire CLI not found. Install it first, then rerun this script." >&2
        exit 1
    fi
fi

exec "$ASPIRE_BIN" run --non-interactive --nologo
