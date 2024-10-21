---
title: "Модуль monitoring-custom"
type:
  - instruction
search: prometheus
---

Модуль расширяет возможности модуля [prometheus](../../modules/300-prometheus/) по мониторингу приложений пользователей.

Чтобы организовать сбор метрик с приложений модулем `monitoring-custom`, необходимо:

- Поставить лейбл `prometheus.deckhouse.io/custom-target` на Service или под. Значение лейбла определит имя в списке target'ов Prometheus.
  - В качестве значения label'а prometheus.deckhouse.io/custom-target стоит использовать название приложения (маленькими буквами, разделитель `-`), которое позволяет его уникально идентифицировать в кластере.

     При этом, если приложение ставится в кластер больше одного раза (staging, testing и т. д.) или даже ставится несколько раз в один namespace, достаточно одного общего названия, так как у всех метрик в любом случае будут лейблы namespace, pod и, если доступ осуществляется через Service, лейбл service. То есть это название, уникально идентифицирующее приложение в кластере, а не единичную его инсталляцию.
- Порту, с которого нужно собирать метрики, указать имя `http-metrics` и `https-metrics` для подключения по HTTP или HTTPS соответственно.

  Если это невозможно (например, порт уже определен и назван другим именем), необходимо воспользоваться аннотациями: `prometheus.deckhouse.io/port: номер_порта` — для указания порта и `prometheus.deckhouse.io/tls: "true"` — если сбор метрик будет проходить по HTTPS.

  > **Важно!** При указании аннотации на Service в качестве значения порта необходимо использовать `targetPort`. То есть тот порт, что открыт и слушается приложением, а не порт Service'а.

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

- При использовании service mesh [Istio](../110-istio/) в режиме STRICT mTLS указать для сбора метрик следующую аннотацию у Service: `prometheus.deckhouse.io/istio-mtls: "true"`. Важно, что метрики приложения должны экспортироваться по протоколу HTTP без TLS.

- *(Не обязательно)* Указать дополнительные аннотации для более тонкой настройки:

  * `prometheus.deckhouse.io/path` — путь для сбора метрик (по умолчанию: `/metrics`).
  * `prometheus.deckhouse.io/query-param-$name` — GET-параметры, будут преобразованы в map вида `$name=$value` (по умолчанию: ''):
    - возможно указать несколько таких аннотаций.

      Например, `prometheus.deckhouse.io/query-param-foo=bar` и `prometheus.deckhouse.io/query-param-bar=zxc` будут преобразованы в query: `http://...?foo=bar&bar=zxc`.
  * `prometheus.deckhouse.io/allow-unready-pod` — разрешает сбор метрик с подов в любом состоянии (по умолчанию метрики собираются только с подов в состоянии Ready). Эта опция полезна в очень редких случаях. Например, если ваше приложение запускается очень долго (при старте загружаются данные в базу или прогреваются кэши), но в процессе запуска уже отдаются полезные метрики, которые помогают следить за запуском приложения.
  * `prometheus.deckhouse.io/sample-limit` — сколько семплов разрешено собирать с пода (по умолчанию 5000). Значение по умолчанию защищает от ситуации, когда приложение внезапно начинает отдавать слишком большое количество метрик, что может нарушить работу всего мониторинга. Эту аннотацию надо вешать на тот же ресурс, на котором висит лейбл  `prometheus.deckhouse.io/custom-target`.

### Пример: Service

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
    prometheus.deckhouse.io/scrape-interval: "60s"            # По умолчанию значение scrapeInterval модуля monitoring.
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

### Пример: Deployment

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
        prometheus.deckhouse.io/sample-limit: "5000"   # По умолчанию принимается не больше 5000 метрик от одного пода.
        prometheus.deckhouse.io/scrape-interval: "60s" # По умолчанию значение scrapeInterval модуля monitoring.
    spec:
      containers:
      - name: my-app
        image: my-app:1.7.9
        ports:
        - name: https-metrics
          containerPort: 443
```
