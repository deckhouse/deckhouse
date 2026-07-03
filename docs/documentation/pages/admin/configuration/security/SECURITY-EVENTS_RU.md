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
действии или факте.

DKP позволяет:

- собирать события безопасности из различных источников (логи подов, файлы на узлах, аудит Kubernetes API);
- приводить события к единому формату с обязательным набором атрибутов;
- обогащать события контекстными данными;
- фильтровать события по источникам и уровню критичности;
- доставлять события в системы хранения и аналитики (Loki, Elasticsearch, Kafka, Splunk, Vector и другие).

За сбор, обработку и доставку событий безопасности отвечает модуль [`security-events-manager`](/modules/security-events-manager/).
Для сбора логов задействуется вспомогательный модуль [`log-shipper`](/modules/log-shipper/).

## Зависимости и требования

Для работы модуля `security-events-manager` требуются следующие модули DKP:

- [`log-shipper`](/modules/log-shipper/) — обеспечивает сбор логов из подов и файлов на узлах, а также выполняет предварительный отбор записей;
- [`loki`](/modules/loki/) — обеспечивает хранение событий безопасности внутри кластера (используется в качестве приёмника событий по умолчанию).

Подробности об архитектуре работы с событиями безопасности можно найти [в разделе «Архитектура событий безопасности»](../../../../architecture/security/security-events.html).

## Источники данных для событий безопасности

Модуль `security-events-manager` собирает данные из двух типов источников:

- **Контейнерные источники** — логи приложений в подах Kubernetes. Сбор выполняется через модуль `log-shipper`, который отбирает записи по лейблам подов и неймспейсам.
- **Кластерные источники** — файлы на узлах и логи системных сервисов, не привязанные к конкретному неймспейсу. Например:
  - `/var/log/kube-audit/audit.log` — лог аудита Kubernetes API;
  - `/var/log/auth.log` — лог системных событий аутентификации узла.

Сбор выполняется в два этапа:

1. Модуль `log-shipper` предварительно отбирает логовые записи с помощью простых операций сравнения (`In`, `NotIn`, `Regex`, `NotRegex`, `Exists`, `DoesNotExist`) и передаёт их в шлюз `security-events-manager`.
1. Шлюз `security-events-manager` выполняет парсинг записей, преобразует их в единую модель и отправляет в настроенный приёмник.

Поскольку парсинг логов — это ресурсоёмкая операция, он выполняется только для записей, заранее отобранных как потенциально содержащие события безопасности.

## Включение событий безопасности

Чтобы включить события безопасности, выполните следующие шаги:

1. Включите необходимые модули, если они ещё не включены:

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

Все доступные параметры `security-events-manager` перечислены [в документации модуля](/modules/security-events-manager/configuration.html).

## Настройка источников событий

Источники событий безопасности настраиваются через следующие кастомные ресурсы:

- [PodSecurityEventShipper](/modules/security-events-manager/cr.html#podsecurityeventshipper) — для контейнерных источников (логи подов в конкретном неймспейсе);
- [ClusterSecurityEventShipper](/modules/security-events-manager/cr.html#clustersecurityeventshipper) — для кластерных источников (файлы на узлах, логи системных сервисов).

В этих ресурсах настраиваются:

- источник логов (поля `source` и `input`);
- правила первичного отбора записей (поле `produces`);
- правила парсинга (поле `parser` или `parserRef` со ссылкой на ресурсы SecurityEventLoggingTransformationRules или ClusterSecurityEventLoggingTransformationRules);
- правила преобразования и обогащения (поля `transform` и `enrich`).

Пример конфигурации PodSecurityEventShipper для настройки контейнерного источника:

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

Пример конфигурации ClusterSecurityEventShipper для настройки кластерного источника:

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

## Настройка отправки событий

После формирования событий необходимо определить, в какие приёмники они будут отправляться.
Для этого требуется:

1. Настроить приёмники событий безопасности.
1. Настроить правила отправки событий в приёмники.

### Настройка приёмника

Приёмники настраиваются через ресурс [ClusterSecurityEventDestination](/modules/security-events-manager/cr.html#clustersecurityeventdestination).

Поддерживаются все типы приёмников, доступные в экосистеме `log-shipper` (Loki, Elasticsearch, Kafka, Splunk, Vector, File и другие).
Для хранения событий безопасности внутри кластера предусмотрена автоматическая настройка приёмника Loki при включении соответствующей опции модуля.

Пример конфигурации ClusterSecurityEventDestination для настройки приёмника Loki:

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

### Настройка правил отправки событий

При настройке правил отправки событий безопасности необходимо определить:

- из каких источников будут отправляться события;
- минимальный уровень критичности (severity) для отправки;
- приёмники, в которые будут отправляться события.

Для этого используется ресурс [ClusterSecurityEventConfig](/modules/security-events-manager/cr.html#clustersecurityeventconfig).
В ресурсе указываются источники (в формате точных имён или масок), минимальный уровень критичности (severity) и массив приёмников
(ресурсы [ClusterSecurityEventDestination](/modules/security-events-manager/cr.html#clustersecurityeventdestination)),
в каждый из которых будут отправляться отобранные события.

Пример конфигурации ClusterSecurityEventConfig для отправки событий из источников `kube-audit`, `runtime-audit-engine` и `user-authn` в приёмник `cluster-loki`:

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
  # Либо укажите маски.
  # enabledSourcesMasks:
  #   - clusterSecurityEventShipper/kube-audit/*
  #   - podSecurityEventShipper/*

  destinations:
    - cluster-loki
```

## Стандартные настройки модуля

Если настройки модуля не заданы явно, в кластере автоматически создаются следующие объекты:

- [ClusterSecurityEventConfig](/modules/security-events-manager/cr.html#clustersecurityeventconfig) — отвечает за настройку отправки событий безопасности в приёмники. По умолчанию создаётся объект со следующими настройками:

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

  Настройка этого объекта выполняется с помощью параметра [`securityEventConfig`](/modules/security-events-manager/configuration.html#parameters-securityeventconfig).

- [ClusterSecurityEventDestination](/modules/security-events-manager/cr.html#clustersecurityeventdestination) — отвечает за настройку приёмника событий безопасности. По умолчанию создаётся следующий объект, который отправляет события в сервис Loki, расположенный внутри кластера:

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

  Отключить создание стандартного приёмника можно с помощью параметра [`clusterSecurityEventDestination.clusterLoki`](/modules/security-events-manager/configuration.html#parameters-clustersecurityeventdestination-clusterloki).

## Поддерживаемые события безопасности

Модуль `security-events-manager` поставляется со встроенным набором правил обнаружения событий безопасности,
охватывающим аутентификацию, конфигурацию, RBAC, среду выполнения и другие категории.
Актуальный перечень поддерживаемых событий безопасности, их коды, критичность и описание
приведены [в документации модуля](/modules/security-events-manager/security_events.html).
