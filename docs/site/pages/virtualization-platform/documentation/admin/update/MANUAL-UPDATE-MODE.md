---
title: "Manual update mode"
permalink: en/virtualization-platform/documentation/admin/update/manual-update-mode.html
---

For manual update confirmation, set the corresponding mode in the configuration:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
    update:
      mode: Manual
```

In this mode, you have to manually confirm each minor update of the platform (excluding patch versions).

Example command to confirm an update to `v1.43.2`:

```shell
d8 k patch DeckhouseRelease v1.43.2 --type=merge -p='{"approved": true}'
```
