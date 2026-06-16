using Aspire.Hosting;
using Aspire.Hosting.ApplicationModel;

namespace Canton.Aspire.Hosting;

public static class CantonParticipantExtensions
{
    public static IResourceBuilder<ContainerResource> AddCantonParticipant(
        this IDistributedApplicationBuilder builder,
        string name,
        string? imageTag = null)
    {
        var tag = imageTag ?? CantonImageDefaults.Tag;
        var image = CantonImageDefaults.Registry + "/canton-participant:" + tag;

        return builder.AddContainer(name, image)
            .WithBindMount(CantonImageDefaults.SharedBootstrapPath, "/app/shared-bootstrap.sc", isReadOnly: true)
            .WithBindMount("./dars", "/app/dars", isReadOnly: true)
            .WithEnvironment("LOG_LEVEL_STDOUT", "INFO")
            .WithEnvironment("ADDITIONAL_CONFIG_04", "canton.participants.participant.ledger-api.auth-services = []")
            .WithEnvironment("ADDITIONAL_CONFIG_05", "canton.participants.participant.storage.type = memory");
    }

    public static IResourceBuilder<ContainerResource> WithAdminPort(
        this IResourceBuilder<ContainerResource> builder,
        int port)
    {
        return builder
            .WithEndpoint("admin", e => { e.Port = port; e.TargetPort = port; })
            .WithEnvironment("ADDITIONAL_CONFIG_01", "canton.participants.participant.admin-api.port = " + port.ToString());
    }

    public static IResourceBuilder<ContainerResource> WithLedgerPort(
        this IResourceBuilder<ContainerResource> builder,
        int port)
    {
        return builder
            .WithEndpoint("ledger-api", e => { e.Port = port; e.TargetPort = port; })
            .WithEnvironment("ADDITIONAL_CONFIG_02", "canton.participants.participant.ledger-api.port = " + port.ToString());
    }

    public static IResourceBuilder<ContainerResource> WithHttpPort(
        this IResourceBuilder<ContainerResource> builder,
        int port)
    {
        return builder
            .WithEndpoint("http-json-api", e => { e.Port = port; e.TargetPort = port; })
            .WithEnvironment("ADDITIONAL_CONFIG_03", "canton.participants.participant.http-ledger-api.port = " + port.ToString())
            .WithEnvironment("ADDITIONAL_CONFIG_06", "canton.participants.participant.http-ledger-api.address = 0.0.0.0");
    }

    public static IResourceBuilder<ContainerResource> WithBootstrapScript(
        this IResourceBuilder<ContainerResource> builder,
        string hostPath)
    {
        return builder.WithBindMount(hostPath, "/app/bootstrap.sc", isReadOnly: true);
    }

    public static IResourceBuilder<ContainerResource> WithDarsDirectory(
        this IResourceBuilder<ContainerResource> builder,
        string hostPath)
    {
        return builder.WithBindMount(hostPath, "/app/dars", isReadOnly: true);
    }
}
