---
title: "Настройка мониторинга приложений"
description: "Настройка мониторинга приложений в Deckhouse Kubernetes Platform. Подключение через лейблы, ServiceMonitor, PodMonitor, ScrapeConfig. Интеграция с Prometheus и blackbox-exporter для проверки доступности."
permalink: ru/user/monitoring/app.html
lang: ru
---

Deckhouse Kubernetes Platform (DKP) поддерживает четыре способа подключения приложения к системе мониторинга:

| Способ подключения | Описание                                                                                                                                                                                                                                                                                                                                 |
| ------------------ |------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [Через лейблы и аннотации](#настройка-сбора-метрик-через-лейблы-и-аннотации) | Самый простой и быстрый способ, требующий лишь добавления метаданных к сервису или поду. Позволяет задать базовые параметры мониторинга.                                                                                                                                                                                                 |
| [С помощью PodMonitor или ServiceMonitor](#настройка-сбора-метрик-с-помощью-ресурса-podmonitor-или-servicemonitor) | Расширенный способ настройки мониторинга для случаев, когда требуется использование relabeling-правил Prometheus. Позволяет гибко управлять сбором метрик и обработкой лейблов. Данный подход подходит для сложных сценариев мониторинга, но требует более глубокого понимания принципов работы Prometheus и его механизма сбора метрик. |
| [С помощью ScrapeConfig](#настройка-сбора-метрик-через-scrape_configs-с-помощью-ресурса-scrapeconfig) | Способ настройки мониторинга, максимально приближенный к нативной структуре конфигурации Prometheus. Предоставляет полный контроль над scrape-настройками, включая relabeling, и позволяет собирать метрики как из Kubernetes, так и с targets, расположенных за пределами кластера.                                                     |
| [Мониторинг доступности с помощью blackbox-exporter](#настройка-сбора-метрик-с-помощью-blackbox-exporter) | Способ мониторинга доступности эндпоинтов с помощью проверок (probes). Подключается с помощью [blackbox-exporter](https://github.com/prometheus/blackbox_exporter/), который необходимо установить в кластере отдельно.                                                                                                                  |

## Настройка сбора метрик через лейблы и аннотации

{% alert level="info" %}
Здесь описан базовый сценарий подключения приложения.
Для более гибкой настройки доступны [дополнительные аннотации](#дополнительные-аннотации-для-тонкой-настройки).
{% endalert %}

1. Убедитесь, что включен [модуль `monitoring-custom`](/modules/monitoring-custom/).
   При необходимости обратитесь к администратору DKP.

1. Убедитесь, что приложение, с которого будут собираться метрики, отдает их в [формате Prometheus](https://prometheus.io/docs/instrumenting/exposition_formats/).

1. Установите лейбл `prometheus.deckhouse.io/custom-target` на сервис или под, которые необходимо подключить к мониторингу.
   Значение лейбла определит имя в списке target'ов Prometheus.
  
   Пример:

   ```yaml
   labels:
     prometheus.deckhouse.io/custom-target: my-app
   ```

   В качестве значения лейбла `prometheus.deckhouse.io/custom-target` рекомендуется использовать название приложения,
   которое позволяет его уникально идентифицировать в кластере.

   Формат лейбла должен соответствовать [требованиям Kubernetes](https://kubernetes.io/ru/docs/concepts/overview/working-with-objects/labels/):
   не более 63 символов, среди которых могут быть буквенно-цифровые символы (`[a-z0-9A-Z]`),
   а также дефисы (`-`), знаки подчеркивания (`_`), точки (`.`).

   Если приложение ставится в кластер больше одного раза (staging, testing и т. д.)
   или даже ставится несколько раз в одно пространство имён, достаточно одного общего названия,
   так как у всех метрик в любом случае будут лейблы `namespace`, `pod` и, если доступ осуществляется через сервис, лейбл `service`.
   Это название, которое уникально идентифицирует приложение в кластере, а не его единичную инсталляцию.

1. Для порта, с которого нужно собирать метрики,
   укажите имя `http-metrics` и `https-metrics` для подключения по HTTP или HTTPS соответственно.

   Если это невозможно (например, порт уже определен и назван другим именем), используйте следующие аннотации:

   - `prometheus.deckhouse.io/port: номер_порта` — для указания порта;
   - `prometheus.deckhouse.io/tls: "true"` — если сбор метрик будет проходить по HTTPS.

   > При указании аннотации для сервиса в качестве значения порта необходимо использовать `targetPort` — порт,
   > который открыт и слушается приложением, а не порт сервиса.

   - Пример 1:

     ```yaml
     ports:
     - name: https-metrics
       containerPort: 443
     ```

   - Пример 2:

     ```yaml
     annotations:
       prometheus.deckhouse.io/port: "443"
       prometheus.deckhouse.io/tls: "true"  # Если метрики отдаются по HTTP, эту аннотацию указывать не нужно.
     ```

1. При использовании service mesh [Istio](../../admin/configuration/network/internal/encrypting-pods.html) в режиме STRICT mTLS
   укажите для сбора метрик аннотацию `prometheus.deckhouse.io/istio-mtls: "true"` у сервиса или пода.
   Важно, что метрики приложения должны экспортироваться по протоколу HTTP без TLS.

   Пример:

   ```yaml
   annotations:
     prometheus.deckhouse.io/istio-mtls: "true"
   ```

### Пример настройки сбора метрик с Service

Ниже приведён пример настройки сбора метрик с Service:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  namespace: my-namespace
  labels:
    prometheus.deckhouse.io/custom-target: my-app
  annotations:
    prometheus.deckhouse.io/port: "8061"                      # По умолчанию будет использоваться порт сервиса с именем http-metrics или https-metrics.
    prometheus.deckhouse.io/path: "/my_app/metrics"           # По умолчанию /metrics.
    prometheus.deckhouse.io/query-param-format: "prometheus"  # По умолчанию ''.
    prometheus.deckhouse.io/allow-unready-pod: "true"         # По умолчанию поды НЕ в Ready игнорируются.
    prometheus.deckhouse.io/sample-limit: "5000"              # По умолчанию принимается не больше 5000 метрик от одного пода.
spec:
  ports:
  - name: my-app
    port: 8060
  - name: http-metrics
    port: 8061
    targetPort: 8061
  selector:
    app: my-app
```

### Пример настройки сбора метрик с Deployment

Ниже приведён пример настройки сбора метрик с Deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  labels:
    app: my-app
spec:
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
        prometheus.deckhouse.io/custom-target: my-app
      annotations:
        prometheus.deckhouse.io/sample-limit: "5000"  # По умолчанию принимается не больше 5000 метрик от одного пода.
    spec:
      containers:
      - name: my-app
        image: my-app:1.7.9
        ports:
        - name: https-metrics
          containerPort: 443
```

### Дополнительные аннотации для тонкой настройки

Для более точной настройки мониторинга приложения можно указать дополнительные аннотации для пода или сервиса,
для которых настраивается мониторинг:

- `prometheus.deckhouse.io/path` — путь для сбора метрик (по умолчанию: `/metrics`);
- `prometheus.deckhouse.io/query-param-$name` — GET-параметры, которые будут преобразованы в map вида `$name=$value` (по умолчанию: `''`).
  Можно указать несколько таких аннотаций.
  Например, `prometheus.deckhouse.io/query-param-foo=bar` и `prometheus.deckhouse.io/query-param-bar=zxc` будут преобразованы в запрос вида `http://...?foo=bar&bar=zxc`;
- `prometheus.deckhouse.io/allow-unready-pod` — разрешает сбор метрик с подов в любом состоянии
  (по умолчанию метрики собираются только с подов в состоянии `Ready`). Эта опция полезна в редких случаях.
  Например, если ваше приложение запускается очень долго (при старте загружаются данные в базу или прогреваются кеши),
  но в процессе запуска уже отдаются полезные метрики, которые помогают следить за запуском приложения;
- `prometheus.deckhouse.io/sample-limit` — сколько семплов разрешено собирать с пода (по умолчанию `5000`).
  Значение по умолчанию защищает от ситуации, когда приложение внезапно начинает отдавать слишком большое количество метрик,
  что может нарушить работу всего мониторинга.
  Аннотация должна быть размещена на том же ресурсе, на который добавлен лейбл `prometheus.deckhouse.io/custom-target`.

## Настройка сбора метрик с помощью ресурса PodMonitor или ServiceMonitor

DKP поддерживает подключение приложений через два схожих по функциональности ресурса:

- [PodMonitor](/modules/operator-prometheus/cr.html#podmonitor) (рекомендуемый вариант) — обнаруживает поды напрямую
  и собирает метрики с их контейнеров. В большинстве случае это предпочтительный вариант,
  поскольку он работает напрямую с подами и не зависит от наличия сервисов.
- [ServiceMonitor](/modules/operator-prometheus/cr.html#servicemonitor) — обнаруживает сервисы и собирает метрики с подов,
  находящихся за ними. При этом сервисы используются как источник метаданных (например, лейблов),
  а фактический сбор метрик выполняется с адресов подов, входящих в соответствующие эндпоинты.

Оба ресурса позволяют задавать интервал опроса, пути, TLS-параметры, параметры переписывания лейблов (relabeling) и другие настройки.

Разница между ресурсами заключается в источнике собираемых метрик.
Используйте PodMonitor, если требуется сбор метрик напрямую с подов,
и ServiceMonitor — если ваше приложение публикует метрики через Service.

Чтобы подключить приложение к системе мониторинга с помощью одного из этих ресурсов, выполните следующие шаги:

1. Установите лейбл `prometheus.deckhouse.io/monitor-watcher-enabled: "true"` в том же пространстве имен,
   где будет находиться PodMonitor или ServiceMonitor:

   ```yaml
   apiVersion: v1
   kind: Namespace
   metadata:
     name: frontend
     labels:
       prometheus.deckhouse.io/monitor-watcher-enabled: "true"
   ```

1. Создайте ресурс PodMonitor или ServiceMonitor, указав обязательный лейбл `prometheus: main`,
   а также перечислив параметры необходимых эндпоинтов.

   Пример для PodMonitor:

   ```yaml
   apiVersion: monitoring.coreos.com/v1
   kind: PodMonitor
   metadata:
     name: example-app
     namespace: frontend
     labels:
       prometheus: main
   spec:
     selector:
       matchLabels:
         app: example-app
     podMetricsEndpoints:
       - port: metrics
         interval: 30s
         path: /metrics
   ```

   Пример для ServiceMonitor:

   ```yaml
   apiVersion: monitoring.coreos.com/v1
   kind: ServiceMonitor
   metadata:
     name: example-app
     namespace: frontend
     labels:
       prometheus: main
   spec:
     selector:
       matchLabels:
         app: example-app
     endpoints:
       - port: web
         interval: 30s
         path: /metrics
   ```

   При необходимости задайте дополнительные настройки, используя справку по доступным параметрам ресурсов:
   [PodMonitor](/modules/operator-prometheus/cr.html#podmonitor), [ServiceMonitor](/modules/operator-prometheus/cr.html#servicemonitor).

## Настройка сбора метрик через scrape_configs с помощью ресурса ScrapeConfig

[ScrapeConfig](/modules/operator-prometheus/cr.html#scrapeconfig) — это кастомный ресурс,
который позволяет настраивать раздел конфигурации Prometheus под названием `scrape_config`
для полного контроля над процессом сбора метрик.

Чтобы подключить приложение к системе мониторинга, выполните следующие шаги:

1. Установите лейбл `prometheus.deckhouse.io/scrape-configs-watcher-enabled: "true"` в пространстве имен,
   где будет находиться ScrapeConfig:

   ```yaml
   apiVersion: v1
   kind: Namespace
   metadata:
     name: frontend
     labels:
       prometheus.deckhouse.io/scrape-configs-watcher-enabled: "true"
   ```

{% raw %}

1. Создайте [ресурс ScrapeConfig](/modules/operator-prometheus/cr.html#scrapeconfig),
   указав обязательный лейбл `prometheus: main`:

   ```yaml
   apiVersion: monitoring.coreos.com/v1alpha1
   kind: ScrapeConfig
   metadata:
     name: example-scrape-config
     namespace: frontend
     labels:
       prometheus: main
   spec:
     honorLabels: true
     staticConfigs:
       - targets: ['example-app.frontend.svc.{{ .Values.global.discovery.clusterDomain }}.:8080']
     relabelings:
       - regex: endpoint|namespace|pod|service
         action: labeldrop
       - targetLabel: scrape_endpoint
         replacement: main
       - targetLabel: job
         replacement: kube-state-metrics
     metricsPath: '/metrics'
   ```

{% endraw %}

   При необходимости задайте дополнительные настройки, используя [справку по доступным параметрам ресурса](/modules/operator-prometheus/cr.html#scrapeconfig).

## Настройка сбора метрик с помощью blackbox-exporter

DKP поддерживает сбор метрик доступности с [blackbox-exporter](https://github.com/prometheus/blackbox_exporter/),
который не входит в состав DKP и должен быть установлен отдельно в кластере.
Для этого используется [кастомный ресурс Probe](/modules/operator-prometheus/cr.html#probe),
который описывает проверки доступности (probes), выполняемые Prometheus.

Чтобы подключить Probe к системе мониторинга DKP, выполните следующие шаги:

1. Установите лейбл `prometheus.deckhouse.io/probe-watcher-enabled: "true"` в пространстве имен, где будет находиться Probe:

   ```yaml
   apiVersion: v1
   kind: Namespace
   metadata:
     name: frontend
     labels:
       prometheus.deckhouse.io/probe-watcher-enabled: "true"
   ```

1. Создайте [ресурс Probe](/modules/operator-prometheus/cr.html#probe), указав обязательный лейбл `prometheus: main`:

   ```yaml
   apiVersion: monitoring.coreos.com/v1
   kind: Probe
   metadata:
     labels:
       app: prometheus
       component: probes
       prometheus: main
     name: cdn-is-up
     namespace: frontend
   spec:
     interval: 30s
     jobName: httpGet
     module: http_2xx
     prober:
       path: /probe
       scheme: http
       url: blackbox-exporter.blackbox-exporter.svc.cluster.local:9115
     targets:
       staticConfig:
         static:
         - https://example.com/status
   ```

   При необходимости задайте дополнительные настройки, используя [справку по доступным параметрам ресурса](/modules/operator-prometheus/cr.html#probe).
