---
title: "The csi-vsphere module"
description: "CSI vSphere Driver for provisioning disks in static clusters based on VMware vSphere."
---

## Description

The `csi-vsphere` module provides Container Storage Interface (CSI) support for VMware vSphere environments, enabling dynamic provisioning and management of persistent storage volumes in Kubernetes clusters running on vSphere infrastructure.

This module is specifically designed for **static Kubernetes clusters** (non-cloud) deployed on VMware vSphere and works independently from the [cloud-provider-vsphere](../cloud-provider-vsphere/) module. It integrates with vSphere datastores to provide persistent storage capabilities through the vSphere CSI driver.

The module deploys the VMware vSphere CSI driver components that:

- **Automatically discover vSphere datastores** — Reads discovery data from the `d8-cloud-provider-discovery-data` secret to identify available datastores and their topology
- **Create StorageClasses** — Automatically generates StorageClass resources for each discovered datastore, filtered by the exclude list if configured
- **Provision persistent volumes** — Dynamically creates persistent volumes on vSphere datastores when PersistentVolumeClaims are created
- **Support volume operations** — Handles volume creation, deletion, attachment, detachment, and online resizing (in modern mode)
- **Implement topology awareness** — Uses zone and region labels for proper volume placement according to vSphere cluster topology

### Architecture

The CSI driver consists of two main components:

1. **CSI Controller** (runs on master nodes):
   - **vsphere-csi-controller** — Main controller managing volume lifecycle operations
   - **csi-provisioner** — Watches for PVCs and triggers volume creation
   - **csi-attacher** — Handles volume attachment to nodes
   - **csi-resizer** — Manages online volume expansion
   - **vsphere-syncer** — Synchronizes metadata between Kubernetes and vSphere every 30 minutes
   - **liveness-probe** — Monitors controller health

1. **CSI Node** (DaemonSet on all nodes):
   - **vsphere-csi-node** — Node-level CSI driver handling volume mounting/unmounting
   - **node-driver-registrar** — Registers the CSI driver with kubelet
   - **liveness-probe** — Monitors node driver health

### Compatibility modes

The module supports two operational modes via the [compatibilityFlag](configuration.html#parameters-storageclass-compatibilityflag) parameter:

- **Modern mode** (default, `compatibilityFlag` not set or empty):
  - Uses the current vSphere CSI driver (`csi.vsphere.vmware.com`)
  - Supports CNS (Cloud Native Storage) volumes
  - Enables online volume resizing
  - Recommended for new deployments

- **Legacy mode** (`compatibilityFlag: "Legacy"`):
  - Uses the older vSphere CSI driver (`vsphere.csi.vmware.com`)
  - Works with FCD (First Class Disk) volumes only
  - No online volume resizing support
  - Used for backward compatibility

- **Migration mode** (`compatibilityFlag: "Migration"`):
  - Runs both driver versions simultaneously
  - Facilitates migration from Legacy to Modern mode
  - Allows gradual transition of workloads

### Storage discovery and provisioning

The module automatically:

1. **Discovers datastores** from the cloud provider discovery data
1. **Generates StorageClass names** by sanitizing datastore names to meet Kubernetes DNS naming requirements
1. **Filters excluded storage** based on [storageClass.exclude](configuration.html#parameters-storageclass-exclude) configuration (supports regex patterns)
1. **Creates StorageClasses** with appropriate parameters:
   - `allowVolumeExpansion: true` (modern mode only)
   - `volumeBindingMode: WaitForFirstConsumer` — Ensures volumes are created in the same zone as the pod
   - Topology constraints matching vSphere regions and zones

### Volume lifecycle

When a pod requests storage:

1. A PVC is created referencing a StorageClass managed by this module
1. The CSI provisioner creates a volume on the specified vSphere datastore
1. The CSI attacher attaches the volume to the appropriate node
1. The CSI node driver mounts the volume into the pod
1. The vsphere-syncer periodically synchronizes metadata to ensure consistency


## Limitations

- This module is designed exclusively for static (non-cloud) Kubernetes clusters on vSphere
- [Legacy mode](#compatibility-modes) does not support online volume resizing
- Volume expansion requires the modern CSI driver
- StorageClasses are automatically managed — manual modifications may be overwritten
