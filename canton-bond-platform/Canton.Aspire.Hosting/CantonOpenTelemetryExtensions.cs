using Aspire.Hosting;
using Aspire.Hosting.ApplicationModel;

namespace Canton.Aspire.Hosting;

public static class CantonOpenTelemetryExtensions
{
    private const int DefaultOtlpGrpcPort = 4317;

    public static IResourceBuilder<ContainerResource> AddOpenTelemetryCollector(
        this IDistributedApplicationBuilder builder,
        string name = "otel-collector",
        string configPath = "./configs/otel-collector.yaml",
        string imageTag = "0.128.0")
    {
        return builder.AddContainer(name, "otel/opentelemetry-collector-contrib:" + imageTag)
            .WithArgs("--config=/etc/otelcol-contrib/config.yaml")
            .WithBindMount(configPath, "/etc/otelcol-contrib/config.yaml", isReadOnly: true)
            .WithEndpoint("otlp-grpc", e => { e.Port = DefaultOtlpGrpcPort; e.TargetPort = DefaultOtlpGrpcPort; })
            .WithOtlpExporter()
            .WithEnvironment("ASPIRE_API_KEY", builder.Configuration["AppHost:OtlpApiKey"]);
    }

    public static IResourceBuilder<ContainerResource> WithCantonOpenTelemetry(
        this IResourceBuilder<ContainerResource> builder,
        string collectorHost = "otel-collector",
        int collectorPort = DefaultOtlpGrpcPort,
        double sampleRatio = 0.1)
    {
        return builder
            .WithEnvironment("ADDITIONAL_CONFIG_90", "canton.monitoring.tracing.propagation = enabled")
            .WithEnvironment("ADDITIONAL_CONFIG_91", "canton.monitoring.tracing.tracer.exporter.type = otlp")
            .WithEnvironment("ADDITIONAL_CONFIG_92", "canton.monitoring.tracing.tracer.exporter.address = \"" + collectorHost + "\"")
            .WithEnvironment("ADDITIONAL_CONFIG_93", "canton.monitoring.tracing.tracer.exporter.port = " + collectorPort.ToString())
            .WithEnvironment("ADDITIONAL_CONFIG_94", "canton.monitoring.tracing.tracer.sampler.type = trace-id-ratio")
            .WithEnvironment("ADDITIONAL_CONFIG_95", "canton.monitoring.tracing.tracer.sampler.ratio = " + sampleRatio.ToString(System.Globalization.CultureInfo.InvariantCulture))
            .WithEnvironment("ADDITIONAL_CONFIG_96", "canton.monitoring.tracing.tracer.sampler.parent-based = true");
    }
}
