using Aspire.Hosting;
using Aspire.Hosting.ApplicationModel;

namespace Canton.Aspire.Hosting;

public static class CantonSynchronizerExtensions
{
    private const int DefaultSequencerPublicPort = 5001;
    private const int DefaultSequencerAdminPort = 5002;
    private const int DefaultMediatorAdminPort = 5202;

    public static IResourceBuilder<ContainerResource> AddCantonSynchronizer(
        this IDistributedApplicationBuilder builder,
        string name,
        string? imageTag = null)
    {
        var tag = imageTag ?? CantonImageDefaults.Tag;
        var image = CantonImageDefaults.Registry + "/canton-base:" + tag;

        return builder.AddContainer(name, image)
            .WithBindMount("./configs/synchronizer-bootstrap.sc", "/app/bootstrap.sc", isReadOnly: true)
            .WithBindMount(CantonImageDefaults.SharedBootstrapPath, "/app/shared-bootstrap.sc", isReadOnly: true)
            .WithEnvironment("LOG_LEVEL_STDOUT", "INFO");
    }

    public static IResourceBuilder<ContainerResource> WithRemoteSequencer(
        this IResourceBuilder<ContainerResource> builder,
        IResourceBuilder<ContainerResource> sequencer,
        int? publicPort = null,
        int? adminPort = null)
    {
        var name = sequencer.Resource.Name;
        var pub = publicPort ?? DefaultSequencerPublicPort;
        var adm = adminPort ?? DefaultSequencerAdminPort;

        return builder
            .WithEnvironment("ADDITIONAL_CONFIG_01", "canton.remote-sequencers." + name + ".public-api.port = " + pub.ToString())
            .WithEnvironment("ADDITIONAL_CONFIG_02", "canton.remote-sequencers." + name + ".public-api.address = " + name)
            .WithEnvironment("ADDITIONAL_CONFIG_03", "canton.remote-sequencers." + name + ".admin-api.port = " + adm.ToString())
            .WithEnvironment("ADDITIONAL_CONFIG_04", "canton.remote-sequencers." + name + ".admin-api.address = " + name)
            .WaitFor(sequencer);
    }

    public static IResourceBuilder<ContainerResource> WithRemoteMediator(
        this IResourceBuilder<ContainerResource> builder,
        IResourceBuilder<ContainerResource> mediator,
        int? adminPort = null)
    {
        var name = mediator.Resource.Name;
        var adm = adminPort ?? DefaultMediatorAdminPort;

        return builder
            .WithEnvironment("ADDITIONAL_CONFIG_05", "canton.remote-mediators." + name + ".admin-api.port = " + adm.ToString())
            .WithEnvironment("ADDITIONAL_CONFIG_06", "canton.remote-mediators." + name + ".admin-api.address = " + name)
            .WaitFor(mediator);
    }
}
