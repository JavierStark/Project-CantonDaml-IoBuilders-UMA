# Canton Learning Labs

Esta carpeta agrupa los prototipos, pruebas y ejercicios que se usaron para
aprender Canton, Daml y la integracion con backends antes de llegar a
`canton-bond-platform`.

No es la aplicacion final. La entrega principal vive en
[`../canton-bond-platform`](../canton-bond-platform).

## Contenido

- `NetworkSetup`: topologia Canton simple en un unico proceso, con scripts de
  bootstrap y notas de REPL.
- `DockerizedSimpleTopology`: topologia Canton dockerizada con tres
  participants, sequencer, mediator y synchronizer.
- `CashBalance_TestContract`: contrato Daml de prueba para practicar build,
  carga de DAR y llamadas a la JSON API.
- `canton-token-template`: implementacion Daml del token/bond y tests de
  lifecycle, transferencias, allocations y casos negativos.
- `go_backend_prueba`: backend Go experimental que habla con Canton mediante
  JSON Ledger API.
- `iobuilders-digital-bond`: prototipo previo de bono digital y artefactos Daml.

La carpeta se conserva porque contiene referencias utiles para depurar,
comparar decisiones de diseno y entender la evolucion del proyecto.
