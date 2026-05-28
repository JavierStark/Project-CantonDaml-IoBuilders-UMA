# Backend Go para Canton + DAML (bonos + liquidacion atomica)

Este backend es un esqueleto minimalista en Go que se conecta al **JSON Ledger API** de Canton para:
- crear contratos DAML (bonos)
- ejercer choices (liquidacion atomica contra USDC)
- consultar contratos activos

Se asume un entorno local con `participant1` y **TLS desactivado**.

## Requisitos
- Go 1.20+
- Canton 3.4.11 corriendo con `examples/01-simple-topology/simple-topology.conf`
- JSON Ledger API habilitada (puerto 5013)
- Daml SDK 3.4.11 (para compilar el contrato de prueba)

En ese ejemplo:
- `participant1` gRPC ledger-api: `127.0.0.1:5011`
- `participant1` JSON Ledger API: `127.0.0.1:5013`

Este backend usa **JSON Ledger API** por simplicidad.

## Estructura

```text
go_backend_prueba/
├── cmd/
│   └── server/
│       └── main.go               # servidor HTTP
├── internal/
│   ├── config/
│   │   └── config.go             # configuracion por variables de entorno
│   ├── httpapi/
│   │   └── server.go             # endpoints REST
│   └── ledger/
│       └── client.go             # cliente HTTP al JSON Ledger API
└── daml/
    ├── daml.yaml
    └── Loan/
        └── Main.daml
```

## Configuracion (variables de entorno)

```
CANTON_LEDGER_API_HOST=127.0.0.1
CANTON_LEDGER_API_PORT=5013
CANTON_USER_ID=ledger-api-user
CANTON_PARTY=<Issuer PartyId>
TEMPLATE_BOND=#Loan.Main:DebtInstrument
CHOICE_SETTLE=AtomicSettlement
HTTP_HOST=127.0.0.1
HTTP_PORT=8080
REQUEST_TIMEOUT=10s
```

Notas:
- `TEMPLATE_BOND` debe coincidir con el template de tu DAML.
- `CANTON_PARTY` es el party que firma los comandos. Usa el PartyId completo que devuelve Canton (por ejemplo `Issuer::1220...`).
- Si activas TLS, este ejemplo debe cambiar a gRPC con certificados.

## Ejecutar

### Iniciar Canton (simple-topology)

1) Arranca Canton con la configuracion de ejemplo y el bootstrap:
```
cd /home/rgb/canton-open-source-3.4.11/examples/01-simple-topology
../../bin/canton -c simple-topology.conf --bootstrap simple-ping.canton
```

El bootstrap inicializa el synchronizer y conecta los nodos.

2) Verifica que el JSON Ledger API esta vivo:
```
curl http://127.0.0.1:5013/v2/state/ledger-end
```
`{"offset":19}` confirma que el JSON Ledger API v2 del participant esta vivo y ya hay actividad en el ledger (bootstrap, etc.)

### Parties

Para que los ejemplos funcionen, necesitas parties. Canton devuelve un PartyId completo (por ejemplo `Issuer::1220...`) y ese es el valor que debes usar en `CANTON_PARTY` y en los payloads.

Crear parties (JSON API):

```bash
curl -X POST http://127.0.0.1:5013/v2/parties \
  -H "Content-Type: application/json" \
  -d '{"partyIdHint":"Issuer"}'

curl -X POST http://127.0.0.1:5013/v2/parties \
  -H "Content-Type: application/json" \
  -d '{"partyIdHint":"Investor"}'
```

Parties creadas en esta maquina (ejemplo real):
- `Issuer::1220aeeeffa894d69b10260e351d3289837a089531d9fc79cef1c53161583243873e`
- `Investor::1220aeeeffa894d69b10260e351d3289837a089531d9fc79cef1c53161583243873e`

Listar parties:

```bash
curl http://127.0.0.1:5013/v2/parties
```

Nota: con `storage.type = memory` (config de ejemplo), si reinicias Canton puedes tener que recrear parties y re-subir el DAR.

### Iniciar el backend Go (carpeta raíz proyecto)

```
go run ./cmd/server
```

## Contrato DAML de prueba 

Este repo incluye un contrato DAML minimo para pruebas:
- `daml/daml.yaml`
- `daml/Loan/Main.daml`

Templates:
- `Loan.Main:DebtInstrument`
- `Loan.Main:USDC`

La liquidacion atomica se hace con el choice `AtomicSettlement` en `DebtInstrument`.

### Compilar el DAML

Si el comando `daml` no existe, primero instala el Daml SDK 3.4.11.

Desde la carpeta `daml`:
```
cd /home/rgb/Documentos/UNIVERSIDAD/Curso_Blockchain/go_backend_prueba/daml
daml build
```

Nota: si ya tienes DPM instalado, el equivalente actualizado es `dpm build`.

Se genera un `.dar` en:
```
.daml/dist/debt-settlement-0.0.1.dar
```

### Subir el DAR al participant

En la consola de Canton:
```
participant1.dars.upload("/home/rgb/Documentos/UNIVERSIDAD/Curso_Blockchain/go_backend_prueba/daml/.daml/dist/debt-settlement-0.0.1.dar")
participant1.dars.list()
```

### Cuando hay que volver a subir el DAR

- Si modificas el contrato DAML: **si**.
- Si reinicias Canton con `storage.type = memory`: **si**.
- Si no cambias DAML y Canton sigue vivo: **no**.

## Endpoints

### Health
`GET /health`

### Ledger end
`GET /ledger-end`

### Contratos activos
`GET /contracts`

### Crear bono
`POST /bonds`

Body:
```
{
  "commandId": "bond-1",
  "args": {
    "issuer": "<Issuer PartyId>",
    "owner": "<Investor PartyId>",
    "notional": "1000",
    "currency": "USD"
  }
}
```

Ejemplo para crear el USDC de prueba:
```
curl -s http://127.0.0.1:8080/bonds \
  -H "Content-Type: application/json" \
  -d '{"commandId":"usdc-1","args":{"issuer":"<Issuer PartyId>","owner":"<Investor PartyId>","amount":"1000"}}'
```

### Liquidacion atomica
`POST /settlements`

Body:
```
{
  "commandId": "settle-1",
  "templateId": "#Loan.Main:DebtInstrument",
  "contractId": "<contract-id>",
  "choice": "AtomicSettlement",
  "choiceArg": {
    "usdcCid": "<usdc-contract-id>"
  }
}
```

## Notas de integracion con DAML

1. Compila el DAML y sube el DAR al participant.
2. Crea parties (`Issuer`, `Investor`) y usa el PartyId completo (`Issuer::...`).
3. Ajusta `TEMPLATE_BOND` a `#Loan.Main:DebtInstrument`.
4. Ajusta los argumentos segun el template (ver ejemplo arriba).

Nota: el template `USDC` tiene `signatory issuer, owner`. Si `issuer != owner`, crear ese contrato requiere firma de ambas partes; este backend es minimalista y firma con un solo `CANTON_PARTY`.

## Proximos pasos a seguir
1. Reemplazar JSON Ledger API por gRPC Ledger API (mas tipado y robusto).
2. Agregar autenticacion JWT si activas auth en Canton.
3. Implementar watchers de eventos con websockets o UpdateService.
```
