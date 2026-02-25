---
title: How can I apply an update immediately, bypassing update windows, canary releases, and manual update mode ?
subsystems:
  - deckhouse
lang: en
---

To apply a Deckhouse Kubernetes Platform (DKP) update immediately,
add the annotation `release.deckhouse.io/apply-now: "true"` to the corresponding [DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease) resource.

This will bypass update windows, [canary release settings](../../../user/network/canary-deployment.html), and the [manual cluster update mode](configuration.html#manual-update-approval).
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
