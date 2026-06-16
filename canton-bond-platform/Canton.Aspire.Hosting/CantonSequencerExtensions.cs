using Aspire.Hosting;
using Aspire.Hosting.ApplicationModel;

namespace Canton.Aspire.Hosting;

public static class CantonSequencerExtensions
{
    private const int DefaultPublicPort = 5001;
    private const int DefaultAdminPort = 5002;

    public static IResourceBuilder<ContainerResource> AddCantonSequencer(
        this IDistributedApplicationBuilder builder,
        string name,
        int publicPort = DefaultPublicPort,
        int adminPort = DefaultAdminPort,
        string? imageTag = null)
    {
        var tag = imageTag ?? CantonImageDefaults.Tag;
        var image = CantonImageDefaults.Registry + "/canton-sequencer:" + tag;

        return builder.AddContainer(name, image)
            .WithEndpoint("public", e => { e.Port = publicPort; e.TargetPort = publicPort; })
            .WithEndpoint("admin", e => { e.Port = adminPort; e.TargetPort = adminPort; })
            .WithBindMount("./configs/sequencer-bootstrap.sc", "/app/bootstrap.sc", isReadOnly: true)
            .WithBindMount(CantonImageDefaults.SharedBootstrapPath, "/app/shared-bootstrap.sc", isReadOnly: true)
            .WithEnvironment("LOG_LEVEL_STDOUT", "INFO")
            .WithEnvironment("ADDITIONAL_CONFIG_01", "canton.sequencers.sequencer.public-api.port = " + publicPort.ToString())
            .WithEnvironment("ADDITIONAL_CONFIG_02", "canton.sequencers.sequencer.admin-api.port = " + adminPort.ToString())
            .WithEnvironment("ADDITIONAL_CONFIG_03", "canton.sequencers.sequencer.storage.type = memory")
            .WithEnvironment("ADDITIONAL_CONFIG_04", "canton.sequencers.sequencer.sequencer.config.storage.type = memory");
    }

    public static IResourceBuilder<ContainerResource> WithPublicPort(
        this IResourceBuilder<ContainerResource> builder,
        int port)
    {
        return builder
            .WithEndpoint("public", e => { e.Port = port; e.TargetPort = port; })
            .WithEnvironment("ADDITIONAL_CONFIG_01", "canton.sequencers.sequencer.public-api.port = " + port.ToString());
    }
}
