using Aspire.Hosting;
using Aspire.Hosting.ApplicationModel;

namespace Canton.Aspire.Hosting;

public static class CantonMediatorExtensions
{
    private const int DefaultAdminPort = 5202;

    public static IResourceBuilder<ContainerResource> AddCantonMediator(
        this IDistributedApplicationBuilder builder,
        string name,
        int adminPort = DefaultAdminPort,
        string? imageTag = null)
    {
        var tag = imageTag ?? CantonImageDefaults.Tag;
        var image = CantonImageDefaults.Registry + "/canton-mediator:" + tag;

        return builder.AddContainer(name, image)
            .WithEndpoint("admin", e => { e.Port = adminPort; e.TargetPort = adminPort; })
            .WithBindMount("./configs/mediator-bootstrap.sc", "/app/bootstrap.sc", isReadOnly: true)
            .WithBindMount(CantonImageDefaults.SharedBootstrapPath, "/app/shared-bootstrap.sc", isReadOnly: true)
            .WithEnvironment("LOG_LEVEL_STDOUT", "INFO")
            .WithEnvironment("ADDITIONAL_CONFIG_01", "canton.mediators.mediator.admin-api.port = " + adminPort.ToString())
            .WithEnvironment("ADDITIONAL_CONFIG_02", "canton.mediators.mediator.storage.type = memory");
    }
}
