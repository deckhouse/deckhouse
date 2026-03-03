---
title: Как принудительно обновить модуль?
lang: ru
---

Чтобы принудительно применить обновление конкретного модуля, установите аннотацию `modules.deckhouse.io/apply-now: "true"` в соответствующем ресурсе [ModuleRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#modulerelease).

Аннотация применяет релиз немедленно, не дожидаясь окна обновлений. Требования из [`spec.requirements`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#modulerelease-v1alpha1-spec-requirements) при этом сохраняются — если они не выполняются, релиз не будет применен.

Пример установки аннотации для модуля `console`:

```shell
d8 k annotate mr console-v1.43.3 modules.deckhouse.io/apply-now: "true"
```

Пример ресурса с установленной аннотацией:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleRelease
metadata:
  name: console-v1.43.3
  annotations:
    modules.deckhouse.io/apply-now: "true"
...
```
