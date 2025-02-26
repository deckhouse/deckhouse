---
title: "Cloud provider - HuaweiCloud: Layouts"
description: "Schemes of placement and interaction of resources in HuaweiCloud when working with the Deckhouse cloud provider."
---

## Standard

* An internal cluster network is created with a gateway to the public network.
* The elastic IP can be assigned to the master node.
* Nodes managed by the Cluster API do not have public IP addresses.

![Standard layout](../../images/cloud-provider-huawei/huawei-standard.png)
<!--- Source: https://docs.google.com/drawings/d/1sB_V7NhDiit8Gok2pq_8syQknCdC4GicpG3L2YF5QIU/edit --->

Example of the layout configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: HuaweiCloudClusterConfiguration
layout: Standard
sshPublicKey: "<Public SSH key>"
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

![VpcPeering layout](../../images/cloud-provider-huawei/huawei-vpc-peering.png)

Example of the layout configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: HuaweiCloudClusterConfiguration
layout: VpcPeering
sshPublicKey: "<Public SSH key>"
vpcPeering:
  internalNetworkDNSServers:
    - 8.8.8.8
  internalNetworkCIDR: 10.221.128.0/24
  internalNetworkSecurity: true
  subnet: subnet-43b4
```
