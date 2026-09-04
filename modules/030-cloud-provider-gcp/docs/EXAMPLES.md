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

{% alert %}
Only specific machine types support nested virtualization. See [the GCP documentation](https://cloud.google.com/compute/docs/instances/nested-virtualization/overview#supported_machine_types) for the list of compatible types.
{% endalert %}

```yaml
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  name: vm-nodes
spec:
  machineType: n2-standard-8
  enableNestedVirtualization: true
```

## Adding additional disks

To attach additional disks to instances (for example, for LINSTOR, Ceph, NFS storage nodes, and similar solutions), specify them in the `additionalDisks` parameter:

```yaml
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  name: storage-nodes
spec:
  machineType: n1-standard-8
  additionalDisks:
  - size: 200
    type: pd-ssd
  - size: 500
    type: pd-standard
    autoDelete: true
```
