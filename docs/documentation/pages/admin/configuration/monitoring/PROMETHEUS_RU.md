---
title: "Настройка системы сбора и хранения метрик"
permalink: ru/admin/configuration/monitoring/prometheus.html
description: "Настройка сбора и хранения метрик Prometheus в Deckhouse Kubernetes Platform. Установка Deckhouse Prom++, настройка метрик и управление системой мониторинга."
lang: ru
---

{{< alert level="info" >}}
Начиная с версии 1.71, в Deckhouse Kubernetes Platform используется [Deckhouse Prom++](/products/prompp/) вместо Prometheus.
{{< /alert >}}

## Назначение Prometheus

Prometheus собирает метрики и выполняет правила:

* Для каждой цели мониторинга с заданной периодичностью `scrape_interval` Prometheus выполняет HTTP-запрос, получает в ответ метрики в [собственном формате](https://github.com/prometheus/docs/blob/main/docs/instrumenting/exposition_formats.md) и сохраняет их в свою базу данных.
* Каждый `evaluation_interval` обрабатывает правила Prometheus, на основании чего:
  * отправляет алерты;
  * или сохраняет новые метрики (результат выполнения правил) в свою базу данных.

## Принцип работы Prometheus

Prometheus устанавливается [модулем `prometheus`](/modules/prometheus/) DKP, который выполняет следующие функции:
- определяет следующие кастомные ресурсы:
  - `Prometheus` — определяет инсталляцию (кластер) *Prometheus*.
  - `ServiceMonitor` — определяет, как собирать метрики с сервисов.
  - `Alertmanager` — определяет кластер *Alertmanager*'ов.
  - `PrometheusRule` — определяет список правил Prometheus.
- следит за этими ресурсами, а также:
  - генерирует `StatefulSet` с самим *Prometheus*.
  - создает секреты с необходимыми для работы Prometheus конфигурационными файлами (`prometheus.yaml` — конфигурация Prometheus, и `configmaps.json` — конфигурация для `prometheus-config-reloader`).
  - следит за ресурсами `ServiceMonitor` и `PrometheusRule` и на их основании обновляет конфигурационные файлы *Prometheus* через внесение изменений в секреты.

Включить модуль можно с использованием следующего ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: prometheus
spec:
  version: 2
  enabled: true
  settings:
    auth:
      password: xxxxxx
    retentionDays: 7
    storageClass: rbd
    nodeSelector:
      node-role/monitoring: ""
    tolerations:
    - key: dedicated.deckhouse.io
      operator: Equal
      value: monitoring
```

Параметр `global.modules.storageClass` используется как значение по умолчанию при создании PVC. Если PVC для Prometheus уже существует, его `storageClassName` определяет фактический `storageClass`, поэтому изменение глобального параметра не изменит существующие PVC. Глобальный параметр может повлиять на хранилище при переходе с `emptyDir` на PVC или если PVC еще не создан. Чтобы изменить StorageClass для `prometheus-main` или `prometheus-longterm`, укажите `storageClass` или `longtermStorageClass` соответственно в `ModuleConfig` модуля `prometheus`.

{{< alert level="warning" >}}
При изменении `storageClass` или `longtermStorageClass` в `ModuleConfig` существующий PVC будет удален и создан заново. Перед изменением проверьте политику удаления PersistentVolume и создайте резервную копию данных.
{{< /alert >}}

Полное описание всех настроек доступно [в документации модуля `prometheus`](/modules/prometheus/configuration.html).
