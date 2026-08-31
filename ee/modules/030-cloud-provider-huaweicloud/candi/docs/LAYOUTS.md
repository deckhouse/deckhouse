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

Additionally, you can enable the creation of a security group using the `internalNetworkSecurity` property (default `true`). The group is named after the cluster prefix and is assigned to the nodes.

The following inbound rules will be created:

- TCP/22 (SSH) from `0.0.0.0/0`;
- ICMP from `0.0.0.0/0`;
- TCP NodePorts `30000–32767` from `0.0.0.0/0` (UDP NodePorts are not opened by default).

Unlike OpenStack, the “all inbound traffic from nodes in the same security group” rule is not created by default in Huawei Cloud.

The module does not add HTTP/HTTPS or other application ports to the managed security group. Create such rules manually in a separate security group and attach it to the nodes. For CloudEphemeral nodes, additional security groups are set in `HuaweiCloudInstanceClass` via `spec.securityGroups` and are applied together with the group created by the module.

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

Additionally, you can enable the creation of a security group using the `internalNetworkSecurity` property (default `true`). Default rules match the [Standard](#standard) layout. For CloudEphemeral nodes, additional security groups are set in `HuaweiCloudInstanceClass.spec.securityGroups`.

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
