---
title: "Cloud provider - Openstack: Layouts"
---

## Layouts
### Standard
In this scheme, an internal cluster network is created with a gateway to the public network; the nodes do not have public IP addresses. Note that the floating IP is assigned to the master node.

**Caution**
If the provider does not support SecurityGroups, all applications running on nodes with FloatingIPs assigned will be available at a public IP. For example, kube-apiserver on master nodes will be available on port 6443. To avoid this, we recommend using the SimpleWithInternalNetwork placement strategy.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vSTIcQnxcwHsgANqHE5Ry_ZcetYX2lTFdDjd3Kip5cteSbUxwRjR3NigwQzyTMDGX10_Avr_mizOB5o/pub?w=960&h=720)
<!--- Исходник: https://docs.google.com/drawings/d/1hjmDn2aJj3ru3kBR6Jd6MAW3NWJZMNkend_K43cMN0w/edit --->

```yaml
apiVersion: deckhouse.io/v1
kind: OpenStackClusterConfiguration
layout: Standard
standard:
  internalNetworkCIDR: 192.168.199.0/24                   # required
  internalNetworkDNSServers:                              # required
  - 8.8.8.8
  - 4.2.2.2
  internalNetworkSecurity: true|false                     # optional, default true
  externalNetworkName: shared                             # required
masterNodeGroup:
  replicas: 3
  instanceClass:
    flavorName: m1.large                                  # required
    imageName: ubuntu-18-04-cloud-amd64                   # required
    rootDiskSize: 50                                      # optional, local disk is used if not specified
    additionalSecurityGroups:                             # optional, additional security groups
    - sec_group_1
    - sec_group_2
    additionalTags:
      severity: critical
      environment: production
  volumeTypeMap:                                          # required, volume type map for etcd and kubernetes certs (always use fastest disk supplied by provider).
    ru-1a: fast-ru-1a                                     # If rootDiskSize specified than this volume type will be also used for master root volume
    ru-1b: fast-ru-1b
    ru-1c: fast-ru-1c
nodeGroups:
- name: front
  replicas: 2
  instanceClass:
    flavorName: m1.small                                  # required
    imageName: ubuntu-18-04-cloud-amd64                   # required
    rootDiskSize: 20                                      # optional, local disk is used if not specified
    configDrive: false                                    # optional, default false, determines if config drive is required during vm bootstrap process. It's needed if there is no dhcp in network that is used as default gateway
    mainNetwork: kube                                     # required, network will be used as default gateway
    additionalNetworks:                                   # optional
    - office
    - shared
    networksWithSecurityDisabled:                         # optional, if there are networks with disabled port security their names must be specified
    - office
    floatingIPPools:                                      # optional, list of network pools where to order floating ips
    - public
    - shared
    additionalSecurityGroups:                             # optional, additional security groups
    - sec_group_1
    - sec_group_2
  zones:
  - ru-1a
  - ru-1b
sshPublicKey: "ssh-rsa ewasfef3wqefwefqf43qgqwfsd"
tags:
  project: cms
  owner: default
provider:
  ...
```

### StandardWithNoRouter
An internal cluster network is created that does not have access to the public network. All nodes (including master ones) have two interfaces: the first one to the public network, the second one to the internal network. This layout should be used if you want all nodes in the cluster to be directly accessible.

**Caution**
This strategy does not support a LoadBalancer since a floating IP is not available for the router-less network. Thus, you cannot provision a load balancer with the floating IP. An internal load balancer with the virtual IP in the public network is only accessible to cluster nodes.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vR9Vlk22tZKpHgjOeQO2l-P0hyAZiwxU6NYGaLUsnv-OH0so8UXNnvrkNNiAROMHVI9iBsaZpfkY-kh/pub?w=960&h=720)
<!--- Исходник: https://docs.google.com/drawings/d/1gkuJhyGza0bXB2lcjdsQewWLEUCjqvTkkba-c5LtS_E/edit --->

```yaml
apiVersion: deckhouse.io/v1
kind: OpenStackClusterConfiguration
layout: StandardWithNoRouter
standardWithNoRouter:
  internalNetworkCIDR: 192.168.199.0/24                   # required
  externalNetworkName: ext-net                            # required
  externalNetworkDHCP: false                              # optional, whether dhcp is enabled in specified external network (default true)
  internalNetworkSecurity: true|false                     # optional, default true
masterNodeGroup:
  replicas: 3
  instanceClass:
    flavorName: m1.large                                  # required
    imageName: ubuntu-18-04-cloud-amd64                   # required
    rootDiskSize: 50                                      # optional, local disk is used if not specified
    additionalSecurityGroups:                             # optional, additional security groups
    - sec_group_1
    - sec_group_2
  volumeTypeMap:                                          # required, volume type map for etcd and kubernetes certs (always use fastest disk supplied by provider).
    nova: ceph-ssd                                        # If rootDiskSize specified than this volume type will be also used for master root volume
nodeGroups:
- name: front
  replicas: 2
  instanceClass:
    flavorName: m1.small                                  # required
    imageName: ubuntu-18-04-cloud-amd64                   # required
    rootDiskSize: 20                                      # optional, local disk is used if not specified
    configDrive: false                                    # optional, default false, determines if config drive is required during vm bootstrap process. It's needed if there is no dhcp in network that is used as default gateway
    mainNetwork: kube                                     # required, network will be used as default gateway
    additionalNetworks:                                   # optional
    - office
    - shared
    networksWithSecurityDisabled:                         # optional, if there are networks with disabled port security their names must be specified
    - office
    floatingIPPools:                                      # optional, list of network pools where to order floating ips
    - public
    - shared
    additionalSecurityGroups:                             # optional, additional security groups
    - sec_group_1
    - sec_group_2
sshPublicKey: "ssh-rsa ewasfef3wqefwefqf43qgqwfsd"
provider:
  ...
```

### Simple

The master node and cluster nodes are connected to the existing network. This placement strategy might come in handy if you need to merge a Kubernetes cluster with existing VMs.

**Caution!**

This strategy does not support a LoadBalancer since a floating IP is not available for the router-less network. Thus, you cannot provision a load balancer with the floating IP. An internal load balancer with the virtual IP in the public network is only accessible to cluster nodes.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vTZbaJg7oIvoh2hkEW-DKbqeujhOiJtv_JSvfvDfXE9-mX_p6uggoY1Z9N2EAJ79c7IMfQC9ttQAmaP/pub?w=960&h=720)
<!--- Исходник: https://docs.google.com/drawings/d/1l-vKRNA1NBPIci3Ya8r4dWL5KA9my7_wheFfMR38G10/edit --->

```yaml
apiVersion: deckhouse.io/v1
kind: OpenStackClusterConfiguration
layout: Simple
simple:
  externalNetworkName: ext-net                            # required
  externalNetworkDHCP: false                              # optional, default true
  podNetworkMode: VXLAN                                   # optional, by default VXLAN, may also be DirectRouting or DirectRoutingWithPortSecurityEnabled
masterNodeGroup:
  replicas: 3
  instanceClass:
    flavorName: m1.large                                  # required
    imageName: ubuntu-18-04-cloud-amd64                   # required
    rootDiskSize: 50                                      # optional, local disk is used if not specified
    additionalSecurityGroups:                             # optional, additional security groups
    - sec_group_1
    - sec_group_2
  volumeTypeMap:                                          # required, volume type map for etcd and kubernetes certs (always use fastest disk supplied by provider).
    nova: ceph-ssd                                        # If rootDiskSize specified than this volume type will be also used for master root volume
nodeGroups:
- name: front
  replicas: 2
  instanceClass:
    flavorName: m1.small                                  # required
    imageName: ubuntu-18-04-cloud-amd64                   # required
    rootDiskSize: 20                                      # optional, local disk is used if not specified
    configDrive: false                                    # optional, default false, determines if config drive is required during vm bootstrap process. It's needed if there is no dhcp in network that is used as default gateway
    mainNetwork: kube                                     # required, network will be used as default gateway
    additionalNetworks:                                   # optional
    - office
    - shared
    networksWithSecurityDisabled:                         # optional, if there are networks with disabled port security their names must be specified
    - office
    floatingIPPools:                                      # optional, list of network pools where to order floating ips
    - public
    - shared
    additionalSecurityGroups:                             # optional, additional security groups
    - sec_group_1
    - sec_group_2
sshPublicKey: "ssh-rsa ewasfef3wqefwefqf43qgqwfsd"
provider:
  ...
```

### SimpleWithInternalNetwork

The master node and cluster nodes are connected to the existing network. This placement strategy might come in handy if you need to merge a Kubernetes cluster with existing VMs.

**Caution!**

This placement strategy does not involve the management of `SecurityGroups` (it is assumed they were created beforehand).
To configure security policies, you must explicitly specify both `additionalSecurityGroups` in the OpenStackClusterConfiguration
for the masterNodeGroup and other nodeGroups, and `additionalSecurityGroups` when creating `OpenStackInstanceClass` in the cluster.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vQOcYZPtHBqMtlNx9PDcMrqI0WEwRssL-oXONnrOoKNaIx1fcEODo9dK2zOoF1wbKeKJlhphFTuefB-/pub?w=960&h=720)
<!--- Исходник: https://docs.google.com/drawings/d/1H9HGOn4abpmZwIhpwwdZSSO9izvyOZakG8HpmmzZZEo/edit --->


```yaml
apiVersion: deckhouse.io/v1
kind: OpenStackClusterConfiguration
layout: SimpleWithInternalNetwork
simpleWithInternalNetwork:
  internalSubnetName: pivot-standard                      # required, all cluster nodes have to be in the same subnet
  podNetworkMode: DirectRoutingWithPortSecurityEnabled    # optional, by default DirectRoutingWithPortSecurityEnabled, may also be DirectRouting or VXLAN
  externalNetworkName: ext-net                            # optional, if set will be used for load balancer default configuration and ordering master floating ip
  masterWithExternalFloatingIP: false                     # optional, default value is true
masterNodeGroup:
  replicas: 3
  instanceClass:
    flavorName: m1.large                                  # required
    imageName: ubuntu-18-04-cloud-amd64                   # required
    rootDiskSize: 50                                      # optional, local disk is used if not specified
    additionalSecurityGroups:                             # optional, additional security groups
    - sec_group_1
    - sec_group_2
  volumeTypeMap:                                          # required, volume type map for etcd and kubernetes certs (always use fastest disk supplied by provider).
    nova: ceph-ssd                                        # If rootDiskSize specified than this volume type will be also used for master root volume
nodeGroups:
- name: front
  replicas: 2
  instanceClass:
    flavorName: m1.small                                  # required
    imageName: ubuntu-18-04-cloud-amd64                   # required
    rootDiskSize: 20                                      # optional, local disk is used if not specified
    configDrive: false                                    # optional, default false, determines if config drive is required during vm bootstrap process. It's needed if there is no dhcp in network that is used as default gateway
    mainNetwork: kube                                     # required, network will be used as default gateway
    additionalNetworks:                                   # optional
    - office
    - shared
    networksWithSecurityDisabled:                         # optional, if there are networks with disabled port security their names must be specified
    - office
    floatingIPPools:                                      # optional, list of network pools where to order floating ips
    - public
    - shared
    additionalSecurityGroups:                             # optional, additional security groups
    - sec_group_1
    - sec_group_2
sshPublicKey: "ssh-rsa ewasfef3wqefwefqf43qgqwfsd"
provider:
  ...
```

## OpenStackClusterConfiguration
A particular placement strategy is defined via the `OpenStackClusterConfiguration` struct. It has the following fields:
* `layout` - the way resources are located in the cloud;
  * Possible values: `Standard`, `StandardWithNoRouter`, `Simple`, `SimpleWithInternalNetwork` (see the description below);
* `Standard` — settings for the `Standard` layout;
  * `internalNetworkCIDR` — routing for the internal cluster network;
  * `internalNetworkDNSServers` — a list of addresses of the recursive DNSs of the internal cluster network;
  * `internalNetworkSecurity` — this parameter defines whether [SecurityGroups](https://early.deckhouse.io/en/documentation/v1/modules/030-cloud-provider-openstack/faq.html#how-to-check-whether-the-provider-supports-securitygroups) and [AllowedAddressPairs](https://docs.openstack.org/developer/dragonflow/specs/allowed_address_pairs.html) must be configured for ports of the internal network;
  * `externalNetworkName` — the name of the network for external connections;
* `StandardWithNoRouter` — settings for the `StandardWithNoRouter` layout;
  * `internalNetworkCIDR` — routing for the internal cluster network;
  * `externalNetworkName` — the name of the network for external connections;
  * `externalNetworkDHCP` — this parameter defines if DHCP is enabled in the external network;
  * `internalNetworkSecurity` — this parameter defines whether [SecurityGroups](https://early.deckhouse.io/en/documentation/v1/modules/030-cloud-provider-openstack/faq.html#how-to-check-whether-the-provider-supports-securitygroups) and [AllowedAddressPairs](https://docs.openstack.org/developer/dragonflow/specs/allowed_address_pairs.html) must be configured for ports of the internal network;
* `Simple` — settings for the `Simple` layout;
  * `externalNetworkName` — the name of the network for external connections;
  * `externalNetworkDHCP` — this parameter defines if DHCP is enabled in the external network;
  * `podNetworkMode` — sets the traffic mode for the network that the pods use to communicate with each other (usually, it is an internal network; however, there can be exceptions).
    * Possible values:
      * `DirectRouting` — nodes are directly routed (SecurityGroups are disabled in this mode);
      * `VXLAN` — direct routing does NOT work between nodes, VXLAN must be used (SecurityGroups are disabled in this mode);
* `SimpleWithInternalNetwork` — settings for the `SimpleWithInternalNetwork` layout;
  * `internalSubnetName` — a subnet to use for cluster nodes;
  * `podNetworkMode` — sets the traffic mode for the network that the pods use to communicate with each other (usually, it is an internal network; however, there can be exceptions).
    * Possible values:
      * `DirectRouting` — nodes are directly routed (SecurityGroups are disabled in this mode);
      * `DirectRoutingWithPortSecurityEnabled` — direct routing is enabled between the nodes, but only if  the range of addresses of the internal network is explicitly allowed in OpenStack for Ports;
        * **Caution!** Make sure that the `username` can edit AllowedAddressPairs on Ports connected to the `internalNetworkName` network. Usually, an OpenStack user doesn't have such a privilege if the network has the `shared` flag set;
      * `VXLAN` — direct routing does NOT work between nodes, VXLAN must be used (SecurityGroups are disabled in this mode);
  * `externalNetworkName` — the name of the network for external connections;
  * `masterWithExternalFloatingIP` — this parameter defines if floatingIP must be assigned to master nodes;
* `provider` — this parameter contains settings to connect to the OpenStack API; these settings are the same as those in the  `connection` field of the [cloud-provider-openstack](/en/documentation/v1/modules/030-cloud-provider-openstack/configuration.html) module;
* `masterNodeGroup` — the definition of the master's NodeGroup;
  * `replicas` — the number of master nodes to create;
  * `instanceClass` — partial contents of the fields of the [OpenStackInstanceClass](cr.html#openstackinstanceclass) CR. Required parameters: `flavorName`, `imageName`. Possible parameters:
    * `flavorName`
    * `imageName`
    * `rootDiskSize`
    * `additionalSecurityGroups`
    * `additionalTags`
  * `volumeTypeMap` — a dictionary of disk types for storing etcd data and kubernetes configuration files. If the `rootDiskSize` parameter is specified, the same disk type will be used for the VM's boot drive. We recommend using the fastest disks provided by the provider in all cases;
    * A mandatory parameter;
    * A dictionary where the key is the name of the zone, value - disk type;
    * An example:
      ```yaml
      ru-1a: fast-ru-1a
      ru-1b: fast-ru-1b
      ```
      If the value specified in `replicas` exceeds the number of elements in the dictionary, the master nodes whose number exceeds the length of the dictionary get the values starting from the beginning of the dictionary. For example, if `replicas: 5`, then master-0, master-2, master-4 will have the `ru-1a` disk type, while master-1, master-3 will have the `ru-1b` disk type;
* `nodeGroups` — an array of additional NodeGroups for creating static nodes (e.g., for dedicated front nodes or gateways). NodeGroup parameters:
  * `name` — the name of the NodeGroup to use for generating node names;
  * `replicas` — the number of nodes to create;
  * `instanceClass` — partial contents of the fields of the [OpenStackInstanceClass](cr.html#openstackinstanceclass) CR. Required parameters:  `flavorName`, `imageName`, `mainNetwork`. The parameters in **bold** are unique for `OpenStackClusterConfiguration`. Possible parameters:
    * `flavorName`
    * `imageName`
    * `rootDiskSize`
    * `mainNetwork`
    * `additionalSecurityGroups`
    * `additionalTags`
    * `additionalNetworks`
    * **`networksWithSecurityDisabled`** — this parameter contains a list of `mainNetwork` and `additionalNetworks` in which `SecurityGroups` and `AllowedAddressPairs` on ports **CANNOT** be configured;
      * Format — an array of strings;
    * **`floatingIPPools`** — a list of networks to assign Floating IPs to nodes;
      * Format — an array of strings;
    * **`configDrive`** — this flag specifies whether an additional disk containing the bootstrapping configuration will be mounted to the node. You must set it if DHCP is disabled in the `mainNetwork`.
      * An optional parameter;
      * It is set to `false` by default;
  * `zones` — a limited set of zones in which nodes can be created;
    * An optional parameter;
    * Format — an array of strings;
  * `nodeTemplate` — parameters of Node objects in Kubernetes to add after registering the node;
    * `labels` — the same as the `metadata.labels` standard [field](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta);
      * An example:
        ```yaml
        labels:
          environment: production
          app: warp-drive-ai
        ```
    * `annotations` — the same as the `metadata.annotations` standard [field](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta);
      * An example:
        ```yaml
        annotations:
          ai.fleet.com/discombobulate: "true"
        ```
    * `taints` — the same as the .spec.taints field of the [Node](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#taint-v1-core) object. **Caution!** Only the `effect`, `key`, `values`  fields are available;
      * An example:

        ```yaml
        taints:
        - effect: NoExecute
          key: ship-class
          value: frigate
        ```
* `sshPublicKey` — a public key for accessing nodes;
  * A mandatory parameter;
  * Fprmat — a string;
* `tags` — a dictionary of tags to create on all resources that support this feature. You have to re-create all the machines to add new tags if tags were modified in the running cluster;
  * An optional parameter;
  * Format — key-value pairs;
* `zones` — the globally restricted set of zones that this Cloud Provider works with.
  * An optional parameter;
  * Format — an array of strings;
