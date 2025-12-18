---
title: Layouts and configuration
permalink: en/admin/integrations/private/huaweicloud/layout.html
---

This section describes the cluster layouts in Huawei Cloud infrastructure and the associated parameters.

## Standard

- An internal cluster network is created with a gateway to the public network.
- An Elastic IP can be assigned to the master node.
- Nodes managed by the Cluster API do not have public IP addresses.

![Standard layout in Huawei CLoud](../../../../images/cloud-provider-huawei/huawei-standard.png)
<!--- Source: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10811&t=Qb5yyWumzPiTBtfL-0 --->

Example configuration:

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

- An existing VPC network and subnet are used.
- Virtual machines are connected to the specified subnet.

![VpcPeering layout in Huawei Cloud](../../../../images/cloud-provider-huawei/huawei-vpc-peering-ru.png)
<!--- Source: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-11715&t=Qb5yyWumzPiTBtfL-0 --->

Example configuration:

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
