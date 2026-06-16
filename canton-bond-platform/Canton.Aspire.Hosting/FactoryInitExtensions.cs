using Aspire.Hosting;
using Aspire.Hosting.ApplicationModel;
using Aspire.Hosting.Lifecycle;

namespace Canton.Aspire.Hosting;

public static class FactoryInitExtensions
{
    public static IResourceBuilder<FactoryInitResource> AddFactoryInit(
        this IDistributedApplicationBuilder builder,
        string name)
    {
        builder.Services.TryAddEventingSubscriber<FactoryInitEventingSubscriber>();

        var resource = new FactoryInitResource(name);
        return builder.AddResource(resource)
            .WithInitialState(new CustomResourceSnapshot
            {
                ResourceType = "FactoryInit",
                State = KnownResourceStates.Waiting,
                Properties = [
                    new(CustomResourceKnownProperties.Source, "Canton SimpleTokenRules factory contract init")
                ]
            })
            .ExcludeFromManifest();
    }
}
