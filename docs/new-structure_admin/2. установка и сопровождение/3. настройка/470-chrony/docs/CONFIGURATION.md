---
title: "The chrony module: configuration"
---

<!-- SCHEMA -->

## An example of the configuration

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: chrony
spec:
  settings:
    ntpServers:
      - pool.ntp.org
      - ntp.ubuntu.com
  version: 1
```
