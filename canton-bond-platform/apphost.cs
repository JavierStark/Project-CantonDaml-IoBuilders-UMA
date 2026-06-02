#:package Aspire.Hosting.Go@13.4.0-preview.1.26281.18
#:package Aspire.Hosting.JavaScript@13.4.0
#:package Canton.Aspire.Hosting@1.0.4
#:sdk Aspire.AppHost.Sdk@13.4.0

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

var sequencer1 = builder.AddCantonSequencer("sequencer1");
var mediator1 = builder.AddCantonMediator("mediator1");
var synchronizer = builder.AddCantonSynchronizer("synchronizer")
    .WithRemoteSequencer(sequencer1)
    .WithRemoteMediator(mediator1);

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
        .WaitFor(synchronizer);

    participants.Add(participant);
}

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
    ;

foreach (var p in participants)
    backend = backend.WaitFor(p);

// ---- Frontend (Vite + YARP static website) ----

#pragma warning disable ASPIREJAVASCRIPT001

var frontend = builder.AddViteApp("frontend", "./frontend");
frontend.PublishAsStaticWebsite(
    apiPath: "/api",
    apiTarget: backend)
    .WithExternalHttpEndpoints()
    .WaitFor(backend);

builder.Build().Run();
