---
title: "Cloud provider — Yandex.Cloud: usage"
---

Below is an example of the Yandex.Cloud cloud provider configuration.

## An example of the module configuration

```yaml
cloudProviderYandexEnabled: "true"
cloudProviderYandex: |
  additionalExternalNetworkIDs:
  - enp6t4snovl2ko4p15em
```

## An example of the `YandexInstanceClass` custom resource

```yaml
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: test
spec:
  cores: 4
  memory: 8192
```

