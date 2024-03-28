---
title: "The Prometheus monitoring module: configuration"
type:
  — instruction
search: prometheus
---

The module does not require any configuration – it works right out-of-the-box.

<!-- SCHEMA -->

## Authentication

[user-authn](/documentation/v1/modules/150-user-authn/) module provides authentication by default. Also, externalAuthentication can be configured (see below).
If these options are disabled, the module will use basic auth with the auto-generated password and the user `admin`.

Use kubectl to see password:

```shell
kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module values prometheus -o json | jq '.prometheus.internal.auth.password'
```

Delete the Secret to re-generate password:

```shell
kubectl -n d8-monitoring delete secret/basic-auth
```

> **Note!** The `auth.password` parameter is deprecated.

## Notes

* `retentionSize` for the `main` and `longterm` Prometheus is **calculated automatically; you cannot set this value manually!**
  * The following calculation algorithm is used:
    * `pvc_size * 0.85` — if the PVC exists;
    * `10 GiB` — if there is no PVC and if the StorageClass supports resizing;
    * `25 GiB` — if there is no PVC and if the StorageClass does not support resizing;
  * If the `local-storage` is used, and you have to change the `retentionSize`, then you need to manually change the size of the PV and PVC. **Caution!** Note that the value from `.status.capacity.storage` PVC is used for the calculation since it reflects the actual size of the PV in the case of manual resizing.
* You can change the size of Prometheus disks in the standard Kubernetes way (if the StorageClass permits this) by editing the `.spec.resources.requests.storage` field of the PersistentVolumeClaim resource.
