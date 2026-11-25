Deckhouse Platform integrates with [{{ site.data.admin.cloud-types.types[page.cloud_type].name }}]({{ site.data.admin.cloud-types.types[page.cloud_type].link }}) and provisions cluster nodes in it. Virtual machine parameters are set in the [OpenStackInstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass) resource, which you reference in a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Integration features:

- Managing {{ site.data.admin.cloud-types.types[page.cloud_type].name }} resources:
  - updates {{ site.data.admin.cloud-types.types[page.cloud_type].name }} server and cluster node metadata and removes from cluster nodes that no longer exist in {{ site.data.admin.cloud-types.types[page.cloud_type].name }};
  - creates load balancers (Octavia) for Services of the LoadBalancer type.
- Provisioning block disks via the Cinder CSI driver (`cinder.csi.openstack.org`) so that PersistentVolumes can be requested from the cluster. Manila (filesystem) is not supported.
- Provisioning base infrastructure and CloudPermanent nodes using the [Terraform/OpenTofu provider](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-openstack.html#module-interactions) `terraform-provider-openstack`.
- Provisioning CloudEphemeral nodes via Machine Controller Manager (MCM) or Cluster API (CAPI).
- Enabling CNI in new clusters automatically. By default, [`cni-cilium`](/modules/cni-cilium/) is used, and the network mode depends on the [`podNetworkMode`](/modules/cloud-provider-openstack/configuration.html#parameters-podnetworkmode) parameter.
