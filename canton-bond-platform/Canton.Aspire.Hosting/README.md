# Canton.Aspire.Hosting

Custom Aspire hosting integration for [Canton](https://www.digitalasset.com/products/canton) network
nodes — sequencer, mediator, synchronizer, and participant. Provides typed extension methods on
`IDistributedApplicationBuilder` so you can declare Canton infrastructure with one-liners instead
of verbose `AddContainer()` blocks.

## API

### Sequencer

```csharp
var sequencer1 = builder.AddCantonSequencer("sequencer1");
// or with custom ports:
builder.AddCantonSequencer("sequencer1", publicPort: 6001, adminPort: 6002);
// or override public port only:
builder.AddCantonSequencer("sequencer1").WithPublicPort(6001);
```

| Method | Default |
|--------|---------|
| `AddCantonSequencer(name, publicPort, adminPort, imageTag?)` | public=5001, admin=5002 |
| `.WithPublicPort(port)` | overrides default public API port |

Configures: `canton-sequencer:3.4.11` image, public/admin endpoints, shared-bootstrap mount,
log level, memory storage, health check on public endpoint.

### Mediator

```csharp
var mediator1 = builder.AddCantonMediator("mediator1");
// or with custom port:
builder.AddCantonMediator("mediator1", adminPort: 5302);
```

| Method | Default |
|--------|---------|
| `AddCantonMediator(name, adminPort, imageTag?)` | admin=5202 |

Configures: `canton-mediator:3.4.11` image, admin endpoint, shared-bootstrap mount, log level,
memory storage.

### Synchronizer

```csharp
var synchronizer = builder.AddCantonSynchronizer("synchronizer")
    .WithRemoteSequencer(sequencer1)
    .WithRemoteMediator(mediator1);
```

| Method | Notes |
|--------|-------|
| `AddCantonSynchronizer(name, imageTag?)` | Uses `canton-base:3.4.11` image |
| `.WithRemoteSequencer(sequencer, publicPort?, adminPort?)` | Injects HOCON env vars + `.WaitFor()` |
| `.WithRemoteMediator(mediator, adminPort?)` | Injects HOCON env vars + `.WaitFor()` |

Default ports: sequencer public=5001, sequencer admin=5002, mediator admin=5202.
Override when sequencer/mediator use non-default ports.

### Participant

```csharp
var participant1 = builder.AddCantonParticipant("participant1")
    .WithAdminPort(5012)
    .WithLedgerPort(5011)
    .WithHttpPort(5013)
    .WithBootstrapScript("./configs/participant1-bootstrap.sc")
    .WaitFor(synchronizer);
```

| Method | Notes |
|--------|-------|
| `AddCantonParticipant(name, imageTag?)` | Mounts shared-bootstrap + DARs dir, memory storage, no auth |
| `.WithAdminPort(port)` | admin endpoint + config env var |
| `.WithLedgerPort(port)` | ledger-api endpoint + config env var |
| `.WithHttpPort(port)` | http-json-api endpoint + config env var + health check |
| `.WithBootstrapScript(path)` | Bind-mounts per-participant bootstrap script |
| `.WithDarsDirectory(path)` | Overrides default DARs directory (`./dars`) |

### Shared defaults

| Default | Value |
|---------|-------|
| Docker registry | `europe-docker.pkg.dev/da-images/public/docker` |
| Canton version | `3.4.11` |
| Bootstrap path | `./configs/shared-bootstrap.sc` |
| Storage | `memory` (override via `.WithEnvironment()`) |

### OpenTelemetry tracing

```csharp
builder.AddCantonSequencer("sequencer1")
    .WithCantonOpenTelemetryTracing("host.docker.internal", 4317);
```

`WithCantonOpenTelemetryTracing` configures Canton OTLP tracing and adds the
Docker host alias needed for containers to reach the Aspire Dashboard OTLP/gRPC
endpoint. The platform AppHost wires this to the port printed by Aspire.

## Build

Requires .NET 10 SDK and the Aspire CLI.

```bash
# From the canton-bond-platform directory:
dotnet pack Canton.Aspire.Hosting/ -c Release -o LocalPackages

# Or with a specific version:
dotnet pack Canton.Aspire.Hosting/ -c Release -o LocalPackages /p:Version=1.0.10
```

The built `.nupkg` is placed in `LocalPackages/`, which is registered as a local NuGet source
in `nuget.config`. The AppHost references it via:

```
#:package Canton.Aspire.Hosting@1.0.10
```

## Run

```bash
dotnet pack Canton.Aspire.Hosting/ -c Release -o LocalPackages
./start-aspire.sh
```

Or use the Aspire CLI from the AppHost directory. The package is resolved
automatically from the local NuGet source.
