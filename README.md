# Project Canton Daml IoBuilders UMA

Este repositorio contiene una plataforma de bonos digitales construida sobre
Canton y Daml. La entrega principal esta en [`canton-bond-platform`](./canton-bond-platform);
el resto de ejercicios y prototipos usados para aprender Canton se han agrupado
en [`canton-learning-labs`](./canton-learning-labs).

## Proyecto principal

`canton-bond-platform` es una red Canton dockerizada con:

- Contrato Daml de bono tokenizado basado en interfaces CIP-056.
- Backend Go con API REST para operar contra Canton Ledger API gRPC.
- Frontend web para emitir, consultar, transferir, quemar y liquidar bonos.
- Listener de eventos del ledger.
- Stack local de observabilidad con OpenTelemetry, Prometheus, Tempo y Grafana.
- Pruebas end-to-end con Playwright.

La plataforma levanta una topologia Canton local con un sequencer, un mediator,
un synchronizer y tres participants. Sobre esa red despliega el contrato de bono
y expone una aplicacion para probar el ciclo completo:

1. Crear o consultar parties.
2. Inicializar el contrato factory `SimpleTokenRules`.
3. Emitir bonos para una party.
4. Transferir bonos con flujo de aceptacion, rechazo o retirada.
5. Quemar holdings.
6. Crear y ejecutar allocations para flujos DvP.
7. Observar metricas, trazas y eventos.

## Arranque rapido

Desde la carpeta principal de la plataforma:

```bash
cd canton-bond-platform
docker compose up -d
```

URLs locales principales:

- Frontend: http://localhost:3000
- Backend REST: http://localhost:8080/api/v1
- Grafana: http://localhost:3001
- Prometheus: http://localhost:9090

Para mas detalle, ver [`canton-bond-platform/README.md`](./canton-bond-platform/README.md).

## Material de aprendizaje

`canton-learning-labs` conserva los proyectos intermedios que ayudaron a
entender Canton, Daml, la JSON API, topologias simples y primeros backends.
No forman parte de la entrega final, pero sirven como referencia historica y
como ejemplos mas pequenos para aislar conceptos.

Contenido:

- `NetworkSetup`: tutorial de topologia Canton en un proceso.
- `DockerizedSimpleTopology`: primera topologia Canton dockerizada.
- `CashBalance_TestContract`: contrato Daml minimo y pruebas con JSON API.
- `canton-token-template`: contrato token/bond en Daml con tests.
- `go_backend_prueba`: backend Go experimental contra JSON Ledger API.
- `iobuilders-digital-bond`: prototipo previo de bono digital.

## Estructura

```text
.
├── canton-bond-platform/     # Plataforma final de bonos digitales
├── canton-learning-labs/     # Prototipos y ejercicios de aprendizaje
└── README.md
```
