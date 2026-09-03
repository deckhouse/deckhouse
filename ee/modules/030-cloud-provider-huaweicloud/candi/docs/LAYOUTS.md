---
title: "Cloud provider - HuaweiCloud: Layouts"
description: "Schemes of placement and interaction of resources in HuaweiCloud when working with the Deckhouse cloud provider."
---

## Standard

* An internal cluster network is created with a gateway to the public network.
* The elastic IP can be assigned to the master node.
* Nodes managed by the Cluster API do not have public IP addresses.

![Standard layout](images/huawei-standard.png)
<!--- Source: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10811&t=IvETjbByf1MSQzcm-0 --->

The [`internalNetworkSecurity`](cluster_configuration.html#huaweicloudclusterconfiguration-standard-internalnetworksecurity) parameter (default `true`) enables the creation of a security group (SecurityGroup) when a cluster is created. The module creates the group named after the cluster prefix and assigns it to the nodes.

The following inbound rules will be created:

- allow incoming traffic over the TCP protocol on port `22` from `0.0.0.0/0`
- allow incoming traffic over the ICMP protocol from `0.0.0.0/0`
- allow incoming traffic over the TCP protocol on ports `30000`–`32767` for services of the `NodePort` type. Inbound UDP traffic to `NodePort` ports is not allowed by default

Unlike OpenStack, the module does not add a rule that allows all inbound traffic between nodes in the same security group. By default, only the ports and protocols listed above are open between cluster nodes.

Attach custom security groups for CloudEphemeral nodes in the [HuaweiCloudInstanceClass](cr.html#huaweicloudinstanceclass) resource via [`spec.securityGroups`](cr.html#huaweicloudinstanceclass-v1-spec-securitygroups). They are applied together with the group created by the module.

Example of the layout configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: HuaweiCloudClusterConfiguration
layout: Standard
sshPublicKey: "<SSH_PUBLIC_KEY>"
standard:
  internalNetworkDNSServers:
    - 8.8.8.8
  internalNetworkCIDR: 192.168.199.0/24
  internalNetworkSecurity: true
  enableEIP: true
provider:
  cloud: hc.sbercloud.ru
  region: ru-moscow-1
  accessKey: "<Access key>"
  secretKey: "<Secret key>"
  projectID: "<Project ID>"
masterNodeGroup:
  replicas: 1
  instanceClass:
    imageName: alt-p11
    flavorName: s7n.xlarge.2
    rootDiskSize: 50
  serverGroup:
    policy: AntiAffinity
  volumeTypeMap:
    ru-moscow-1a: SSD
```

## VpcPeering

![VpcPeering layout](images/huawei-vpc-peering.png)
<!--- Source: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-11646&t=IvETjbByf1MSQzcm-0 --->

The [`internalNetworkSecurity`](cluster_configuration.html#huaweicloudclusterconfiguration-vpcpeering-internalnetworksecurity) parameter (default `true`) enables the creation of a security group (SecurityGroup) when a cluster is created. The module creates the group named after the cluster prefix and assigns it to the nodes.

The following inbound rules will be created:

- allow incoming traffic over the TCP protocol on port `22` from `0.0.0.0/0`
- allow incoming traffic over the ICMP protocol from `0.0.0.0/0`
- allow incoming traffic over the TCP protocol on ports `30000`–`32767` for services of the `NodePort` type. Inbound UDP traffic to `NodePort` ports is not allowed by default

Unlike OpenStack, the module does not add a rule that allows all inbound traffic between nodes in the same security group. By default, only the ports and protocols listed above are open between cluster nodes.

Attach custom security groups for CloudEphemeral nodes in the [HuaweiCloudInstanceClass](cr.html#huaweicloudinstanceclass) resource via [`spec.securityGroups`](cr.html#huaweicloudinstanceclass-v1-spec-securitygroups). They are applied together with the group created by the module.

Example of the layout configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: HuaweiCloudClusterConfiguration
layout: VpcPeering
sshPublicKey: "<SSH_PUBLIC_KEY>"
vpcPeering:
  internalNetworkDNSServers:
    - 8.8.8.8
  internalNetworkCIDR: 10.221.128.0/24
  internalNetworkSecurity: true
  subnet: subnet-43b4
```
