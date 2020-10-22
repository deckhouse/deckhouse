---
title: "Сloud provider — Yandex.Cloud: примеры конфигурации"
---

## Пример конфигурации модуля

```yaml
cloudProviderYandexEnabled: "true"
cloudProviderYandex: |
  additionalExternalNetworkIDs:
  - enp6t4snovl2ko4p15em
```

## Примеры CR `YandexInstanceClass`

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: YandexInstanceClass
metadata:
  name: test
spec:
  cores: 4
  memory: 8192
```

