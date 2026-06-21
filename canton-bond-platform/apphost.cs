#:package Aspire.Hosting.Go@13.4.2-preview.1.26303.6
#:package Aspire.Hosting.JavaScript@13.4.2
#:package Canton.Aspire.Hosting@1.0.6
#:sdk Aspire.AppHost.Sdk@13.4.2

using System.Diagnostics;
using Aspire.Hosting.ApplicationModel;
using Canton.Aspire.Hosting;

var builder = DistributedApplication.CreateBuilder(args);

builder.Eventing.Subscribe<BeforeStartEvent>(async (e, ct) =>
{
    var darFile = Path.Combine(Directory.GetCurrentDirectory(), "dars", "simple-token-0.1.0.dar");
    if (File.Exists(darFile))
    {
        Console.WriteLine("[OK] DAR already exists at dars/simple-token-0.1.0.dar");
        return;
    }

    Console.WriteLine("[INFO] Building bond contract DAR...");
    var psi = new ProcessStartInfo
    {
        WorkingDirectory = Path.Combine(Directory.GetCurrentDirectory(), "bond-contract")
    };

    if (IsCommandAvailable("dpm"))
    {
        psi.FileName = "dpm";
        psi.Arguments = "build";
    }
    else if (IsCommandAvailable("daml"))
    {
        psi.FileName = "daml";
        psi.Arguments = "build";
    }
    else
    {
        Console.WriteLine("[WARN] Neither 'dpm' nor 'daml' found. DAR must exist at dars/simple-token-0.1.0.dar");
        return;
    }

    var proc = Process.Start(psi);
    if (proc == null)
        throw new Exception("Failed to start DAR build process");

    await proc.WaitForExitAsync(ct);

    if (proc.ExitCode != 0)
        throw new Exception("DAR build failed");

    var distDir = Path.Combine(Directory.GetCurrentDirectory(), "bond-contract", ".daml", "dist");
    var built = Directory.GetFiles(distDir, "simple-token-*.dar").FirstOrDefault();
    if (built != null)
    {
        Directory.CreateDirectory(Path.Combine(Directory.GetCurrentDirectory(), "dars"));
        File.Copy(built, darFile, overwrite: true);
        Console.WriteLine("[OK] DAR built and copied to dars/");
    }
});

static bool IsCommandAvailable(string cmd)
{
    try
    {
        var pi = new ProcessStartInfo("which", cmd)
        {
            RedirectStandardOutput = true,
            RedirectStandardError = true
        };
        var proc = Process.Start(pi);
        if (proc == null)
            return false;
        proc.WaitForExit(3000);
        return proc.ExitCode == 0;
    }
    catch
    {
        return false;
    }
}

// ---- Canton Infrastructure (using typed integration) ----

var otelCollector = builder.AddContainer("otel-collector", "otel/opentelemetry-collector-contrib:0.128.0")
    .WithArgs("--config=/etc/otelcol-contrib/config.yaml")
    .WithBindMount("./configs/otel-collector.yaml", "/etc/otelcol-contrib/config.yaml", isReadOnly: true)
    .WithEndpoint("otlp-grpc", e => { e.Port = 4317; e.TargetPort = 4317; })
    .WithEndpoint("otlp-http", e => { e.Port = 4318; e.TargetPort = 4318; })
    .WithEndpoint("health", e => { e.Port = 13133; e.TargetPort = 13133; })
    .WithEndpoint("zpages", e => { e.Port = 55679; e.TargetPort = 55679; });

static IResourceBuilder<ContainerResource> WithCantonOtelTracing(IResourceBuilder<ContainerResource> resource)
{
    return resource
        .WithEnvironment("ADDITIONAL_CONFIG_90", "canton.monitoring.tracing.propagation = enabled")
        .WithEnvironment("ADDITIONAL_CONFIG_91", "canton.monitoring.tracing.tracer.exporter.type = otlp")
        .WithEnvironment("ADDITIONAL_CONFIG_92", "canton.monitoring.tracing.tracer.exporter.address = \"otel-collector\"")
        .WithEnvironment("ADDITIONAL_CONFIG_93", "canton.monitoring.tracing.tracer.exporter.port = 4317")
        .WithEnvironment("ADDITIONAL_CONFIG_94", "canton.monitoring.tracing.tracer.sampler.type = trace-id-ratio")
        .WithEnvironment("ADDITIONAL_CONFIG_95", "canton.monitoring.tracing.tracer.sampler.ratio = 0.1")
        .WithEnvironment("ADDITIONAL_CONFIG_96", "canton.monitoring.tracing.tracer.sampler.parent-based = true");
}

var sequencer1 = WithCantonOtelTracing(builder.AddCantonSequencer("sequencer1"))
    .WaitFor(otelCollector);
var mediator1 = WithCantonOtelTracing(builder.AddCantonMediator("mediator1"))
    .WaitFor(otelCollector);
var synchronizer = builder.AddCantonSynchronizer("synchronizer")
    .WithRemoteSequencer(sequencer1)
    .WithRemoteMediator(mediator1);
synchronizer = WithCantonOtelTracing(synchronizer)
    .WaitFor(otelCollector);

var participantConfigs = new[]
{
    new { Name = "participant1", AdminPort = 5012, LedgerPort = 5011, HttpPort = 5013, Bootstrap = "./configs/participant1-bootstrap.sc", Parties = "admin,alice,executor" },
    new { Name = "participant2", AdminPort = 5022, LedgerPort = 5021, HttpPort = 5023, Bootstrap = "./configs/participant2-bootstrap.sc", Parties = "bob" },
    new { Name = "participant3", AdminPort = 5032, LedgerPort = 5031, HttpPort = 5033, Bootstrap = "./configs/participant3-bootstrap.sc", Parties = "charlie" },
};

var participants = new List<IResourceBuilder<ContainerResource>>();

foreach (var cfg in participantConfigs)
{
    var participant = builder.AddCantonParticipant(cfg.Name)
        .WithAdminPort(cfg.AdminPort)
        .WithLedgerPort(cfg.LedgerPort)
        .WithHttpPort(cfg.HttpPort)
        .WithBootstrapScript(cfg.Bootstrap)
        .WithEnvironment("ADDITIONAL_CONFIG_90", "canton.monitoring.tracing.propagation = enabled")
        .WithEnvironment("ADDITIONAL_CONFIG_91", "canton.monitoring.tracing.tracer.exporter.type = otlp")
        .WithEnvironment("ADDITIONAL_CONFIG_92", "canton.monitoring.tracing.tracer.exporter.address = \"otel-collector\"")
        .WithEnvironment("ADDITIONAL_CONFIG_93", "canton.monitoring.tracing.tracer.exporter.port = 4317")
        .WithEnvironment("ADDITIONAL_CONFIG_94", "canton.monitoring.tracing.tracer.sampler.type = trace-id-ratio")
        .WithEnvironment("ADDITIONAL_CONFIG_95", "canton.monitoring.tracing.tracer.sampler.ratio = 0.1")
        .WithEnvironment("ADDITIONAL_CONFIG_96", "canton.monitoring.tracing.tracer.sampler.parent-based = true")
        .WaitFor(otelCollector)
        .WaitFor(synchronizer);

    participants.Add(participant);
}

// ---- Backend (Go) ----

var backend = builder.AddGoApp("backend", "./backend")
    .WithHttpEndpoint(env: "HTTP_PORT")
    .WithEnvironment("HTTP_HOST", "0.0.0.0")
    .WithEnvironment("CANTON_USER_ID", "ledger-api-user")
    .WithEnvironment("REQUEST_TIMEOUT", "30s")
    .WithEnvironment("LEDGER_TRANSPORT", "grpc")
    .WithEnvironment("PARTICIPANT1_URL", "http://localhost:5013")
    .WithEnvironment("PARTICIPANT1_GRPC_URL", "localhost:5011")
    .WithEnvironment("PARTICIPANT1_PARTIES", "admin,alice,executor")
    .WithEnvironment("PARTICIPANT2_URL", "http://localhost:5023")
    .WithEnvironment("PARTICIPANT2_GRPC_URL", "localhost:5021")
    .WithEnvironment("PARTICIPANT2_PARTIES", "bob")
    .WithEnvironment("PARTICIPANT3_URL", "http://localhost:5033")
    .WithEnvironment("PARTICIPANT3_GRPC_URL", "localhost:5031")
    .WithEnvironment("PARTICIPANT3_PARTIES", "charlie")
    .WithEnvironment("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317")
    .WithEnvironment("OTEL_EXPORTER_OTLP_INSECURE", "true")
    .WithEnvironment("OTEL_SERVICE_NAME", "backend")
    .WaitFor(otelCollector)
    ;

foreach (var p in participants)
    backend = backend.WaitFor(p);

// ---- Factory initialization (custom resource — runs once after participants are ready) ----

var factoryInit = builder.AddFactoryInit("factory-init");
foreach (var p in participants)
    factoryInit = factoryInit.WaitFor(p);

backend = backend.WaitFor(factoryInit);

// ---- Frontend (Vite + YARP static website) ----

#pragma warning disable ASPIREJAVASCRIPT001

var frontend = builder.AddViteApp("frontend", "./frontend");
frontend.PublishAsStaticWebsite(
    apiPath: "/api",
    apiTarget: backend)
    .WithExternalHttpEndpoints()
    .WaitFor(backend);

// ---- Listener (Go — real-time gRPC event stream → WebSocket) ----

var listener = builder.AddGoApp("listener", "./listener")
    .WithHttpEndpoint(env: "LISTENER_WS_PORT")
    .WithEnvironment("LISTENER_PARTICIPANT_URL", "localhost:5011")
    .WithEnvironment("LISTENER_HTTP_URL", "http://localhost:5013")
    .WithEnvironment("LISTENER_PARTY", "admin");

foreach (var p in participants)
    listener = listener.WaitFor(p);

builder.Build().Run();
