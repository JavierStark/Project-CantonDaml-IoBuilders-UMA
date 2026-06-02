#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"

VERSION="${1:-1.0.4}"

echo "Building Canton.Aspire.Hosting v${VERSION}..."
dotnet pack -c Release -o ../LocalPackages "/p:Version=${VERSION}"

echo ""
echo "Package created: ../LocalPackages/Canton.Aspire.Hosting.${VERSION}.nupkg"
echo ""
echo "Update apphost.cs to reference the new version if needed:"
echo "  #:package Canton.Aspire.Hosting@${VERSION}"
