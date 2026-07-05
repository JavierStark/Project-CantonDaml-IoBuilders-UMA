# Migracion progresiva a gRPC

Este documento describe el estado de la migracion del proyecto a la
Canton Ledger API gRPC. La API REST publica del backend no cambia: el frontend
continua llamando a los mismos endpoints HTTP bajo `/api/v1`.

## Estado: Completado

La migracion gRPC esta completa y estable. El backend usa gRPC por defecto para
lecturas y HTTP JSON API v2 para escrituras. El listener usa gRPC para streaming.
El modo HTTP permanece como fallback configurable.

### Backend REST

El backend REST usa gRPC por defecto para comunicarse con los
participantes Canton. El transporte se selecciona con:

```bash
LEDGER_TRANSPORT=grpc   # default (recomendado)
LEDGER_TRANSPORT=http   # fallback legacy
```

Cuando `LEDGER_TRANSPORT=grpc`, el backend usa:

| Operacion interna | Servicio |
|---|---|
| Ledger end | gRPC `StateService.GetLedgerEnd` |
| Active contracts / holdings / factory / pending | gRPC `StateService.GetActiveContracts` (streaming) |
| Submit-and-wait (create/exercise) | HTTP JSON API v2 `/v2/commands/submit-and-wait` |
| Listar parties | HTTP JSON API v2 `GET /v2/parties` |
| Crear parties | HTTP JSON API v2 `POST /v2/parties` |

**Motivo del diseno hibrido**: las lecturas via gRPC evitan el overhead de
serializacion JSON y permiten streaming nativo. Las escrituras se mantienen en
HTTP JSON API v2 porque construir valores Daml en protobuf es complejo (tipos
anidados, sum types, enums) y el encoder gRPC solo cubre los payloads de este
proyecto.

La implementacion HTTP JSON API v2 sigue presente como fallback en
`pkg/cantonledger/client.go`.

### Listener

El listener usa gRPC `UpdateService.GetUpdates` para recibir eventos del ledger
en tiempo real. Los eventos se reenvian al frontend via WebSocket en
`ws://localhost:8081/ws/bonds`.

```go
IncludeCreatedEventBlob: false
```

Esto es intencionado. El listener solo necesita:
- accion del evento (`created` o `archived`),
- `contractId`,
- `templateId`.

No necesita el blob binario del `CreatedEvent`. Activar el blob solo aumentaria
el tamano de los mensajes.

### Arranque

El proyecto se inicia con `./start.sh`, que levanta todos los servicios
incluyendo el backend en modo gRPC:

```bash
./start.sh
```

Para arrancar manualmente con verificacion:

```bash
docker compose up -d --build
docker compose ps
```

Antes de probar endpoints, esperar a que los participantes terminen el
bootstrap. El script `start.sh` se encarga automaticamente de esto.

### Verificacion

```bash
# Build y tests Go
cd pkg/cantonledger && go test ./...
cd ../../backend && go test ./...
cd ../listener && go test ./...

# Pruebas REST
curl -s http://localhost:8080/api/v1/health
curl -s http://localhost:8080/api/v1/parties
curl -s -X POST http://localhost:8080/api/v1/factory

# Mint
curl -s -X POST http://localhost:8080/api/v1/mint \
  -H "Content-Type: application/json" \
  -d '{
    "admin": "admin",
    "owner": "alice",
    "amount": 1000,
    "couponRate": 5,
    "maturityDate": "2028-12-31",
    "description": "Corporate Bond A"
  }'

# Holdings
curl -s "http://localhost:8080/api/v1/holdings?party=alice"
```

### Fallback HTTP

Para comprobar que el backend sigue pudiendo usar la HTTP JSON API v2:

```bash
LEDGER_TRANSPORT=http docker compose up -d --build backend
```

Y las comprobaciones basicas deben seguir funcionando:

```bash
curl -s http://localhost:8080/api/v1/health
curl -s http://localhost:8080/api/v1/parties
```

Para volver a gRPC:

```bash
docker compose up -d --build backend
```

## Limitaciones conocidas

- El encoder gRPC no es un serializador universal de Daml. Cubre factory,
  mint, transfer, accept/reject/withdraw, burn, y las estructuras usadas por
  allocations en este proyecto.
- Si un payload nuevo introduce variantes, enums o mapas complejos no cubiertos,
  debe extenderse el encoder en `pkg/cantonledger/grpc_values.go` o ejecutarse
  con `LEDGER_TRANSPORT=http` hasta migrarlo.
- La implementacion gRPC usa credenciales inseguras (`insecure.NewCredentials`)
  porque la red es local/Docker. Para un entorno productivo habria que configurar
  TLS/autenticacion.
- Las escrituras (`submit-and-wait`) siguen usando HTTP JSON API v2 incluso en
  modo gRPC. Esto es por diseno — el encoder gRPC cubre lecturas y eventos de
  streaming, pero la construccion de comandos Daml en protobuf es mas compleja.
- La migracion completa de escrituras a gRPC nativo (incluyendo
  `CommandSubmissionService.SubmitAndWait`) requeriria un encoder Daml→protobuf
  completo.
