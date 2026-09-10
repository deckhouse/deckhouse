---
title: "Cloud provider — Basis Dynamix: FAQ"
---

## How to configure a LoadBalancer?

o configure a Service of the LoadBalancer type, add the following annotations to the Service manifest:

```yaml
metadata:
  annotations:
    dynamix.cpi.flant.com/internal-network-name: <internal_name>
    dynamix.cpi.flant.com/external-network-name: <external_name>
```

Both annotations are required:

- `dynamix.cpi.flant.com/internal-network-name` — the name of the internal network in Basis Dynamix
- `dynamix.cpi.flant.com/external-network-name` — the name of the external network in Basis Dynamix

The terms "internal network" and "external network" are used in the context of Basis Dynamix. The external network does not have to be public and may use private IP addresses.

If one of the annotations is not specified, cloud-controller-manager will fail to process the Service.

## Why did a storage class appear for a storage endpoint from another location?

Storage classes are generated from the storage endpoints that `cloud-data-discoverer` receives from the `/restmachine/cloudapi/sep/listAvailableSepAndPools` endpoint. This endpoint scopes its answer to the account and does not report the location of an endpoint, so the discoverer cannot filter the result by the `location` set in `DynamixClusterConfiguration`.

If the account is present in more than one location, storage endpoints from the other locations will also produce storage classes, and a `PersistentVolumeClaim` created on such a class will not be provisioned. Use only the storage classes that belong to the location specified in `DynamixClusterConfiguration`.
