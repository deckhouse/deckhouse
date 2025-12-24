---
title: "NetApp data storage"
permalink: en/admin/configuration/storage/external/netapp.html
---

Deckhouse Kubernetes Platform (DKP) implements support for NetApp data storage systems for volume management in Kubernetes using CSI driver. This integration provides reliable, scalable, and high-performance storage suitable for critical workloads. For working with NetApp storage systems, the [`csi-netapp` module](/modules/csi-netapp/) is used, which allows creating StorageClass in Kubernetes through creating a [NetappStorageClass](/modules/csi-netapp/cr.html#netappstorageclass) resource.

{% alert level="warning" %}
Creating StorageClass for `csi.netapp.com` CSI driver by users is prohibited.
Currently, the module supports storage systems compatible with [NetApp Trident CSI](https://github.com/NetApp/trident). For support of other NetApp storage systems, please contact [Deckhouse technical support](/tech-support/).
{% endalert %}

This page provides instructions for connecting NetApp to DKP, configuring the connection, and creating StorageClass.

## System Requirements

Before configuring work with the NetApp storage system, make sure that the following requirements are met:

- Availability of deployed and configured NetApp storage system.
- Unique IQNs in `/etc/iscsi/initiatorname.iscsi` on each Kubernetes node.

## Configuring cluster integration with the NetApp storage system

To start working with the NetApp storage system, follow the step-by-step instructions below. Run all commands on a machine with administrative access to the Kubernetes API.

{% alert level="info" %}
To work with snapshots, the [snapshot-controller](/modules/snapshot-controller/) module must be connected.
{% endalert %}

1. Execute the command to activate the `csi-netapp` module.

   ```shell
   d8 system module enable csi-netapp
   ```

   After activation, the following will be deployed on all cluster nodes:

   - CSI driver will be registered.
   - Service pods of `csi-netapp` components will be deployed.

1. Wait for the module to transition to the `Ready` state:

   ```shell
   d8 k get module csi-netapp -w
   ```

1. Ensure that all pods in the `d8-csi-netapp` namespace are in the `Running` or `Completed` state and deployed on all cluster nodes:

   ```shell
   d8 k -n d8-csi-netapp get pod -owide -w
   ```

1. Create a [NetappStorageConnection](/modules/csi-netapp/cr.html#netappstorageconnection) resource:

   ```shell
   d8 k apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: NetappStorageConnection
   metadata:
     name: netapp
   spec:
     controlPlane:
       address: "172.17.1.55"
       username: "admin"
       password: "password"
       svm: "svm1"
   EOF
   ```

1. Verify object creation with the following command (`Phase` should be `Created`):

   ```shell
   d8 k get netappstorageconnections.storage.deckhouse.io <netappstorageconnection name>
   ```

1. Create a [NetappStorageClass](/modules/csi-netapp/cr.html#netappstorageclass) resource:

   ```shell
   d8 k apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: NetappStorageClass
   metadata:
     name: netapp
   spec:
     pool: "test-cpg"
     accessProtocol: "iscsi" # fc or iscsi (default iscsi), immutable parameter.
     fsType: "xfs" # xfs, ext3, ext4 (default ext4), configurable parameter.
     storageConnectionName: "netapp" # Immutable parameter.
     reclaimPolicy: Delete # Delete or Retain.
     cpg: "test-cpg"
   EOF
   ```

1. Verify object creation with the following command (`Phase` should be `Created`):

   ```shell
   d8 k get netappstorageclasses.storage.deckhouse.io <netappstorageclass name>
   ```

The NetApp storage system is now ready for use. You can use the created StorageClass to create PersistentVolumeClaims in your applications.
