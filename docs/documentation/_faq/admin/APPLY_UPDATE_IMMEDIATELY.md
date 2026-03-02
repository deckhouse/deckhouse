---
title: How can I apply an update DKP or module immediately, bypassing update windows, canary releases, and manual update mode ?
subsystems:
  - deckhouse
lang: en
---

To apply a Deckhouse Kubernetes Platform (DKP) update immediately, add the annotation `release.deckhouse.io/apply-now: "true"` to the corresponding [DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease) resource.

This will bypass update windows, [canary release settings](../user/network/canary-deployment.html), and the [manual cluster update mode](../admin/configuration/update/configuration.html#manual-update-approval).
The update will be applied immediately after the annotation is set.

Example command to set the annotation and skip update windows for version `v1.56.2`:

```shell
d8 k annotate deckhousereleases v1.56.2 release.deckhouse.io/apply-now="true"
```

Example of a resource with the annotation set:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  annotations:
    release.deckhouse.io/apply-now: "true"
...
```

#### Forced module update

To apply an update for a specific module immediately, set the `modules.deckhouse.io/apply-now: "true"` annotation on the corresponding [ModuleRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#modulerelease) resource.

This annotation applies the release immediately without waiting for the update window. The requirements from [`spec.requirements`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#modulerelease-v1alpha1-spec-requirements) still apply. If they are not met, the release will not be applied.

Example resource with the annotation set:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleRelease
metadata:
  name: console-0.9.3
  annotations:
    modules.deckhouse.io/apply-now: "true"
...
