## Security groups

The `internalNetworkSecurity` parameter for the [Standard](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration-standard-internalnetworksecurity) and [StandardWithNoRouter](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration-standardwithnorouter-internalnetworksecurity) layouts (default `true`) enables the creation of a security group (SecurityGroup) when a cluster is created. Deckhouse Platform creates the group named after the cluster prefix (`prefix`) and assigns it to the nodes.

The following inbound rules will be created:

- Allow incoming traffic over the TCP protocol on port `22` from the CIDRs listed in [`sshAllowList`](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration-sshallowlist) (default `0.0.0.0/0`).
- Allow incoming traffic over the ICMP protocol from `0.0.0.0/0`.
- Allow incoming traffic over the TCP protocol on ports `30000`–`32767` for services of the `NodePort` type. Inbound UDP traffic to `NodePort` ports is not allowed by default.
- Allow any incoming traffic from nodes in the same security group.

In the Simple and SimpleWithInternalNetwork layouts, Deckhouse Platform does not create security groups. Prepare and attach them as described below.

### Attaching custom security groups

You can attach your own security groups to the nodes. Prepare them in the cloud beforehand by following the [OpenStack documentation](https://docs.openstack.org/nova/latest/user/security-groups.html) and specify them in `additionalSecurityGroups`:

- For master nodes, in the [`masterNodeGroup.instanceClass.additionalSecurityGroups`](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration-masternodegroup-instanceclass-additionalsecuritygroups) parameter of the [OpenStackClusterConfiguration](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration) resource.
- For static nodes, in the [`nodeGroups[].instanceClass.additionalSecurityGroups`](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration-nodegroups-instanceclass-additionalsecuritygroups) parameter of the [OpenStackClusterConfiguration](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration) resource.
- For ephemeral nodes, in the [`spec.additionalSecurityGroups`](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-additionalsecuritygroups) parameter of the [OpenStackInstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass) resource.
