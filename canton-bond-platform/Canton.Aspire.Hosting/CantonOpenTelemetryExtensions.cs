using Aspire.Hosting;
using Aspire.Hosting.ApplicationModel;

namespace Canton.Aspire.Hosting;

public static class CantonOpenTelemetryExtensions
{
    private const int DefaultOtlpGrpcPort = 4317;
    private const int DefaultOtlpHttpPort = 4318;
    private const int DefaultHealthPort = 13133;
    private const int DefaultZPagesPort = 55679;
    private const int DefaultPrometheusPort = 9090;
    private const int DefaultTempoPort = 3200;
    private const int DefaultGrafanaHostPort = 3001;
    private const int DefaultGrafanaContainerPort = 3000;

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
            .WithEndpoint("otlp-http", e => { e.Port = DefaultOtlpHttpPort; e.TargetPort = DefaultOtlpHttpPort; })
            .WithEndpoint("health", e => { e.Port = DefaultHealthPort; e.TargetPort = DefaultHealthPort; })
            .WithEndpoint("zpages", e => { e.Port = DefaultZPagesPort; e.TargetPort = DefaultZPagesPort; });
    }

    public static IResourceBuilder<ContainerResource> AddPrometheus(
        this IDistributedApplicationBuilder builder,
        string name = "prometheus",
        string configPath = "./configs/prometheus.yml",
        string imageTag = "v2.55.1")
    {
        return builder.AddContainer(name, "prom/prometheus:" + imageTag)
            .WithArgs(
                "--config.file=/etc/prometheus/prometheus.yml",
                "--storage.tsdb.path=/prometheus",
                "--web.enable-lifecycle",
                "--web.enable-remote-write-receiver")
            .WithBindMount(configPath, "/etc/prometheus/prometheus.yml", isReadOnly: true)
            .WithEndpoint("http", e => { e.Port = DefaultPrometheusPort; e.TargetPort = DefaultPrometheusPort; });
    }

    public static IResourceBuilder<ContainerResource> AddTempo(
        this IDistributedApplicationBuilder builder,
        string name = "tempo",
        string configPath = "./configs/tempo.yaml",
        string imageTag = "2.7.1")
    {
        return builder.AddContainer(name, "grafana/tempo:" + imageTag)
            .WithArgs("-config.file=/etc/tempo.yaml")
            .WithBindMount(configPath, "/etc/tempo.yaml", isReadOnly: true)
            .WithEndpoint("http", e => { e.Port = DefaultTempoPort; e.TargetPort = DefaultTempoPort; });
    }

    public static IResourceBuilder<ContainerResource> AddGrafana(
        this IDistributedApplicationBuilder builder,
        string name = "grafana",
        string imageTag = "11.5.2")
    {
        return builder.AddContainer(name, "grafana/grafana:" + imageTag)
            .WithEnvironment("GF_SECURITY_ADMIN_USER", "admin")
            .WithEnvironment("GF_SECURITY_ADMIN_PASSWORD", "admin")
            .WithEnvironment("GF_USERS_ALLOW_SIGN_UP", "false")
            .WithBindMount("./configs/grafana/provisioning", "/etc/grafana/provisioning", isReadOnly: true)
            .WithBindMount("./configs/grafana/dashboards", "/var/lib/grafana/dashboards", isReadOnly: true)
            .WithEndpoint("http", e => { e.Port = DefaultGrafanaHostPort; e.TargetPort = DefaultGrafanaContainerPort; });
    }

    public static IResourceBuilder<ContainerResource> WithCantonOpenTelemetryTracing(
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
