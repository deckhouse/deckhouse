---
title: "Cloud provider — Yandex.Cloud: примеры конфигурации"
---

Ниже представлен пример конфигурации cloud-провайдера Yandex.Cloud.

## Пример конфигурации модуля

```yaml
cloudProviderYandexEnabled: "true"
cloudProviderYandex: |
  additionalExternalNetworkIDs:
  - enp6t4snovl2ko4p15em
```

## Пример custom resource `YandexInstanceClass`

```yaml
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: test
spec:
  cores: 4
  memory: 8192
```

