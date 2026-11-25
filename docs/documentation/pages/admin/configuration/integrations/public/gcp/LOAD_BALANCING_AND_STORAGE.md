---
title: Storage in Google Cloud Platform
permalink: en/admin/integrations/public/gcp/load-balancing-and-storage.html
lang: en
---

## Storage

Deckhouse Platform automatically creates StorageClasses that cover all available disk types in GCP:

| Disk type | Replication | StorageClass name |
|---|---|---|
| standard | none | `pd-standard-not-replicated` |
| standard | regional | `pd-standard-replicated` |
| balanced | none | `pd-balanced-not-replicated` |
| balanced | regional | `pd-balanced-replicated` |
| ssd | none | `pd-ssd-not-replicated` |
| ssd | regional | `pd-ssd-replicated` |

You can exclude unnecessary StorageClasses by specifying them in the [`exclude`](/modules/cloud-provider-gcp/configuration.html#parameters-storageclass-exclude) parameter.

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
  - sizeGb: 200
    type: pd-ssd
  - sizeGb: 500
    type: pd-standard
    autoDelete: true
```
