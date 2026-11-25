## Configuration

Integration with {{ site.data.admin.cloud-types.types[page.cloud_type].name }} is performed
using the [OpenStackClusterConfiguration](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration) resource.
It defines the configuration of the cloud cluster in {{ site.data.admin.cloud-types.types[page.cloud_type].name }}
and is used by the cloud provider when the cluster control plane is hosted in the cloud.
The Deckhouse Platform module responsible for the integration is automatically configured based on the selected layout.

To modify the configuration in a running cluster, run the following command:

```shell
d8 system edit provider-cluster-configuration
```

{% alert level="info" %}
After changing node-related parameters, run the `dhctl converge` command to apply the changes.
{% endalert %}
