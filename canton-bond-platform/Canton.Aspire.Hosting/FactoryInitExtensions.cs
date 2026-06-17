using Aspire.Hosting;
using Aspire.Hosting.ApplicationModel;
using Aspire.Hosting.Eventing;
using Microsoft.Extensions.DependencyInjection;


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
                Console.WriteLine($"[FactoryInitExtensions] Processing {factoryResource.Name}");

                var notification = @event.Services.GetRequiredService<ResourceNotificationService>();

                _ = Task.Run(async () =>
                {
                    try
                    {
                        await notification.WaitForDependenciesAsync(factoryResource, ct);

                        await notification.PublishUpdateAsync(factoryResource, s => s with
                        {
                            State = KnownResourceStates.Starting,
                            Properties = [new("status", "Initializing factory...")]
                        });

                        Console.WriteLine("[FactoryInitExtensions] Starting init logic...");
                        using var http = new System.Net.Http.HttpClient { Timeout = TimeSpan.FromSeconds(15) };

                        var p1Url = "http://localhost:5013";
                        var allUrls = new[] { p1Url, "http://localhost:5023", "http://localhost:5033" };
                        var jsonOpts = new System.Text.Json.JsonSerializerOptions { PropertyNamingPolicy = null };

                        long offset = 0;
                        bool factoryExists = false;
                        const string FactoryTemplateId = "#simple-token:SimpleToken.Rules:SimpleTokenRules";

                        await RetryAsync(ct, 20, 3000, async token =>
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
                                new System.Net.Http.StringContent(filterPayload, System.Text.Encoding.UTF8, "application/json"), token);
                            filterResp.EnsureSuccessStatusCode();
                            var contractsBody = await filterResp.Content.ReadAsStringAsync(token);
                            factoryExists = contractsBody.Contains(FactoryTemplateId);
                        }, "Querying ledger-end and checking existing factory");

                        if (factoryExists)
                        {
                            Console.WriteLine("[FactoryInitExtensions] Factory contract already exists, skipping creation");
                            await notification.PublishUpdateAsync(factoryResource, s => s with
                            {
                                State = KnownResourceStates.Running,
                                Properties = [new("status", "Factory contract already exists")]
                            });
                            return;
                        }

                        string? adminParty = null;
                        await RetryAsync(ct, 10, 3000, async token =>
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

                        Console.WriteLine($"[FactoryInitExtensions] Admin party resolved: {adminParty}");

                        var observers = new List<string>();
                        foreach (var url in allUrls)
                        {
                            try
                            {
                                var partiesJson = await http.GetStringAsync($"{url}/v2/parties", ct);
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
                                Console.WriteLine($"[FactoryInitExtensions] Could not list parties from {url}: {ex.Message}");
                            }
                        }

                        await RetryAsync(ct, 5, 3000, async token =>
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

                            Console.WriteLine("[FactoryInitExtensions] Creating factory contract...");
                            var createResp = await http.PostAsync($"{p1Url}/v2/commands/submit-and-wait",
                                new System.Net.Http.StringContent(createPayload, System.Text.Encoding.UTF8, "application/json"), token);
                            createResp.EnsureSuccessStatusCode();
                            Console.WriteLine("[FactoryInitExtensions] Factory contract created successfully");
                        }, "Creating factory contract");

                        await notification.PublishUpdateAsync(factoryResource, s => s with
                        {
                            State = KnownResourceStates.Running,
                            Properties = [new("status", "Factory contract initialized")]
                        });
                    }
                    catch (Exception ex)
                    {
                        Console.Error.WriteLine($"[FactoryInitExtensions] Factory initialization failed: {ex}");
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
                }, ct);
            }
        });

        return result;
    }

    private static async Task RetryAsync(
        CancellationToken ct,
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
                Console.WriteLine($"[FactoryInitExtensions] {description} attempt {i + 1}/{maxRetries}: {ex.Message}");
                await Task.Delay(delayMs, ct);
            }
        }

        await operation(ct);
    }
}
