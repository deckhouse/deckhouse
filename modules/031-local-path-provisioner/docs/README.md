---
title: "The local-path-provisioner module"
---

Local Path Provisioner provides a way for the Kubernetes users to utilize the local storage on each node.

## How does it work?
For each CR ```LocalPathProvisioner```, a corresponding ```StorageClass``` is created.

The allowed topology for SC is calculated based on the list of nodegroup names from CR.
The topology is used for scheduling pods.
When the pod orders a disk, a ```HostPath``` PV is created, and the ```Provisioner``` creates a local disk folder on the desired node along the path consisting
of the ```path``` CR parameter, the PV name and the PVC name
(for example, ```/opt/local-path-provisioner/pvc-d9bd3878-f710-417b-a4b3-38811aa8aac1_d8-monitoring_prometheus-main-db-prometheus-main-0```).

## Limitations
The disk size limit is not supported for the local path provisioned volumes.
