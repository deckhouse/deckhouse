---
title: "Перезапуск подов при изменении конфигурации"
permalink: ru/admin/configuration/app-scaling/pod-restart.html
description: "Настройка автоматического перезапуска подов при изменении конфигурации в Deckhouse Kubernetes Platform. Интеграция Pod reloader для обновлений ConfigMap и Secret с автоматизацией перезапуска подов."
lang: ru
---

Deckhouse Kubernetes Platform может автоматически перезапускать поды при изменении определенных ресурсов ConfigMap и Secret. Эта возможность реализована на базе проекта [Reloader](https://github.com/stakater/Reloader) и управляется через аннотации, добавляемые к контроллерам подов (Deployment, DaemonSet, StatefulSet).

{% alert %}
Reloader не предназначен для работы в отказоустойчивом режиме.
{% endalert %}

Далее описаны основные аннотации, позволяющие контролировать перезапуск подов.

## Поддерживаемые аннотации

| Аннотация | Применяется к объектам | Назначение | Примеры значений |
|----------|----------------|------------|------------------|
| `pod-reloader.deckhouse.io/auto` | Deployment, DaemonSet, StatefulSet | Автоматический перезапуск подов при изменении всех связанных с ним ConfigMap и Secret (используемых как volume или переменные окружения) | `"true"`, `"false"` |
| `pod-reloader.deckhouse.io/search` | Deployment, DaemonSet, StatefulSet | Перезапуск только при изменении ресурсов с аннотацией `pod-reloader.deckhouse.io/match: "true"` | `"true"`, `"false"` |
| `pod-reloader.deckhouse.io/configmap-reload` | Deployment, DaemonSet, StatefulSet | Указание конкретных `ConfigMap`, при изменении которых должен выполняться перезапуск | `"some-cm"`, `"some-cm1,some-cm2"` |
| `pod-reloader.deckhouse.io/secret-reload` | Deployment, DaemonSet, StatefulSet | Указание конкретных `Secret`, при изменении которых должен выполняться перезапуск | `"some-secret"`, `"some-secret1,some-secret2"` |
| `pod-reloader.deckhouse.io/match` | ConfigMap, Secret | Помечает ресурсы, изменения которых отслеживаются при использовании аннотации `pod-reloader.deckhouse.io/search`: `"true"` | `"true"`, `"false"` |

{% alert level="warning"%}
Аннотация `pod-reloader.deckhouse.io/search` не должна использоваться совместно с `pod-reloader.deckhouse.io/auto: "true"`. В этом случае аннотации `pod-reloader.deckhouse.io/search` и `pod-reloader.deckhouse.io/match` будут проигнорированы. Для корректной работы установите `pod-reloader.deckhouse.io/auto: "false"` или удалите её.

Аннотации `pod-reloader.deckhouse.io/configmap-reload` и `pod-reloader.deckhouse.io/secret-reload` не работают при наличии `pod-reloader.deckhouse.io/auto: "true"`. Для корректной работы отключите `auto`.
{% endalert %}

## Примеры использования

### Слежение за всеми изменениями во всех подключенных ресурсах

Подключённые ресурсы могут быть использованы как в переменных окружения, так и смонтированы как тома (volumes).

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

Указывает, что нужно отслеживать только те секреты или ConfigMap'ы, которые имеют аннотацию `pod-reloader.deckhouse.io/match: "true"`. Это удобно, если в поде используется множество ресурсов, но перезапуск требуется только при изменении определённых.

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

Явное указание списка ConfigMap, при изменении которых должен перезапускаться под.

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

1. Через [веб-интерфейс Deckhouse](/modules/console/):

   - Перейдите в раздел «Deckhouse - «Модули»;
   - Найдите модуль `pod-reloader` и нажмите на него;
   - Включите тумблер «Модуль включен».

## Настройка

Механизм перезапуска подов работает «из коробки» и не требует обязательной конфигурации. По умолчанию он включён в наборах модулей Default и Managed и отключён в наборе Minimal.

При необходимости его поведение можно изменить в настройках модуля [pod-reloader](/modules/pod-reloader/) (ModuleConfig `pod-reloader`).

Доступные параметры:

| Параметр        | Тип      | Описание                                                                 |  Значение по&nbsp;умолчанию |
|----------------|----------|--------------------------------------------------------------------------|--------------|
| `reloadOnCreate` | boolean  | Перезапуск при создании ConfigMap или Secret, а не только при изменении | `true`       |
| `nodeSelector`   | object   | Ограничение на узлы для запуска компонента (аналог `spec.nodeSelector`)     | Не задан     |
| `tolerations`    | array    | Допуски к размещению на узлах с `taint` (аналог `spec.tolerations`)         | Не заданы    |
