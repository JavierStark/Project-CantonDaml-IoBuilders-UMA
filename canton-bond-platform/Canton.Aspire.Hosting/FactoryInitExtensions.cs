using Aspire.Hosting;
using Aspire.Hosting.ApplicationModel;
using Aspire.Hosting.Eventing;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;

namespace Canton.Aspire.Hosting;

public static class FactoryInitExtensions
{
    public static IResourceBuilder<FactoryInitResource> AddFactoryInit(
        this IDistributedApplicationBuilder builder,
        string name)
    {
        var resource = new FactoryInitResource(name);
        var result = builder.AddResource(resource)
            .WithInitialState(new CustomResourceSnapshot
            {
                ResourceType = "FactoryInit",
                State = KnownResourceStates.Waiting,
                Properties = [
                    new(CustomResourceKnownProperties.Source, "Canton SimpleTokenRules factory contract init")
                ]
            })
            .ExcludeFromManifest();

        builder.Eventing.Subscribe<BeforeStartEvent>(async (@event, ct) =>
        {
            foreach (var factoryResource in @event.Model.Resources.OfType<FactoryInitResource>())
            {
                var loggerFactory = @event.Services.GetRequiredService<ILoggerFactory>();
                var logger = loggerFactory.CreateLogger("Canton.Aspire.Hosting.FactoryInit");

                logger.LogInformation("Processing {ResourceName}", factoryResource.Name);

                var notification = @event.Services.GetRequiredService<ResourceNotificationService>();

                try
                {
                    await notification.WaitForDependenciesAsync(factoryResource, ct);

                    await notification.PublishUpdateAsync(factoryResource, s => s with
                    {
                        State = KnownResourceStates.Starting,
                        Properties = [new("status", "Initializing factory...")]
                    });

                    logger.LogInformation("Starting init logic...");
                    using var http = new HttpClient { Timeout = TimeSpan.FromSeconds(15) };
                    var jsonOpts = new System.Text.Json.JsonSerializerOptions { PropertyNamingPolicy = null };

                    // Derive participant HTTP JSON API URLs from endpoint annotations
                    var participantUrls = new List<string>();
                    foreach (var r in @event.Model.Resources)
                    {
                        var ep = r.Annotations.OfType<EndpointAnnotation>()
                            .FirstOrDefault(e => e.Name == "http-json-api");
                        if (ep?.Port != null)
                            participantUrls.Add($"http://localhost:{ep.Port}");
                    }

                    // Fall back to defaults if no participant endpoints found
                    if (participantUrls.Count == 0)
                    {
                        participantUrls = ["http://localhost:5013", "http://localhost:5023", "http://localhost:5033"];
                        logger.LogWarning("No participant endpoints found via annotations, using default ports");
                    }

                    var p1Url = participantUrls[0];

                    long offset = 0;
                    bool factoryExists = false;
                    const string FactoryTemplateId = "#simple-token:SimpleToken.Rules:SimpleTokenRules";

                    await RetryAsync(ct, logger, 20, 3000, async token =>
                    {
                        var ledgerJson = await http.GetStringAsync($"{p1Url}/v2/state/ledger-end", token);
                        offset = System.Text.Json.JsonDocument.Parse(ledgerJson).RootElement.GetProperty("offset").GetInt64();

                        var filterPayload = System.Text.Json.JsonSerializer.Serialize(new
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
                            new StringContent(filterPayload, System.Text.Encoding.UTF8, "application/json"), token);
                        filterResp.EnsureSuccessStatusCode();
                        var contractsBody = await filterResp.Content.ReadAsStringAsync(token);
                        factoryExists = contractsBody.Contains(FactoryTemplateId);
                    }, "Querying ledger-end and checking existing factory");

                    if (factoryExists)
                    {
                        logger.LogInformation("Factory contract already exists, skipping creation");
                        await notification.PublishUpdateAsync(factoryResource, s => s with
                        {
                            State = KnownResourceStates.Running,
                            Properties = [new("status", "Factory contract already exists")]
                        });
                        return;
                    }

                    string? adminParty = null;
                    await RetryAsync(ct, logger, 10, 3000, async token =>
                    {
                        var partiesJson = await http.GetStringAsync($"{p1Url}/v2/parties", token);
                        using var doc = System.Text.Json.JsonDocument.Parse(partiesJson);
                        foreach (var pd in doc.RootElement.GetProperty("partyDetails").EnumerateArray())
                        {
                            var party = pd.GetProperty("party").GetString()!;
                            var display = pd.TryGetProperty("displayName", out var dn) ? dn.GetString() : null;
                            if (party.StartsWith("admin::") || display == "admin")
                            {
                                adminParty = party;
                                return;
                            }
                        }
                        throw new Exception("Admin party not found on participant1");
                    }, "Resolving admin party");

                    logger.LogInformation("Admin party resolved: {AdminParty}", adminParty);

                    var observers = new List<string>();
                    await RetryAsync(ct, logger, 5, 3000, async token =>
                    {
                        observers.Clear();
                        foreach (var url in participantUrls)
                        {
                            try
                            {
                                var partiesJson = await http.GetStringAsync($"{url}/v2/parties", token);
                                using var doc = System.Text.Json.JsonDocument.Parse(partiesJson);
                                foreach (var pd in doc.RootElement.GetProperty("partyDetails").EnumerateArray())
                                {
                                    var party = pd.GetProperty("party").GetString()!;
                                    if (party != adminParty && !observers.Contains(party))
                                        observers.Add(party);
                                }
                            }
                            catch (Exception ex)
                            {
                                logger.LogWarning(ex, "Could not list parties from {Url}", url);
                            }
                        }
                        if (observers.Count == 0)
                            throw new Exception("No observer parties found on any participant");
                    }, "Collecting observer parties");

                    await RetryAsync(ct, logger, 5, 3000, async token =>
                    {
                        var createPayload = System.Text.Json.JsonSerializer.Serialize(new
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

                        logger.LogInformation("Creating factory contract...");
                        var createResp = await http.PostAsync($"{p1Url}/v2/commands/submit-and-wait",
                            new StringContent(createPayload, System.Text.Encoding.UTF8, "application/json"), token);
                        createResp.EnsureSuccessStatusCode();
                        logger.LogInformation("Factory contract created successfully");
                    }, "Creating factory contract");

                    await notification.PublishUpdateAsync(factoryResource, s => s with
                    {
                        State = KnownResourceStates.Running,
                        Properties = [new("status", "Factory contract initialized")]
                    });
                }
                catch (Exception ex)
                {
                    logger.LogError(ex, "Factory initialization failed for {ResourceName}", factoryResource.Name);
                    try
                    {
                        await notification.PublishUpdateAsync(factoryResource, s => s with
                        {
                            State = KnownResourceStates.FailedToStart,
                            Properties = [new("error", ex.Message)]
                        });
                    }
                    catch { }
                }
            }
        });

        return result;
    }

    private static async Task RetryAsync(
        CancellationToken ct,
        ILogger logger,
        int maxRetries,
        int delayMs,
        Func<CancellationToken, Task> operation,
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
                logger.LogWarning(ex, "{Description} attempt {Attempt}/{MaxRetries} failed",
                    description, i + 1, maxRetries);
                await Task.Delay(delayMs, ct);
            }
        }
    }
}
