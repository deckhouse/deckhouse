---
title: События безопасности
permalink: ru/admin/configuration/security/events/security-events.html
description: "Настройка сбора, обработки и доставки событий безопасности в Deckhouse Kubernetes Platform. Единый контур событий безопасности из логов приложений и инфраструктурных компонентов Kubernetes."
lang: ru
---

Deckhouse Kubernetes Platform (DKP) предоставляет средства для декларативного сбора, обработки,
нормализации и доставки событий безопасности, извлекаемых из логов приложений
и инфраструктурных компонентов Kubernetes.

Событие безопасности — это структурированная запись о значимом с точки зрения информационной безопасности (ИБ)
действии или факте. DKP позволяет:

- собирать события безопасности из различных источников (логи подов, файлы узлов, аудит Kubernetes API);
- нормализовать события к единому формату с обязательным минимумом атрибутов;
- обогащать события контекстными данными;
- фильтровать события по источникам и уровню критичности;
- доставлять события в системы хранения и аналитики (Loki, Elasticsearch, Kafka, Splunk, Vector и др.).

## Какой модуль отвечает за события безопасности

За сбор, обработку и доставку событий безопасности отвечает модуль [`security-events-manager`](/modules/security-events-manager/).
Этот модуль использует вспомогательный модуль [`log-shipper`](/modules/log-shipper/) для сбора логов.

## Зависимости и требования

Для работы модуля `security-events-manager` требуются следующие модули DKP:

- [`log-shipper`](/modules/log-shipper/) — обеспечивает сбор логов из подов и узловых файлов, выполняет предварительный отбор записей;
- [`loki`](/modules/loki/) — обеспечивает хранение событий безопасности внутри кластера (используется по умолчанию в качестве приёмника).

Подробности об архитектуре работы с событиями безопасности можно найти [в разделе Архитектура](../../../../architecture/security/security-events.html).

## Источники данных для событий безопасности

Модуль `security-events-manager` собирает данные из двух типов источников:

- **Контейнерные источники** — логи приложений в подах Kubernetes. Сбор выполняется через модуль `log-shipper`, который отбирает записи по меткам подов и неймспейсам.
- **Кластерные источники** — файлы узлов и системных сервисов, не привязанные к конкретному неймспейсу. Например:
  - `/var/log/kube-audit/audit.log` — логи аудита Kubernetes API;
  - `/var/log/auth.log` — системные события аутентификации узла.

Применяется двухуровневая схема сбора:

1. Модуль `log-shipper` выполняет предварительный отбор логовых записей с помощью простых операций сравнения (`In`, `NotIn`, `Regex`, `NotRegex`, `Exists`, `DoesNotExist`) и передаёт их в шлюз `security-events-manager`.
1. Шлюз `security-events-manager` выполняет распознавание полей (парсинг), обработку, преобразование в единую модель и отправку.

Поскольку парсинг логов — ресурсоёмкая операция, он выполняется только для записей, заранее отобранных как потенциально содержащие события безопасности.

## Как включить события безопасности

1. Включите необходимые модули-зависимости, если они ещё не включены:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: log-shipper
   spec:
     enabled: true
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: loki
   spec:
     enabled: true
   ```

1. Включите модуль `security-events-manager`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: security-events-manager
   spec:
     enabled: true
   ```

Все доступные параметры модуля перечислены [в разделе документации модуля `security-events-manager`](/modules/security-events-manager/configuration.html).

## Настройка источников сбора

Сбор логов и обработка настраиваются через следующие кастомные ресурсы:

- [`PodSecurityEventShipper`](/modules/security-events-manager/cr.html#podsecurityeventshipper) — для контейнерных источников (логи подов в конкретном неймспейсе);
- [`ClusterSecurityEventShipper`](/modules/security-events-manager/cr.html#clustersecurityeventshipper) — для кластерных источников (файлы узлов, системные сервисы).

В этих ресурсах настраиваются:

1. источник логов (поле `source` и `input`);
1. правила первичного отбора записей (поле `produces`);
1. правила парсинга (поле `parser` или `parserRef` со ссылкой на `SecurityEventLoggingTransformationRules` / `ClusterSecurityEventLoggingTransformationRules`);
1. правила преобразования и обогащения (поля `transform` и `enrich`).

Пример настройки контейнерного источника:

```yaml
apiVersion: security.deckhouse.io/v1alpha1
kind: PodSecurityEventShipper
metadata:
  name: runtime-audit-engine
  namespace: d8-runtime-audit-engine
spec:
  - source: runtime-audit-engine
    input:
      type: KubernetesPods
      kubernetesPods:
        labelSelector:
          matchLabels:
            app: runtime-audit-engine
    produces:
      - eventCode: K8S_POD_CREATED
        extract:
          field: message
          operator: Regex
          values:
            - ".*K8s Pod Created.*"
```

Пример настройки кластерного источника:

```yaml
apiVersion: security.deckhouse.io/v1alpha1
kind: ClusterSecurityEventShipper
metadata:
  name: kube-audit
spec:
  - source: kube-audit
    input:
      type: File
      files:
        - /var/log/kube-audit/audit.log
    produces:
      - eventCode: UNAUTHORIZED_ACCESS
        extract:
          field: message
          operator: Regex
          values:
            - ".*\"code\":401.*"
```

## Настройка отправки событий безопасности

После формирования событий необходимо определить, в какие приёмники их отправлять.
Для этого требуется:

1. Настроить приёмники событий безопасности.
1. Настроить правила отправки событий в приёмники.

### Настройка приёмника

Приёмники настраиваются через ресурс [`ClusterSecurityEventDestination`](/modules/security-events-manager/cr.html#clustersecurityeventdestination).

В качестве приёмников используются типы, доступные в экосистеме `log-shipper` (Loki, Elasticsearch, Kafka, Splunk, Vector, File и др.).
Для хранения событий безопасности внутри кластера предусмотрена автоматическая настройка приёмника `Loki` при включении соответствующей опции модуля.

Пример:

```yaml
apiVersion: security.deckhouse.io/v1alpha1
kind: ClusterSecurityEventDestination
metadata:
  name: cluster-loki
spec:
  type: Loki
  loki:
    auth:
      strategy: Bearer
      token: <EXAMPLE>
    endpoint: https://loki.d8-monitoring:3100
    tls:
      verifyCertificate: true
      verifyHostname: true
      ca: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0t...
```

### Настройка правил отправки событий в приёмники

При настройке правил отправки необходимо определить:

- события из каких источников отправляются;
- минимальный уровень критичности (severity) для отправки;
- целевые приёмники.

Для этого используется ресурс [`ClusterSecurityEventConfig`](/modules/security-events-manager/cr.html#clustersecurityeventconfig).
В ресурсе указываются источники (в формате точных имён или масок), минимальный `severity`, и массив приёмников
(ресурсы [`ClusterSecurityEventDestination`](/modules/security-events-manager/cr.html#clustersecurityeventdestination)),
в каждый из которых отправляются отобранные события.

Пример:

```yaml
apiVersion: security.deckhouse.io/v1alpha1
kind: ClusterSecurityEventConfig
metadata:
  name: default
spec:
  defaultSeverityThreshold: Low
  enabledSources:
    - clusterSecurityEventShipper/kube-audit/kube-audit
    - podSecurityEventShipper/d8-runtime-audit-engine/runtime-audit-engine/falco
    - podSecurityEventShipper/d8-user-authn/user-authn/dex
  # Либо указать маски.
  # enabledSourcesMasks:
  #   - clusterSecurityEventShipper/kube-audit/*
  #   - podSecurityEventShipper/*

  destinations:
    - cluster-loki
```

## Стандартные настройки модуля

Если настройки модуля явно не заданы, то в кластере размещаются следующие объекты:

1. `ClusterSecurityEventConfig` — отвечает за настройку отправки событий безопасности в приёмники. Стандартно используются следующие настройки:

    ```yaml
    apiVersion: security.deckhouse.io/v1alpha1
    kind: ClusterSecurityEventConfig
    metadata:
      name: default
    spec:
      defaultSeverityThreshold: Medium
      destinations:
        - cluster-loki
      enabledSourcesMasks:
        - podSecurityEventShipper/*
        - clusterSecurityEventShipper/*
    ```

    На конфигурирование этих параметров отвечает настройка модуля [`securityEventConfig`](/modules/security-events-manager/configuration.html#securityeventconfig).

1. `ClusterSecurityEventDestination` — отвечает за настройку приёмника событий безопасности. Стандартно генерируется объект, с помощью которого возможно осуществить отправку событий безопасности в сервис `loki`, расположенный в кластере. Генерируется следующий объект:

    ```yaml
    apiVersion: security.deckhouse.io/v1alpha1
    kind: ClusterSecurityEventDestination
    metadata:
      name: cluster-loki
    spec:
      type: Loki
      loki:
        auth:
          strategy: Bearer
          token: <token> # Заполняется автоматически.
        endpoint: https://loki.d8-monitoring:3100
    ```

    Отключить генерацию стандартного приёмника можно отдельным параметром [`clusterSecurityEventDestination.clusterLoki`](/modules/security-events-manager/configuration.html#clustersecurityeventdestination).

## Реализованные события безопасности

Модуль поставляется со встроенным набором правил обнаружения событий безопасности,
охватывающим аутентификацию, конфигурацию, RBAC, среду выполнения и другие категории.
Актуальный перечень реализованных событий безопасности, их коды, критичность и описания
приведены в документации модуля [`security-events-manager`](/modules/security-events-manager/security_events.html).
