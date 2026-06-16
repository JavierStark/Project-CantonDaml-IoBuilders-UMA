# Migracion progresiva a gRPC

Este documento describe el estado actual de la migracion del proyecto a la
Canton Ledger API gRPC. La API REST publica del backend no cambia: el frontend
continua llamando a los mismos endpoints HTTP bajo `/api/v1`.

## Estado actual

### Backend REST

El backend REST usa ahora gRPC por defecto para comunicarse con los
participantes Canton. El transporte se selecciona con:

```bash
LEDGER_TRANSPORT=grpc   # default
LEDGER_TRANSPORT=http   # fallback legacy
```

Cuando `LEDGER_TRANSPORT=grpc`, el backend usa estos servicios de Ledger API:

| Operacion interna | Servicio gRPC |
|---|---|
| Ledger end | `com.daml.ledger.api.v2.StateService.GetLedgerEnd` |
| Active contracts / holdings / factory / pending | `com.daml.ledger.api.v2.StateService.GetActiveContracts` |
| Submit-and-wait para create/exercise | `com.daml.ledger.api.v2.CommandService.SubmitAndWait` |
| Listar parties | `com.daml.ledger.api.v2.admin.PartyManagementService.ListKnownParties` |
| Crear parties | `com.daml.ledger.api.v2.admin.PartyManagementService.AllocateParty` |

La implementacion HTTP JSON API v2 sigue presente como fallback y usa las rutas
legacy:

- `/v2/state/ledger-end`
- `/v2/state/active-contracts`
- `/v2/parties`
- `/v2/commands/submit-and-wait`

### Listener

El listener ya usaba gRPC y se mantiene asi. Se conecta a
`UpdateService.GetUpdates` y reenvia eventos al frontend mediante WebSocket en
`/ws/bonds`.

El listener mantiene:

```go
IncludeCreatedEventBlob: false
```

Esto es intencionado. El listener actual solo necesita:

- accion del evento (`created` o `archived`),
- `contractId`,
- `templateId`.

No necesita el blob binario del `CreatedEvent`, porque no reenvia contratos
divulgados ni usa el payload opaco para futuras submissions. Activar el blob
solo aumentaria el tamano de los mensajes.

## Endpoints REST publicos

Estos endpoints se mantienen estables para el frontend:

- `GET /api/v1/health`
- `GET /api/v1/parties`
- `POST /api/v1/parties`
- `GET /api/v1/factory`
- `POST /api/v1/factory`
- `POST /api/v1/mint`
- `GET /api/v1/holdings?party=alice`
- `POST /api/v1/transfer`
- `POST /api/v1/transfer/accept`
- `POST /api/v1/transfer/reject`
- `POST /api/v1/transfer/withdraw`
- `POST /api/v1/burn`
- `POST /api/v1/self-transfer`
- `GET /api/v1/transfer-instructions?party=alice`
- endpoints de `allocations`

## Configuracion Docker

El backend recibe URLs HTTP y gRPC por participante. gRPC es el transporte por
defecto; HTTP queda disponible para fallback.

```yaml
backend:
  environment:
    - LEDGER_TRANSPORT=grpc
    - PARTICIPANT1_URL=http://participant1:5013
    - PARTICIPANT1_GRPC_URL=participant1:5011
    - PARTICIPANT2_URL=http://participant2:5023
    - PARTICIPANT2_GRPC_URL=participant2:5021
    - PARTICIPANT3_URL=http://participant3:5033
    - PARTICIPANT3_GRPC_URL=participant3:5031
```

Al arrancar, el backend imprime el transporte activo:

```text
ledger transport: grpc
participant participant1 -> participant1:5011 (grpc), fallback http http://participant1:5013
```

Para volver temporalmente al modo HTTP:

```bash
LEDGER_TRANSPORT=http docker compose up -d --build backend
```

## Verificacion

### Build y tests Go

Desde `canton-bond-platform`:

```bash
cd pkg/cantonledger && go test ./...
cd ../../backend && go test ./...
cd ../listener && go test ./...
```

Si el entorno tiene restricciones de cache:

```bash
GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./...
```

### Arranque Docker

```bash
docker compose up -d --build
docker compose ps
```

Antes de probar endpoints, esperar a que los tres participantes terminen el
bootstrap y aparezcan como `healthy`:

```text
participant1   Up ... (healthy)
participant2   Up ... (healthy)
participant3   Up ... (healthy)
```

No es suficiente con que el backend responda `/health`: el backend puede
arrancar antes de que Canton haya abierto la Ledger API gRPC o antes de que el
script de bootstrap haya creado las parties. Si se prueba demasiado pronto,
pueden aparecer respuestas como `[]` en `/api/v1/parties` o
`failed to query ledger end` en `/api/v1/factory`.

Comprobar en logs del backend:

```bash
docker compose logs backend
```

```text
ledger transport: grpc
participant participant1 -> participant1:5011 (grpc), fallback http http://participant1:5013
participant participant2 -> participant2:5021 (grpc), fallback http http://participant2:5023
participant participant3 -> participant3:5031 (grpc), fallback http http://participant3:5033
```

Comprobar en logs del listener:

```bash
docker compose logs listener
```

```text
¡ÉXITO! ID resuelto: admin -> admin::...
Bond Listener Iniciado con gRPC
Stream gRPC abierto. Esperando eventos para enviar a los WebSockets...
```

Mientras aparezca `Canton aún no ha generado la party 'admin'`, esperar y
reintentar. En un arranque limpio con storage en memoria puede tardar 1-2
minutos.

### Pruebas REST basicas

```bash
curl -s http://localhost:8080/api/v1/health
curl -s http://localhost:8080/api/v1/parties
curl -s -X POST http://localhost:8080/api/v1/factory
```

Mint:

```bash
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
```

Holdings:

```bash
curl -s "http://localhost:8080/api/v1/holdings?party=alice"
```

Transfer:

```bash
curl -s -X POST http://localhost:8080/api/v1/transfer \
  -H "Content-Type: application/json" \
  -d '{
    "sender": "alice",
    "receiver": "bob",
    "amount": 100
  }'
```

Accept transfer:

```bash
curl -s "http://localhost:8080/api/v1/transfer-instructions?party=bob"

curl -s -X POST http://localhost:8080/api/v1/transfer/accept \
  -H "Content-Type: application/json" \
  -d '{
    "party": "bob",
    "contractId": "<transfer-instruction-cid>"
  }'
```

Burn:

```bash
curl -s -X POST http://localhost:8080/api/v1/burn \
  -H "Content-Type: application/json" \
  -d '{
    "party": "alice",
    "contractId": "<holding-cid>"
  }'
```

### Verificacion del listener

Mientras se ejecutan `mint`, `transfer` o `burn`:

```bash
docker compose logs -f listener
```

El listener debe imprimir eventos de creacion y archivado recibidos por gRPC.

### Fallback HTTP

Para comprobar que el backend sigue pudiendo usar la HTTP JSON API v2:

```bash
LEDGER_TRANSPORT=http docker compose up -d --build backend
docker compose logs backend
```

El log debe mostrar:

```text
ledger transport: http
```

Y las comprobaciones basicas deben seguir funcionando:

```bash
curl -s http://localhost:8080/api/v1/health
curl -s http://localhost:8080/api/v1/parties
```

Para volver a gRPC, arrancar de nuevo con el valor del `docker-compose.yml`:

```bash
docker compose up -d --build backend
```

## Limitaciones conocidas

- La migracion evita un refactor amplio del dominio. Los handlers REST siguen
  construyendo payloads Daml como mapas y el cliente gRPC los convierte a
  protobuf con un encoder limitado a los payloads reales del proyecto.
- El encoder gRPC no es un serializador universal de Daml. Cubre factory,
  mint, transfer, accept/reject/withdraw, burn y las estructuras usadas por
  allocations en este proyecto.
- Si un payload nuevo introduce variantes, enums o mapas complejos no cubiertos,
  debe extenderse el encoder o ejecutarse con `LEDGER_TRANSPORT=http` hasta
  migrarlo.
- La implementacion gRPC usa credenciales inseguras (`insecure.NewCredentials`)
  porque la red es local/Docker. Para un entorno productivo habria que configurar
  TLS/autenticacion.
