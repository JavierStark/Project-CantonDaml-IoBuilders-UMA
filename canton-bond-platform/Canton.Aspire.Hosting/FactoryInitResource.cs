using Aspire.Hosting.ApplicationModel;

namespace Canton.Aspire.Hosting;

public sealed class FactoryInitResource(string name) : Resource(name), IResourceWithWaitSupport;
