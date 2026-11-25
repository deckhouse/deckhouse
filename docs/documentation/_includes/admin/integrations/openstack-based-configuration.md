## Layouts

This section describes the possible node placement layouts in {{ site.data.admin.cloud-types.types[page.cloud_type].name }} infrastructure.
The selected layout affects networking behavior, availability of public IP addresses, outgoing traffic routing,
and how nodes are accessed.

{% alert level="info" %}
An OpenStack cluster is deployed in a single region ([`provider.region`](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration-provider-region)).
Zones from other regions cannot be used.
For details on configuring zones for CloudPermanent and CloudEphemeral nodes, see
[How do I create NodeGroups in availability zones?](/modules/cloud-provider-openstack/faq.html#how-do-i-create-nodegroups-in-availability-zones) section.
{% endalert %}

### Standard

In this scheme, an internal cluster network is created with a gateway to the public network;
the nodes do not have public IP addresses.
The floating IP is assigned to the master node.

![Standard layout in OpenStack](../../../../images/cloud-provider-openstack/openstack-standard.png)
<!--- Source: https://docs.google.com/drawings/d/1hjmDn2aJj3ru3kBR6Jd6MAW3NWJZMNkend_K43cMN0w/edit --->

{% alert level="warning" %}
If the provider does not support SecurityGroups,
all applications running on nodes with Floating IPs assigned will be available at a public IP.
For example, `kube-apiserver` on master nodes will be available on port `6443`.
To avoid this, we recommend using the [SimpleWithInternalNetwork](#simplewithinternalnetwork) or [Standard](#standard) layout
with a bastion host.
{% endalert %}

Example layout configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: OpenStackClusterConfiguration
layout: Standard
standard:
  internalNetworkCIDR: 192.168.199.0/24         # Required.
  internalNetworkDNSServers:                    # Required.
  - 8.8.8.8
  - 4.2.2.2
  internalNetworkSecurity: true|false           # Optional, true by default.
  externalNetworkName: shared                   # Required.
  bastion:
    zone: ru2-b                                 # Optional.
    volumeType: fast-ru-2b                      # Optional.
    instanceClass:
      flavorName: m1.large                      # Required.
      imageName: ubuntu-20-04-cloud-amd64       # Required.
      rootDiskSize: 50                          # Optional, 50 GB by default.
      additionalTags:
        severity: critical                      # Optional.
        environment: production                 # Optional.
masterNodeGroup:
  replicas: 3
  instanceClass:
    flavorName: m1.large                        # Required.
    imageName: ubuntu-18-04-cloud-amd64         # Required.
    # Optional, local disk is used if not specified.
    rootDiskSize: 50
    # Optional, additional security groups.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
    additionalTags:
      severity: critical
      environment: production
  # Required, volume type map for etcd and Kubernetes certificates
  # (always use the fastest disk supplied by the provider).
  volumeTypeMap:
    # If rootDiskSize is specified, this volume type will
    # be also used for master root volume.
    ru-1a: fast-ru-1a
    ru-1b: fast-ru-1b
    ru-1c: fast-ru-1c
nodeGroups:
- name: front
  replicas: 2
  instanceClass:
    flavorName: m1.small                        # Required.
    imageName: ubuntu-18-04-cloud-amd64         # Required
    # Optional, local disk is used if not specified.
    rootDiskSize: 20
    # Optional, false by default. Determines if configuration drive is
    # required during vm bootstrap process. It's needed if there
    # is no DHCP in the network that is used as a default gateway.
    configDrive: false
    # Required, the gateway of this network will be used as the default gateway.
    # Matches the prefix parameter in the global ModuleConfig.
    mainNetwork: kube
    additionalNetworks:                         # Optional.
    - office
    - shared
    # Optional, if there are networks with disabled port
    # security their names must be specified.
    networksWithSecurityDisabled:
    - office
    # Optional, a list of network pools where to order floating IPs.
    floatingIPPools:
    - public
    - shared
    # Optional, additional security groups.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
  zones:
  - ru-1a
  - ru-1b
sshPublicKey: "<SSH_PUBLIC_KEY>"
tags:
  project: cms
  owner: default
provider:
  ...
```

### StandardWithNoRouter

An internal cluster network is created that does not have access to the public network.
All nodes (including master ones) have two interfaces: the first one to the public network, the second one to the internal network.
This layout should be used if you want all nodes in the cluster to be directly accessible.

{% alert level="warning" %}

- This strategy does not support a LoadBalancer since a Floating IP is not available for the router-less network.
  Thus, you cannot provision a LoadBalancer with the Floating IP.
  An internal LoadBalancer with the virtual IP in the public network is only accessible from cluster nodes.
- Using this strategy, make sure to explicitly specify the name of the internal network in `additionalNetworks`
  when creating an [OpenStackInstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass) in the cluster.
{% endalert %}

![StandardWithNoRouter layout in OpenStack](../../../../images/cloud-provider-openstack/openstack-standardwithnorouter.png)
<!--- Source: https://docs.google.com/drawings/d/1gkuJhyGza0bXB2lcjdsQewWLEUCjqvTkkba-c5LtS_E/edit --->

Example layout configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: OpenStackClusterConfiguration
layout: StandardWithNoRouter
standardWithNoRouter:
  internalNetworkCIDR: 192.168.199.0/24          # Required.
  externalNetworkName: ext-net                   # Required.
  # Optional, defines whether DHCP is enabled in specified external
  # network (true by default).
  externalNetworkDHCP: false
  internalNetworkSecurity: true|false            # Optional, true by default.
masterNodeGroup:
  replicas: 3
  instanceClass:
    flavorName: m1.large                         # Required.
    imageName: ubuntu-18-04-cloud-amd64          # Required.
    # Optional, local disk is used if not specified.
    rootDiskSize: 50
    # Optional, additional security groups.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
  # Required, volume type map for etcd and Kubernetes certificates
  # (always use the fastest disk supplied by the provider).
  volumeTypeMap:
    # If rootDiskSize is specified, this volume type will
    # be also used for master root volume.
    nova: ceph-ssd
nodeGroups:
- name: front
  replicas: 2
  instanceClass:
    flavorName: m1.small                         # Required.
    imageName: ubuntu-18-04-cloud-amd64          # Required.
    # Optional, local disk is used if not specified.
    rootDiskSize: 20
    # Optional, false by default, determines if configuration drive
    # is required during vm bootstrap process. It's needed
    # if there is no DHCP in the network that is used as a default
    # gateway.
    configDrive: false
    # Required, the gateway of the network will be used as the default gateway.
    # Matches the prefix parameter in the global ModuleConfig.
    mainNetwork: kube
    additionalNetworks:                          # Optional.
    - office
    - shared
    # Optional, if there are networks with disabled port
    # security, their names must be specified.
    networksWithSecurityDisabled:
    - office
    # Optional, a list of network pools where to order floating IPs.
    floatingIPPools:
    - public
    - shared
    # Optional, additional security groups.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
  # Required if rootDiskSize is specified. Volume type map for node's root volume.
  volumeTypeMap:
    nova: ceph-ssd
sshPublicKey: "<SSH_PUBLIC_KEY>"
provider:
  ...
```

### Simple

The master node and cluster nodes are connected to the existing network.
This placement strategy can be useful if you need to merge a Kubernetes cluster with existing VMs.

{% alert level="warning" %}
This strategy does not support a LoadBalancer since a Floating IP is not available for the router-less network.
Thus, you cannot provision a LoadBalancer with the Floating IP.
An internal LoadBalancer with the virtual IP in the public network is only accessible from cluster nodes.
{% endalert %}

![Simple layout in OpenStack](../../../../images/cloud-provider-openstack/openstack-simple.png)
<!--- Source: https://docs.google.com/drawings/d/1l-vKRNA1NBPIci3Ya8r4dWL5KA9my7_wheFfMR38G10/edit --->

Example layout configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: OpenStackClusterConfiguration
layout: Simple
simple:
  externalNetworkName: ext-net                  # Required.
  externalNetworkDHCP: false                    # Optional, true by default.
  # Optional, VXLAN by default, may also be DirectRouting
  # or DirectRoutingWithPortSecurityEnabled.
  podNetworkMode: VXLAN
masterNodeGroup:
  replicas: 3
  instanceClass:
    flavorName: m1.large                        # Required.
    imageName: ubuntu-18-04-cloud-amd64         # Required.
    # Optional, local disk is used if not specified.
    rootDiskSize: 50
    # Optional, additional security groups.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
  # Required, volume type map for etcd and Kubernetes certificates
  # (always use the fastest disk supplied by the provider).
  volumeTypeMap:
    # If rootDiskSize is specified, this volume type will
    # be also used for master root volume.
    nova: ceph-ssd
nodeGroups:
- name: front
  replicas: 2
  instanceClass:
    flavorName: m1.small                        # Required.
    imageName: ubuntu-18-04-cloud-amd64         # Required.
    # Optional, local disk is used if not specified.
    rootDiskSize: 20
    # Optional, false by default, determines if configuration drive
    # is required during vm bootstrap process. It's needed
    # if there is no DHCP in network that is used as a default
    # gateway.
    configDrive: false
    # Required, the network will be used as a default gateway.
    # Matches the name of the pre-created network.
    mainNetwork: kube
    additionalNetworks:                         # Optional.
    - office
    - shared
    # Optional, if there are networks with disabled port
    # security, their names must be specified.
    networksWithSecurityDisabled:
    - office
    # Optional, list of network pools where to order floating IPs.
    floatingIPPools:
    - public
    - shared
    # Optional, additional security groups.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
sshPublicKey: "<SSH_PUBLIC_KEY>"
provider:
  ...
```

### SimpleWithInternalNetwork

The master node and cluster nodes are connected to the existing network.
This layout can be useful if you need to merge a Kubernetes cluster with existing VMs.

![SimpleWithInternalNetwork layout in OpenStack](../../../../images/cloud-provider-openstack/openstack-simplewithinternalnetwork.png)
<!--- Source: https://docs.google.com/drawings/d/1H9HGOn4abpmZwIhpwwdZSSO9izvyOZakG8HpmmzZZEo/edit --->

Example of the layout configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: OpenStackClusterConfiguration
layout: SimpleWithInternalNetwork
simpleWithInternalNetwork:
  # All cluster nodes have to be in the same subnet. Set either the subnet name
  # in internalSubnetName or its UUID in internalSubnetID.
  internalSubnetName: pivot-standard
  # Optional, DirectRoutingWithPortSecurityEnabled by default,
  # may also be DirectRouting or VXLAN.
  podNetworkMode: DirectRoutingWithPortSecurityEnabled
  # Optional. If set, it will be used for the load balancer default
  # configuration and ordering the master floating IP.
  externalNetworkName: ext-net
  # Optional, true by default.
  masterWithExternalFloatingIP: false
masterNodeGroup:
  replicas: 3
  instanceClass:
    flavorName: m1.large                        # Required.
    imageName: ubuntu-18-04-cloud-amd64         # Required.
    # Optional, local disk is used if not specified.
    rootDiskSize: 50
    # Optional, additional security groups.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
  # Required, volume type map for etcd and Kubernetes certificates
  # (always use the fastest disk supplied by the provider).
  volumeTypeMap:
    # If rootDiskSize is specified, this volume type will
    # be also used for master root volume.
    nova: ceph-ssd
nodeGroups:
- name: front
  replicas: 2
  instanceClass:
    flavorName: m1.small                        # Required.
    imageName: ubuntu-18-04-cloud-amd64         # Required.
    # Optional, local disk is used if not specified.
    rootDiskSize: 20
    # Optional, false by default, determines if configuration drive
    # is required during vm bootstrap process. It's needed
    # if there is no DHCP in the network that is used as a default
    # gateway.
    configDrive: false
    # Required, the network will be used as a default gateway.
    # Matches the name of the pre-created network.
    mainNetwork: kube
    additionalNetworks:                         # Optional.
    - office
    - shared
    # Optional, if there are networks with disabled port
    # security, their names must be specified.
    networksWithSecurityDisabled:
    - office
    # Optional, a list of network pools where to order floating IPs.
    floatingIPPools:
    - public
    - shared
    # Optional, additional security groups.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
sshPublicKey: "<SSH_PUBLIC_KEY>"
provider:
  ...
```
