---
title: Storage and load balancing
permalink: en/admin/integrations/private/huaweicloud/storage.html
---

## Storage

Deckhouse Kubernetes Platform provisions disks in Huawei Cloud using the CSI driver.
Storage type is configured via parameters in the HuaweiCloudClusterConfiguration resource,
specifically the [`volumeTypeMap`](/modules/cloud-provider-huaweicloud/cluster_configuration.html#huaweicloudclusterconfiguration-masternodegroup-volumetypemap) field.

Example configuration:

```yaml
masterNodeGroup:
  volumeTypeMap:
    ru-moscow-1a: SSD
```

When the `rootDiskSize` parameter is specified, the same disk type is used for the boot (root) volume.

The following parameters can also be set in the HuaweiCloudInstanceClass:

- [`rootDiskSize`](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass-v1-spec-rootdisksize): Disk size in GiB.
- [`rootDiskType`](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass-v1-spec-rootdisktype): Type of the root disk (SSD, GPSSD, SAS, etc.).

Example:

```yaml
spec:
  rootDiskSize: 50
  rootDiskType: GPSSD
```

It is recommended that you use the fastest available disk types supported in the cloud provider's region.

## Load balancing

Load balancing in Huawei Cloud is handled via Elastic Load Balancer (ELB) services.
To access the load balancer API, the IAM policy must include the [`ELB FullAccess`](./authorization.html) permission.

In the [Standard](./layout.html#standard) layout, the [`enableEIP`](/modules/cloud-provider-huaweicloud/cluster_configuration.html#huaweicloudclusterconfiguration-standard-enableeip) parameter is available to assign Elastic IPs to master nodes.

By default, nodes managed by the Cluster API do not receive public IP addresses.
