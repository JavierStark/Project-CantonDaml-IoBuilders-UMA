#:package Aspire.Hosting.Go@13.4.2-preview.1.26303.6
#:package Aspire.Hosting.JavaScript@13.4.2
#:package Canton.Aspire.Hosting@1.0.4
#:sdk Aspire.AppHost.Sdk@13.4.2

using System.Diagnostics;
using System.Net.Http;
using System.Text;
using System.Text.Json;
using Aspire.Hosting.ApplicationModel;
using Canton.Aspire.Hosting;
using Microsoft.Extensions.Logging;

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
    ;

foreach (var p in participants)
    backend = backend.WaitFor(p);

// ---- Factory initialization (blocks backend until factory contract exists) ----

const string FactoryTemplateId = "#simple-token:SimpleToken.Rules:SimpleTokenRules";

backend.WithAnnotation(new EnvironmentCallbackAnnotation(
    new Func<EnvironmentCallbackContext, Task>(async ctx =>
{
    var ct = ctx.CancellationToken;
    var log = ctx.Logger;

    try
    {
    using var http = new HttpClient { Timeout = TimeSpan.FromSeconds(15) };

    var p1Url = "http://localhost:5013";
    var allUrls = new[] { p1Url, "http://localhost:5023", "http://localhost:5033" };

    log?.LogInformation("Initializing factory contract on {Url}...", p1Url);

    // 1. Get ledger end offset
    var ledgerJson = await http.GetStringAsync($"{p1Url}/v2/state/ledger-end", ct);
    var offset = JsonDocument.Parse(ledgerJson).RootElement.GetProperty("offset").GetInt64();

    // 2. Check if factory already exists
    var filterPayload = JsonSerializer.Serialize(new
    {
        filter = new
        {
            filtersByParty = new { },
            filtersForAnyParty = new
            {
                cumulative = new[]
                {
                    new
                    {
                        identifierFilter = new
                        {
                            WildcardFilter = new
                            {
                                value = new { includeCreatedEventBlob = true }
                            }
                        }
                    }
                }
            }
        },
        activeAtOffset = offset,
        verbose = false
    }, new JsonSerializerOptions { PropertyNamingPolicy = null });

    var filterResp = await http.PostAsync($"{p1Url}/v2/state/active-contracts",
        new StringContent(filterPayload, Encoding.UTF8, "application/json"), ct);
    filterResp.EnsureSuccessStatusCode();
    var contractsBody = await filterResp.Content.ReadAsStringAsync(ct);

    if (contractsBody.Contains(FactoryTemplateId))
    {
        log?.LogInformation("Factory contract already exists, skipping creation");
        return;
    }

    // 3. Resolve admin party (retry up to 10x while participant bootstraps)
    string? adminParty = null;
    for (int i = 0; i < 10; i++)
    {
        try
        {
            var partiesJson = await http.GetStringAsync($"{p1Url}/v2/parties", ct);
            using var doc = JsonDocument.Parse(partiesJson);
            foreach (var pd in doc.RootElement.GetProperty("partyDetails").EnumerateArray())
            {
                var party = pd.GetProperty("party").GetString()!;
                var display = pd.TryGetProperty("displayName", out var dn)
                    ? dn.GetString() : null;
                if (party.StartsWith("admin::") || display == "admin")
                {
                    adminParty = party;
                    break;
                }
            }
        }
        catch (Exception ex) when (i < 9)
        {
            log?.LogWarning("Admin party lookup attempt {N}/10: {Msg}", i + 1, ex.Message);
        }
        if (adminParty != null) break;
        await Task.Delay(3000, ct);
    }

    if (adminParty == null)
        throw new Exception("Admin party not resolved on participant1 after 10 retries — bootstrap may not be complete");

    log?.LogInformation("Admin party resolved: {Party}", adminParty);

    // 4. Collect observer parties from all participants
    var observers = new List<string>();
    foreach (var url in allUrls)
    {
        try
        {
            var partiesJson = await http.GetStringAsync($"{url}/v2/parties", ct);
            using var doc = JsonDocument.Parse(partiesJson);
            foreach (var pd in doc.RootElement.GetProperty("partyDetails").EnumerateArray())
            {
                var party = pd.GetProperty("party").GetString()!;
                if (party != adminParty && !observers.Contains(party))
                    observers.Add(party);
            }
        }
        catch (Exception ex)
        {
            log?.LogWarning("Could not list parties from {Url}: {Msg}", url, ex.Message);
        }
    }

    // 5. Create the factory contract
    var createPayload = JsonSerializer.Serialize(new
    {
        commands = new[]
        {
            new
            {
                CreateCommand = new
                {
                    templateId = FactoryTemplateId,
                    createArguments = new
                    {
                        admin = adminParty,
                        supportedInstruments = new[] { "BOND" },
                        observers = observers.ToArray()
                    }
                }
            }
        },
        userId = "ledger-api-user",
        commandId = "factory-init-" + Guid.NewGuid().ToString("N")[..8],
        actAs = new[] { adminParty },
        readAs = new[] { adminParty }
    }, new JsonSerializerOptions { PropertyNamingPolicy = null });

    log?.LogInformation("Creating factory contract...");

    var createResp = await http.PostAsync($"{p1Url}/v2/commands/submit-and-wait",
        new StringContent(createPayload, Encoding.UTF8, "application/json"), ct);
    createResp.EnsureSuccessStatusCode();

    log?.LogInformation("Factory contract created successfully");
    }
    catch (Exception ex)
    {
        log?.LogError(ex, "Factory initialization failed");
        throw;
    }
})));

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
