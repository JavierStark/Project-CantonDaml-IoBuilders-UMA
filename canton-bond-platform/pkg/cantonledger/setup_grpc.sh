#!/bin/bash
echo "1. Limpiando directorios antiguos..."
rm -rf proto/com
rm -rf /tmp/canton

echo "2. Clonando repositorio oficial de Canton..."
git clone --depth 1 https://github.com/digital-asset/canton.git /tmp/canton

echo "3. Preparando esquemas gRPC..."
PROTO_DIR=$(find /tmp/canton -type d -path "*/com/daml/ledger/api/v2" | head -n 1)
mkdir -p proto/com/daml/ledger/api/v2
cp -r $PROTO_DIR/* proto/com/daml/ledger/api/v2/

echo "4. Inyectando el nombre EXACTO de tu go.mod..."
cd proto
# EL CAMBIO CLAVE ESTÁ AQUÍ:
MOD_PATH="canton-bond-platform/pkg/cantonledger/proto/com/daml/ledger/api/v2;v2"

find com/daml/ledger/api/v2 -name "*.proto" | while read -r file; do
  if ! grep -q "go_package" "$file"; then
    echo "" >> "$file"
    echo "option go_package = \"$MOD_PATH\";" >> "$file"
  fi
done

echo "5. ¡Compilando binarios de gRPC!"
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       -I . \
       $(find com/daml/ledger/api/v2 -name "*.proto")

echo "✅ ¡COMPILACIÓN EXITOSA! Ya hablamos gRPC."
