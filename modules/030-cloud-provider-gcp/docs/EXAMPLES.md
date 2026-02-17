---
title: "Cloud provider — GCP: examples"
---

## An example of the `GCPInstanceClass` custom resource

Below is a simple example of custom resource `GCPInstanceClass` configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  name: test
spec:
  machineType: n1-standard-1
```

## Enabling nested virtualization

To run virtual machine workloads (e.g., KVM-based VMs) inside GCP instances, enable nested virtualization.

> **Note.** Only supported on specific machine types. See the [GCP documentation](https://cloud.google.com/compute/docs/instances/nested-virtualization/overview#supported_machine_types) for the list of compatible types.

```yaml
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  name: vm-nodes
spec:
  machineType: n2-standard-8
  nestedVirtualization: true
```

## Adding additional disks

To attach extra disks to instances (e.g., for LinStor, Ceph, NFS storage nodes):

```yaml
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  name: storage-nodes
spec:
  machineType: n1-standard-8
  additionalDisks:
  - sizeGb: 200
    type: pd-ssd
  - sizeGb: 500
    type: pd-standard
    autoDelete: true
```

## Configuring security policies on nodes

For cluster virtual machines in GCP, you may need to limit or expand incoming and outgoing traffic for various reasons. Some of these may include:

- Allowing connections to cluster nodes from virtual machines from a different subnet.
- Allow connections to static node ports for application operation.
- Restricting access to external resources or other virtual machines in the cloud at the request of the security service.

For all this, additional network tags must be used.

## Enabling additional network tags on static and master nodes

This parameter can be set either in an existing cluster or when creating one. In both cases, additional network tags are declared in the `GCPClusterConfiguration`:

- for master nodes, in the `additionalNetworkTags` field of the `masterNodeGroup` section;
- for static nodes, in the `additionalNetworkTags` field of the `nodeGroups` subsection that corresponds to the target nodeGroup.

The `additionalNetworkTags` field contains an array of strings with network tags names.

## Enabling additional network tags on ephemeral nodes

You have to set the `additionalNetworkTags` parameter for all [`GCPInstanceClass`](cr.html#gcpinstanceclass) that require additional network tags.
