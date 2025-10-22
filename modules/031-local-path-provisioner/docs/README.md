---
title: "The local-path-provisioner module"
description: "Management of local storage on Kubernetes nodes in the Deckhouse Kubernetes Platform."
---

Local Path Provisioner provides a way for the Kubernetes users to utilize the local storage on each node.

## How does it work?

For each custom resource [LocalPathProvisioner](cr.html), a corresponding `StorageClass` is created.

The allowed topology for `StorageClass` is calculated based on the list of `nodeGroup` names from the CR.
The topology is used for scheduling Pods.

When a Pod orders a disk:
- a `HostPath` PV is created
- the `Provisioner` creates a local disk folder on the desired node along the path consisting of the `path` custom resource parameter, the PV name and the PVC name.
  
  Example of a path:

  ```shell
  /opt/local-path-provisioner/pvc-d9bd3878-f710-417b-a4b3-38811aa8aac1_d8-monitoring_prometheus-main-db-prometheus-main-0
  ```

## Limitations

- The disk size limit is not supported for the local path provisioned volumes.
