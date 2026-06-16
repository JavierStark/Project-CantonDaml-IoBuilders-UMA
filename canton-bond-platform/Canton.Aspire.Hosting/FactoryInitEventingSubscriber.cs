using System.Net.Http;
using System.Text;
using System.Text.Json;
using Aspire.Hosting;
using Aspire.Hosting.ApplicationModel;
using Aspire.Hosting.Eventing;
using Aspire.Hosting.Lifecycle;
using Microsoft.Extensions.Logging;

namespace Canton.Aspire.Hosting;

internal sealed class FactoryInitEventingSubscriber(
    ResourceNotificationService notification,
    ResourceLoggerService loggerService)
    : IDistributedApplicationEventingSubscriber
{
    private const string FactoryTemplateId = "#simple-token:SimpleToken.Rules:SimpleTokenRules";

    public Task SubscribeAsync(
        IDistributedApplicationEventing eventing,
        DistributedApplicationExecutionContext context,
        CancellationToken cancellationToken)
    {
        eventing.Subscribe<AfterResourcesCreatedEvent>(async (@event, ct) =>
        {
            foreach (var resource in @event.Model.Resources.OfType<FactoryInitResource>())
            {
                var logger = loggerService.GetLogger(resource);

                await notification.PublishUpdateAsync(resource, s => s with
                {
                    State = KnownResourceStates.Starting,
                    Properties = [new("status", "Waiting for participants and initializing factory...")]
                });

                try
                {
                    using var http = new HttpClient { Timeout = TimeSpan.FromSeconds(15) };

                    var p1Url = "http://localhost:5013";
                    var allUrls = new[] { p1Url, "http://localhost:5023", "http://localhost:5033" };
                    var jsonOpts = new JsonSerializerOptions { PropertyNamingPolicy = null };

                    logger?.LogInformation("Initializing factory contract on {Url}...", p1Url);

                    // 1-2. Get ledger end and check if factory exists (retry up to 20x, 3s = 60s)
                    long offset = 0;
                    bool factoryExists = false;

                    await RetryAsync(ct, 20, 3000, async token =>
                    {
                        var ledgerJson = await http.GetStringAsync($"{p1Url}/v2/state/ledger-end", token);
                        offset = JsonDocument.Parse(ledgerJson).RootElement.GetProperty("offset").GetInt64();

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
                        }, jsonOpts);

                        var filterResp = await http.PostAsync($"{p1Url}/v2/state/active-contracts",
                            new StringContent(filterPayload, Encoding.UTF8, "application/json"), token);
                        filterResp.EnsureSuccessStatusCode();
                        var contractsBody = await filterResp.Content.ReadAsStringAsync(token);

                        factoryExists = contractsBody.Contains(FactoryTemplateId);
                    }, logger, "Querying ledger-end and checking existing factory");

                    if (factoryExists)
                    {
                        logger?.LogInformation("Factory contract already exists, skipping creation");

                        await notification.PublishUpdateAsync(resource, s => s with
                        {
                            State = KnownResourceStates.Running,
                            Properties = [new("status", "Factory contract already exists")]
                        });
                        return;
                    }

                    // 3. Resolve admin party (retry up to 10x, 3s = 30s)
                    string? adminParty = null;

                    await RetryAsync(ct, 10, 3000, async token =>
                    {
                        var partiesJson = await http.GetStringAsync($"{p1Url}/v2/parties", token);
                        using var doc = JsonDocument.Parse(partiesJson);
                        foreach (var pd in doc.RootElement.GetProperty("partyDetails").EnumerateArray())
                        {
                            var party = pd.GetProperty("party").GetString()!;
                            var display = pd.TryGetProperty("displayName", out var dn)
                                ? dn.GetString() : null;
                            if (party.StartsWith("admin::") || display == "admin")
                            {
                                adminParty = party;
                                return;
                            }
                        }
                        throw new Exception("Admin party not found on participant1");
                    }, logger, "Resolving admin party");

                    logger?.LogInformation("Admin party resolved: {Party}", adminParty);

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
                            logger?.LogWarning("Could not list parties from {Url}: {Msg}", url, ex.Message);
                        }
                    }

                    // 5. Create the factory contract (retry up to 5x, 3s)
                    await RetryAsync(ct, 5, 3000, async token =>
                    {
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
                        }, jsonOpts);

                        logger?.LogInformation("Creating factory contract...");

                        var createResp = await http.PostAsync($"{p1Url}/v2/commands/submit-and-wait",
                            new StringContent(createPayload, Encoding.UTF8, "application/json"), token);
                        createResp.EnsureSuccessStatusCode();

                        logger?.LogInformation("Factory contract created successfully");
                    }, logger, "Creating factory contract");

                    await notification.PublishUpdateAsync(resource, s => s with
                    {
                        State = KnownResourceStates.Running,
                        Properties = [new("status", "Factory contract initialized")]
                    });
                }
                catch (Exception ex)
                {
                    logger?.LogError(ex, "Factory initialization failed");

                    await notification.PublishUpdateAsync(resource, s => s with
                    {
                        State = KnownResourceStates.FailedToStart,
                        Properties = [new("error", ex.Message)]
                    });
                    throw;
                }
            }
        });

        return Task.CompletedTask;
    }

    private static async Task RetryAsync(
        CancellationToken ct,
        int maxRetries,
        int delayMs,
        Func<CancellationToken, Task> operation,
        ILogger? logger,
        string description)
    {
        for (int i = 0; i < maxRetries; i++)
        {
            try
            {
                await operation(ct);
                return;
            }
            catch (Exception ex) when (i < maxRetries - 1)
            {
                logger?.LogWarning("{Description} attempt {N}/{Max}: {Msg}",
                    description, i + 1, maxRetries, ex.Message);
                await Task.Delay(delayMs, ct);
            }
        }

        await operation(ct);
    }
}
