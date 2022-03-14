---
title: "Cloud provider - Openstack: Layouts"
---

Four layouts are supported. Below is more information about each of them.

## Standard
In this scheme, an internal cluster network is created with a gateway to the public network; the nodes do not have public IP addresses. Note that the floating IP is assigned to the master node.

> **Caution!**
> If the provider does not support SecurityGroups, all applications running on nodes with FloatingIPs assigned will be available at a public IP. For example, kube-apiserver on master nodes will be available on port 6443. To avoid this, we recommend using the SimpleWithInternalNetwork placement strategy.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vSTIcQnxcwHsgANqHE5Ry_ZcetYX2lTFdDjd3Kip5cteSbUxwRjR3NigwQzyTMDGX10_Avr_mizOB5o/pub?w=960&h=720)
<!--- Source: https://docs.google.com/drawings/d/1hjmDn2aJj3ru3kBR6Jd6MAW3NWJZMNkend_K43cMN0w/edit --->

Example of the layout configuration:

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
  bastion:
    zone: ru2-b                                           # optional
    volumeType: fast-ru-2b                                # optional
    instanceClass:
      flavorName: m1.large                                # required
      imageName: ubuntu-20-04-cloud-amd64                 # required
      rootDiskSize: 50                                    # optional, local disk is used if not specified
      additionalTags:
        severity: critical                                # optional
        environment: production                           # optional
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
sshPublicKey: "ssh-rsa <SSH_PUBLIC_KEY>"
tags:
  project: cms
  owner: default
provider:
  ...
```

## StandardWithNoRouter
An internal cluster network is created that does not have access to the public network. All nodes (including master ones) have two interfaces: the first one to the public network, the second one to the internal network. This layout should be used if you want all nodes in the cluster to be directly accessible.

> **Caution!**
> This strategy does not support a LoadBalancer since a floating IP is not available for the router-less network. Thus, you cannot provision a load balancer with the floating IP. An internal load balancer with the virtual IP in the public network is only accessible to cluster nodes.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vR9Vlk22tZKpHgjOeQO2l-P0hyAZiwxU6NYGaLUsnv-OH0so8UXNnvrkNNiAROMHVI9iBsaZpfkY-kh/pub?w=960&h=720)
<!--- Source: https://docs.google.com/drawings/d/1gkuJhyGza0bXB2lcjdsQewWLEUCjqvTkkba-c5LtS_E/edit --->

Example of the layout configuration:

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
sshPublicKey: "ssh-rsa <SSH_PUBLIC_KEY>"
provider:
  ...
```

## Simple

The master node and cluster nodes are connected to the existing network. This placement strategy might come in handy if you need to merge a Kubernetes cluster with existing VMs.

> **Caution!**
> This strategy does not support a LoadBalancer since a floating IP is not available for the router-less network. Thus, you cannot provision a load balancer with the floating IP. An internal load balancer with the virtual IP in the public network is only accessible to cluster nodes.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vTZbaJg7oIvoh2hkEW-DKbqeujhOiJtv_JSvfvDfXE9-mX_p6uggoY1Z9N2EAJ79c7IMfQC9ttQAmaP/pub?w=960&h=720)
<!--- Source: https://docs.google.com/drawings/d/1l-vKRNA1NBPIci3Ya8r4dWL5KA9my7_wheFfMR38G10/edit --->

Example of the layout configuration:

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
sshPublicKey: "ssh-rsa <SSH_PUBLIC_KEY>"
provider:
  ...
```

## SimpleWithInternalNetwork

The master node and cluster nodes are connected to the existing network. This placement strategy might come in handy if you need to merge a Kubernetes cluster with existing VMs.

> **Caution!**
> This placement strategy does not involve the management of `SecurityGroups` (it is assumed they were created beforehand).
>To configure security policies, you must explicitly specify both `additionalSecurityGroups` in the OpenStackClusterConfiguration
>for the masterNodeGroup and other nodeGroups, and `additionalSecurityGroups` when creating `OpenStackInstanceClass` in the cluster.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vQOcYZPtHBqMtlNx9PDcMrqI0WEwRssL-oXONnrOoKNaIx1fcEODo9dK2zOoF1wbKeKJlhphFTuefB-/pub?w=960&h=720)
<!--- Source: https://docs.google.com/drawings/d/1H9HGOn4abpmZwIhpwwdZSSO9izvyOZakG8HpmmzZZEo/edit --->

Example of the layout configuration:

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
sshPublicKey: "ssh-rsa <SSH_PUBLIC_KEY>"
provider:
  ...
```
