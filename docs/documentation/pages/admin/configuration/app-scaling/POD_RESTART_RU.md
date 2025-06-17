---
title: "Перезапуск подов при изменении конфигурации"
permalink: ru/admin/configuration/app-scaling/pod-restart.html
lang: ru
---

Deckhouse Kubernetes Platform поддерживает автоматический rollout (перезапуск с созданием новых реплик) подов при изменении ресурсов ConfigMap и Secret, который работает на системных узлах кластера. Эта возможность реализована на базе [Reloader](https://github.com/stakater/Reloader) и управляется через аннотации, добавляемые к контроллерам рабочих нагрузок (Deployment, DaemonSet, StatefulSet).

> Reloader не предназначен для работы в отказоустойчивом режиме.

Далее описаны основные аннотации, позволяющие контролировать перезапуск подов.

## Поддерживаемые аннотации

| Аннотация | Применяется к | Назначение | Примеры значений |
|----------|----------------|------------|------------------|
| `pod-reloader.deckhouse.io/auto` | Deployment, DaemonSet, StatefulSet | Автоматический перезапуск подов при изменении всех связанных `ConfigMap` и `Secret` (используемых как volume или переменные окружения) | `"true"`, `"false"` |
| `pod-reloader.deckhouse.io/search` | Deployment, DaemonSet, StatefulSet | Перезапуск только при изменении ресурсов с аннотацией `match: "true"` | `"true"`, `"false"` |
| `pod-reloader.deckhouse.io/configmap-reload` | Deployment, DaemonSet, StatefulSet | Указание конкретных `ConfigMap`, при изменении которых должен выполняться перезапуск | `"some-cm"`, `"some-cm1,some-cm2"` |
| `pod-reloader.deckhouse.io/secret-reload` | Deployment, DaemonSet, StatefulSet | Указание конкретных `Secret`, при изменении которых должен выполняться перезапуск | `"some-secret"`, `"some-secret1,some-secret2"` |
| `pod-reloader.deckhouse.io/match` | ConfigMap, Secret | Помечает ресурсы, изменения которых отслеживаются при использовании аннотации `search: "true"` | `"true"`, `"false"` |

> Аннотация `search` не должна использоваться совместно с `auto: "true"`. В этом случае аннотации `search` и `match` будут проигнорированы. Для корректной работы установите `auto: "false"` или удалите её.
>
> Аннотации `configmap-reload` и `secret-reload` не работают при наличии `auto: "true"`. Для корректной работы отключите `auto`.

## Примеры использования

### Слежение за всеми изменениями во всех подключенных ресурсах: смонтированных как volume или используемых в переменных окружения

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
  annotations:
    pod-reloader.deckhouse.io/auto: "true"
spec:
  template:
    spec:
      containers:
        - name: nginx
          env:
            - name: SECRET_WORD
              valueFrom:
                secretKeyRef:
                  name: nginx-secret-value
                  key: extra
          volumeMounts:
            - name: pages
              mountPath: "/usr/share/nginx/pages"
      volumes:
        - name: pages
          configMap:
            name: nginx-pages
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: nginx-secret-value
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: nginx-pages
```

### Слежение за изменениями только в конкретных ресурсах

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  annotations:
    pod-reloader.deckhouse.io/search: "true"
spec:
  template:
    spec:
      containers:
        - name: nginx
          env:
            - name: SECRET_WORD
              valueFrom:
                secretKeyRef:
                  name: nginx-secret-value
                  key: extra
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: nginx-secret-value
  annotations:
    pod-reloader.deckhouse.io/match: "true"
```

### Слежение за изменениями в ресурсах из списка

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  annotations:
    pod-reloader.deckhouse.io/configmap-reload: "nginx-config,nginx-pages"
spec:
  template:
    spec:
      containers:
        - name: nginx
          volumeMounts:
            - name: pages
              mountPath: "/usr/share/nginx/pages"
            - name: config
              mountPath: "/etc/nginx/templates"
      volumes:
        - name: pages
          configMap:
            name: nginx-pages
        - name: config
          configMap:
            name: nginx-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: nginx-pages
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: nginx-config
```

## Как включить или отключить перезапуск подов

Включить или отключить перезапуск подов можно следующими способами:

1. Через ресурс ModuleConfig (например, ModuleConfig/pod-reloader). Установите параметр `spec.enabled` в значение `true` или `false`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: pod-reloader
   spec:
     enabled: true
   ```

1. Через команду `d8` (в поде `d8-system/deckhouse`):

   ```console
   d8 platform module enable pod-reloader
   ```

1. Через [веб-интерфейс Deckhouse](https://deckhouse.ru/products/kubernetes-platform/modules/console/stable/):

   - Перейдите в раздел «Deckhouse - «Модули»;
   - Найдите модуль `pod-reloader` и нажмите на него;
   - Включите тумблер «Модуль включен».

## Настройка

Механизм перезапуска подов работает «из коробки» и не требует обязательной конфигурации. По умолчанию он включён в наборах модулей Default и Managed и отключён в наборе Minimal.

При необходимости его поведение можно изменить через ресурс ModuleConfig.

Доступные параметры:

| Параметр        | Тип      | Описание                                                                 | По умолчанию |
|----------------|----------|--------------------------------------------------------------------------|--------------|
| `reloadOnCreate` | boolean  | Перезапуск при создании ConfigMap или Secret, а не только при изменении | `true`       |
| `nodeSelector`   | object   | Ограничение на узлы для запуска компонента (аналог `spec.nodeSelector`)     | Не задан     |
| `tolerations`    | array    | Допуски к размещению на узлах с `taint` (аналог `spec.tolerations`)         | Не заданы    |
