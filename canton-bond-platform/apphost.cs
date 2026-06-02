#:package Aspire.Hosting.Go@13.4.0-preview.1.26281.18
#:package Aspire.Hosting.JavaScript@13.4.0
#:sdk Aspire.AppHost.Sdk@13.4.0

using System.Diagnostics;
using Aspire.Hosting.ApplicationModel;

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

// ---- Canton Infrastructure ----

var sequencer1 = builder.AddContainer("sequencer1",
        "europe-docker.pkg.dev/da-images/public/docker/canton-sequencer:3.4.11")
    .WithEndpoint("public", e => { e.Port = 5001; e.TargetPort = 5001; })
    .WithEndpoint("admin", e => { e.Port = 5002; e.TargetPort = 5002; })
    .WithBindMount("./configs/sequencer-bootstrap.sc", "/app/bootstrap.sc", isReadOnly: true)
    .WithBindMount("./configs/shared-bootstrap.sc", "/app/shared-bootstrap.sc", isReadOnly: true)
    .WithEnvironment("LOG_LEVEL_STDOUT", "INFO")
    .WithEnvironment("ADDITIONAL_CONFIG_01", "canton.sequencers.sequencer.public-api.port = 5001")
    .WithEnvironment("ADDITIONAL_CONFIG_02", "canton.sequencers.sequencer.admin-api.port = 5002")
    .WithEnvironment("ADDITIONAL_CONFIG_03", "canton.sequencers.sequencer.storage.type = memory")
    .WithEnvironment("ADDITIONAL_CONFIG_04", "canton.sequencers.sequencer.sequencer.config.storage.type = memory");

var mediator1 = builder.AddContainer("mediator1",
        "europe-docker.pkg.dev/da-images/public/docker/canton-mediator:3.4.11")
    .WithEndpoint("admin", e => { e.Port = 5202; e.TargetPort = 5202; })
    .WithBindMount("./configs/mediator-bootstrap.sc", "/app/bootstrap.sc", isReadOnly: true)
    .WithBindMount("./configs/shared-bootstrap.sc", "/app/shared-bootstrap.sc", isReadOnly: true)
    .WithEnvironment("LOG_LEVEL_STDOUT", "INFO")
    .WithEnvironment("ADDITIONAL_CONFIG_01", "canton.mediators.mediator.admin-api.port = 5202")
    .WithEnvironment("ADDITIONAL_CONFIG_02", "canton.mediators.mediator.storage.type = memory");

var synchronizer = builder.AddContainer("synchronizer",
        "europe-docker.pkg.dev/da-images/public/docker/canton-base:3.4.11")
    .WithBindMount("./configs/synchronizer-bootstrap.sc", "/app/bootstrap.sc", isReadOnly: true)
    .WithBindMount("./configs/shared-bootstrap.sc", "/app/shared-bootstrap.sc", isReadOnly: true)
    .WithEnvironment("LOG_LEVEL_STDOUT", "INFO")
    .WithEnvironment("ADDITIONAL_CONFIG_01", "canton.remote-sequencers.sequencer1.public-api.port = 5001")
    .WithEnvironment("ADDITIONAL_CONFIG_02", "canton.remote-sequencers.sequencer1.public-api.address = sequencer1")
    .WithEnvironment("ADDITIONAL_CONFIG_03", "canton.remote-sequencers.sequencer1.admin-api.port = 5002")
    .WithEnvironment("ADDITIONAL_CONFIG_04", "canton.remote-sequencers.sequencer1.admin-api.address = sequencer1")
    .WithEnvironment("ADDITIONAL_CONFIG_05", "canton.remote-mediators.mediator1.admin-api.port = 5202")
    .WithEnvironment("ADDITIONAL_CONFIG_06", "canton.remote-mediators.mediator1.admin-api.address = mediator1")
    .WaitFor(sequencer1)
    .WaitFor(mediator1);

var participant1 = builder.AddContainer("participant1",
        "europe-docker.pkg.dev/da-images/public/docker/canton-participant:3.4.11")
    .WithEndpoint("admin", e => { e.Port = 5012; e.TargetPort = 5012; })
    .WithEndpoint("ledger-api", e => { e.Port = 5011; e.TargetPort = 5011; })
    .WithEndpoint("http-json-api", e => { e.Port = 5013; e.TargetPort = 5013; })
    .WithBindMount("./configs/participant1-bootstrap.sc", "/app/bootstrap.sc", isReadOnly: true)
    .WithBindMount("./configs/shared-bootstrap.sc", "/app/shared-bootstrap.sc", isReadOnly: true)
    .WithBindMount("./dars", "/app/dars", isReadOnly: true)
    .WithEnvironment("LOG_LEVEL_STDOUT", "INFO")
    .WithEnvironment("ADDITIONAL_CONFIG_01", "canton.participants.participant.admin-api.port = 5012")
    .WithEnvironment("ADDITIONAL_CONFIG_02", "canton.participants.participant.ledger-api.port = 5011")
    .WithEnvironment("ADDITIONAL_CONFIG_03", "canton.participants.participant.http-ledger-api.port = 5013")
    .WithEnvironment("ADDITIONAL_CONFIG_04", "canton.participants.participant.ledger-api.auth-services = []")
    .WithEnvironment("ADDITIONAL_CONFIG_05", "canton.participants.participant.storage.type = memory")
    .WaitFor(synchronizer);

var participant2 = builder.AddContainer("participant2",
        "europe-docker.pkg.dev/da-images/public/docker/canton-participant:3.4.11")
    .WithEndpoint("admin", e => { e.Port = 5022; e.TargetPort = 5022; })
    .WithEndpoint("ledger-api", e => { e.Port = 5021; e.TargetPort = 5021; })
    .WithEndpoint("http-json-api", e => { e.Port = 5023; e.TargetPort = 5023; })
    .WithBindMount("./configs/participant2-bootstrap.sc", "/app/bootstrap.sc", isReadOnly: true)
    .WithBindMount("./configs/shared-bootstrap.sc", "/app/shared-bootstrap.sc", isReadOnly: true)
    .WithBindMount("./dars", "/app/dars", isReadOnly: true)
    .WithEnvironment("LOG_LEVEL_STDOUT", "INFO")
    .WithEnvironment("ADDITIONAL_CONFIG_01", "canton.participants.participant.admin-api.port = 5022")
    .WithEnvironment("ADDITIONAL_CONFIG_02", "canton.participants.participant.ledger-api.port = 5021")
    .WithEnvironment("ADDITIONAL_CONFIG_03", "canton.participants.participant.http-ledger-api.port = 5023")
    .WithEnvironment("ADDITIONAL_CONFIG_04", "canton.participants.participant.ledger-api.auth-services = []")
    .WithEnvironment("ADDITIONAL_CONFIG_05", "canton.participants.participant.storage.type = memory")
    .WaitFor(synchronizer);

var participant3 = builder.AddContainer("participant3",
        "europe-docker.pkg.dev/da-images/public/docker/canton-participant:3.4.11")
    .WithEndpoint("admin", e => { e.Port = 5032; e.TargetPort = 5032; })
    .WithEndpoint("ledger-api", e => { e.Port = 5031; e.TargetPort = 5031; })
    .WithEndpoint("http-json-api", e => { e.Port = 5033; e.TargetPort = 5033; })
    .WithBindMount("./configs/participant3-bootstrap.sc", "/app/bootstrap.sc", isReadOnly: true)
    .WithBindMount("./configs/shared-bootstrap.sc", "/app/shared-bootstrap.sc", isReadOnly: true)
    .WithBindMount("./dars", "/app/dars", isReadOnly: true)
    .WithEnvironment("LOG_LEVEL_STDOUT", "INFO")
    .WithEnvironment("ADDITIONAL_CONFIG_01", "canton.participants.participant.admin-api.port = 5032")
    .WithEnvironment("ADDITIONAL_CONFIG_02", "canton.participants.participant.ledger-api.port = 5031")
    .WithEnvironment("ADDITIONAL_CONFIG_03", "canton.participants.participant.http-ledger-api.port = 5033")
    .WithEnvironment("ADDITIONAL_CONFIG_04", "canton.participants.participant.ledger-api.auth-services = []")
    .WithEnvironment("ADDITIONAL_CONFIG_05", "canton.participants.participant.storage.type = memory")
    .WaitFor(synchronizer);

// ---- Backend (Go) ----

var backend = builder.AddGoApp("backend", "./backend")
    .WithHttpEndpoint(env: "HTTP_PORT")
    .WithEnvironment("HTTP_HOST", "0.0.0.0")
    .WithEnvironment("CANTON_USER_ID", "ledger-api-user")
    .WithEnvironment("REQUEST_TIMEOUT", "30s")
    .WithEnvironment("PARTICIPANT1_URL", "http://localhost:5013")
    .WithEnvironment("PARTICIPANT1_PARTIES", "admin,alice,executor")
    .WithEnvironment("PARTICIPANT2_URL", "http://localhost:5023")
    .WithEnvironment("PARTICIPANT2_PARTIES", "bob")
    .WithEnvironment("PARTICIPANT3_URL", "http://localhost:5033")
    .WithEnvironment("PARTICIPANT3_PARTIES", "charlie")
    .WaitFor(participant1)
    .WaitFor(participant2)
    .WaitFor(participant3);

// ---- Frontend (Vite + YARP static website) ----

#pragma warning disable ASPIREJAVASCRIPT001

var frontend = builder.AddViteApp("frontend", "./frontend");
frontend.PublishAsStaticWebsite(
    apiPath: "/api",
    apiTarget: backend)
    .WithExternalHttpEndpoints()
    .WaitFor(backend);

builder.Build().Run();
