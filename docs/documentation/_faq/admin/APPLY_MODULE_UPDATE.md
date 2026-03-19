---
title: How can I apply an update for a specific module immediately?
lang: en
---

To apply an update for a specific module immediately, set the `modules.deckhouse.io/apply-now: "true"` annotation on the corresponding [ModuleRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#modulerelease) resource.

This annotation applies the release immediately without waiting for the update window. The requirements from [`spec.requirements`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#modulerelease-v1alpha1-spec-requirements) still apply. If they are not met, the release will not be applied.

Example of setting the annotation for the `console` module:

```shell
d8 k annotate mr console-v1.43.3 modules.deckhouse.io/apply-now="true"
```

This can also be done using the [`d8`](/products/kubernetes-platform/documentation/v1/cli/d8/) CLI for convenience (module names and versions are autocompleted):

```shell
d8 system module apply-now console v1.43.3
```

Example resource with the annotation set:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleRelease
metadata:
  name: console-v1.43.3
  annotations:
    modules.deckhouse.io/apply-now: "true"
...
```
