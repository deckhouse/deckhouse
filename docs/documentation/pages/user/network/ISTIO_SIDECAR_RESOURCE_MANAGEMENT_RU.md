---
title: "Настройка ресурсов для istio-proxy сайдкаров"
permalink: ru/user/network/istio-sidecar-resource-management.html
lang: ru
---

При использовании модуля [istio](/modules/istio/) в кластере вы можете управлять ресурсами, выделяемыми для istio-proxy сайдкаров в отдельных рабочих нагрузках. Для этого используются аннотации.

## Поддерживаемые аннотации

Для переопределения глобальных ограничений ресурсов для istio-proxy сайдкаров в отдельных рабочих нагрузках поддерживаются аннотации:

| Аннотация                           | Описание                     | Пример значения |
|-------------------------------------|------------------------------|-----------------|
| `sidecar.istio.io/proxyCPU`         | Запрос CPU для сайдкара      | `200m`          |
| `sidecar.istio.io/proxyCPULimit`    | Лимит CPU для сайдкара       | `"1"`           |
| `sidecar.istio.io/proxyMemory`      | Запрос памяти для сайдкара   | `128Mi`         |
| `sidecar.istio.io/proxyMemoryLimit` | Лимит памяти для сайдкара    | `512Mi`         |

{% alert level="warning" %}
Все аннотации из таблицы должны быть указаны в манифесте рабочей нагрузки одновременно. Частичная конфигурация не поддерживается.
{% endalert %}

## Примеры конфигурации

Для Deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
# ...
spec:
  template:
    metadata:
      annotations:
        sidecar.istio.io/proxyCPU: 200m
        sidecar.istio.io/proxyCPULimit: "1"
        sidecar.istio.io/proxyMemory: 128Mi
        sidecar.istio.io/proxyMemoryLimit: 512Mi
# ... остальная часть манифеста
```

Для ReplicaSet:

```yaml
apiVersion: apps/v1
kind: ReplicaSet
metadata:
# ...
spec:
  template:
    metadata:
      annotations:
        sidecar.istio.io/proxyCPU: 200m
        sidecar.istio.io/proxyCPULimit: "1"
        sidecar.istio.io/proxyMemory: 128Mi
        sidecar.istio.io/proxyMemoryLimit: 512Mi
# ... остальная часть манифеста
```

Для Pod:

```yaml
apiVersion: v1
kind: Pod
metadata:
  annotations:
    sidecar.istio.io/proxyCPU: 200m
    sidecar.istio.io/proxyCPULimit: "1"
    sidecar.istio.io/proxyMemory: 128Mi
    sidecar.istio.io/proxyMemoryLimit: 512Mi
# ... остальная часть манифеста
```
