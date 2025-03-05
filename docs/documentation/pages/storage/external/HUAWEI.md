---
title: "Huawei Storage"
permalink: en/storage/admin/external/huawei.html
---

The module installs and configures the CSI driver for Huawei SAN. The module allows you to create a `StorageClass` in `Kubernetes` by creating [Kubernetes custom resources](./cr.html#yadrostorageclass) `YadroStorageClass`.

> **Caution!** The user is not allowed to create a `StorageClass` for the `csi.huawei.com` CSI driver.
> **Caution!** Currently, supports 3par SAN devices. For other Huawei SAN support please contact tech support.


## System requirements and recommendations

### Requirements

- Presence of a deployed and configured Huawei SAN.
- Unique iqn in /etc/iscsi/initiatorname.iscsi on each of Kubernetes Nodes

## Quickstart guide

Note that all commands must be run on a machine that has administrator access to the Kubernetes API.

### Enabling module

- Enable the `csi-huawei` module. This will result in the following actions across all cluster nodes:
  - registration of the CSI driver;
  - launch of service pods for the `csi-huawei` components.

```shell
kubectl apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-huawei
spec:
  enabled: true
  version: 1
EOF
```

- Wait for the module to become `Ready`.

```shell
kubectl get module csi-huawei -w
```

### Creating a StorageClass

To create a StorageClass, you need to use the [HuaweiStorageClass](./cr.html#huaweistorageclass) and [HuaweiStorageConnection](./cr.html#huaweistorageconnection) resource. Here is an example command to create such a resource:

```yaml
kubectl apply -f -<<EOF
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
kubectl apply -f -<<EOF
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

- You can check objects creation (Phase must be `Created`):

```shell
kubectl get huaweistorageconnections.storage.deckhouse.io <huaweistorageconnection name>
```

```shell
kubectl get huaweistorageclasses.storage.deckhouse.io <huaweistorageclass name>
```

### How to check module health?

To do this, you need to check the status of the pods in the `d8-csi-huawei` namespace. All pods should be in the `Running` or `Completed` state and should be running on all nodes.

```shell
kubectl -n d8-csi-huawei get pod -owide -w
```


