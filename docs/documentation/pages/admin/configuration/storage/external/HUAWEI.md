---
title: "Huawei data storage"
permalink: en/storage/admin/external/huawei.html
---

Deckhouse provides support for Huawei Dorado storage systems, enabling volume management in Kubernetes using a CSI driver through the creation of custom resources like [HuaweiStorageClass](../../../reference/cr/huaweistorageclass/). This solution ensures high-performance and fault-tolerant storage, making it an optimal choice for mission-critical workloads.

{% alert level="warning" %}
User-created StorageClass for the `csi.huawei.com` CSI driver is not allowed.  
Only Huawei Dorado storage systems are supported. For other Huawei storage systems, please contact technical support.
{% endalert %}

This page provides instructions on connecting Huawei Dorado to Deckhouse, configuring the connection, creating StorageClass, and verifying storage functionality.

## System requirements

- Presence of a deployed and configured Huawei SAN.
- Unique IQNs in `/etc/iscsi/initiatorname.iscsi` on each Kubernetes Nodes.

## Setup and Configuration

Note that all commands must be run on a machine that has administrator access to the Kubernetes API.

### Enabling the module

To support Huawei Dorado storage systems, enable the `csi-huawei` module. This will ensure that all cluster nodes have:
- registration of the CSI driver.
- launch of service pods for the `csi-huawei` components.

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-huawei
spec:
  enabled: true
  version: 1
EOF
```

Wait for the module to become `Ready`.

```shell
d8 k get module csi-huawei -w
```

### Creating a StorageClass

To create a StorageClass, you need to use the [HuaweiStorageClass](../../../reference/cr/huaweistorageclass/) and [HuaweiStorageConnection](../../../reference/cr/huaweistorageconnection/) resource. Here is an example command to create such a resource:

```yaml
d8 k apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: HuaweiStorageConnection
metadata:
  name: huaweistorageconn
spec:
  storageType: OceanStorSAN
  pools:
    - test
  urls: 
    - https://192.168.128.101:8088 
  login: "admin"
  password: "ivkerg43grdsf_"
  protocol: ISCSI
  portals:
    - 10.240.0.101
    - 10.250.0.101 
  maxClientThreads: 30

EOF
```

```yaml
d8 k apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: HuaweiStorageClass
metadata:
  name: huaweisc
spec:
  fsType: ext4
  pool: test
  reclaimPolicy: Delete
  storageConnectionName: huaweistorageconn
  volumeBindingMode: WaitForFirstConsumer
EOF
```

You can check objects creation (`Phase` must be `Created`):

```shell
d8 k get huaweistorageconnections.storage.deckhouse.io <huaweistorageconnection name>
```

```shell
d8 k get huaweistorageclasses.storage.deckhouse.io <huaweistorageclass name>
```

### How to check module health?

To do this, you need to check the status of the pods in the `d8-csi-huawei` namespace. All pods should be in the `Running` or `Completed` state and should be running on all nodes.

```shell
d8 k -n d8-csi-huawei get pod -owide -w
```
