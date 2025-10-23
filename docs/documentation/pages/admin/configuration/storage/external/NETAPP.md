---
title: "NetApp data storage"
permalink: en/admin/configuration/storage/external/netapp.html
lang: en
---

Deckhouse Kubernetes Platform (DKP) implements support for NetApp data storage systems for volume management in Kubernetes using CSI driver. This integration provides reliable, scalable, and high-performance storage suitable for critical workloads. For working with NetApp storage systems, the [`csi-netapp` module](/modules/csi-netapp/) is used, which allows creating StorageClass in Kubernetes through creating a [NetappStorageClass](/modules/csi-netapp/cr.html#netappstorageclass) resource.

{% alert level="warning" %}
Creating StorageClass for `csi.netapp.com` CSI driver by users is prohibited.
Currently, the module supports storage systems compatible with [NetApp Trident CSI](https://github.com/NetApp/trident). For support of other NetApp storage systems, please contact [Deckhouse technical support](/tech-support/).
{% endalert %}

This page provides instructions for connecting NetApp to DKP, configuring the connection, and creating StorageClass.

## System Requirements

Before configuring the `csi-netapp` module, ensure the following requirements are met:

- Availability of deployed and configured NetApp storage system.
- Unique IQNs in `/etc/iscsi/initiatorname.iscsi` on each Kubernetes node.

## Configuration

To start working with NetApp storage, enable the [`csi-netapp` module](/modules/csi-netapp/) and configure the connection to the storage system. Execute all commands on a machine with access to Kubernetes API with administrator privileges.

{% alert level="info" %}
To work with snapshots, the [snapshot-controller](../../snapshot-controller/) module must be connected.
{% endalert %}

### Creating StorageClass

Create a StorageClass for working with NetApp volumes. Use [NetappStorageClass](/modules/csi-netapp/cr.html#netappstorageclass) and [NetappStorageConnection](/modules/csi-netapp/cr.html#netappstorageconnection) resources to create StorageClass. Example commands for creating such resources:

1. Create a NetappStorageConnection resource:

   ```shell
   d8 k apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: NetappStorageConnection
   metadata:
     name: netapp
   spec:
     controlPlane:
       backendAddress: "172.17.1.55" # Storage system address (configurable parameter).
       username: "admin" # Username for API access (configurable parameter).
       password: "password" # Password for API access (configurable parameter).
       serviceName: "trident-csp-svc"
       servicePort: "8080"
   EOF
   ```

1. Verify object creation with the following command (`Phase` should be `Created`):

   ```shell
   d8 k get netappstorageconnections.storage.deckhouse.io <netappstorageconnection name>
   ```

1. Create a NetappStorageClass resource:

   ```shell
   d8 k apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: NetappStorageClass
   metadata:
     name: netapp
   spec:
     pool: "test-pool"
     accessProtocol: "fc" # fc or iscsi (default iscsi), immutable parameter.
     fsType: "xfs" # xfs, ext3, ext4 (default ext4), configurable parameter.
     storageConnectionName: "netapp" # Immutable parameter.
     reclaimPolicy: Delete # Delete or Retain.
     cpg: "test-pool"
   EOF
   ```

1. Verify object creation with the following command (`Phase` should be `Created`):

   ```shell
   d8 k get netappstorageclasses.storage.deckhouse.io <netappstorageclass name>
   ```
