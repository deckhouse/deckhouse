---
title: "Модуль monitoring-custom"
type:
  - instruction
search: prometheus
---

Модуль расширяет возможности модуля [prometheus](../../modules/300-prometheus/) по мониторингу приложений пользователей.

Чтобы организовать сбор метрик с приложений модулем `monitoring-custom`, необходимо:

- Поставить лейбл `prometheus.deckhouse.io/custom-target` на Service или под. Значение лейбла определит имя в списке target Prometheus.
  - В качестве значения `label prometheus.deckhouse.io/custom-target` рекомендуется использовать название приложения (маленькими буквами, разделитель `-`), которое позволяет идентифицировать его в кластере.

     Если приложение ставится в кластер больше одного раза (staging, testing и т. д.) или ставится несколько раз в один namespace, достаточно одного общего названия, так как у всех метрик обозначатся лейблы namespace, pod и, если доступ осуществляется через Service, лейбл service. Это название, идентифицирующее приложение в кластере, а не его единичную инсталляцию.

Порту, с которого собираются метрики, укажите имя `http-metrics` и `https-metrics` для подключения по HTTP или HTTPS.

  Если это невозможно (например, порт уже определен и назван другим именем), необходимо воспользоваться аннотациями:
  * `prometheus.deckhouse.io/port: номер_порта` — для указания порта;
  * `prometheus.deckhouse.io/tls: "true"` — если сбор метрик будет проходить по HTTPS.

  > **Важно!** При указании аннотации на Service в качестве значения порта необходимо использовать `targetPort`. Это означает, что порт`targetPort` открыт и слушается приложением.

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

- При использовании service mesh [Istio](../110-istio/) в режиме STRICT mTLS указажите для сбора метрик аннотацию у Service: `prometheus.deckhouse.io/istio-mtls: "true"`. Важно, что метрики приложения должны экспортироваться по протоколу HTTP без TLS.

- *(Не обязательно)* Укажите дополнительные аннотации для более тонкой настройки:

  * `prometheus.deckhouse.io/path` — путь для сбора метрик (по умолчанию: `/metrics`).
  * `prometheus.deckhouse.io/query-param-$name` — GET-параметры, будут преобразованы в map вида `$name=$value` (по умолчанию: '') - для этого существует возможность нескольких аннотаций.

      Например, `prometheus.deckhouse.io/query-param-foo=bar` и `prometheus.deckhouse.io/query-param-bar=zxc` будут преобразованы в query: `http://...?foo=bar&bar=zxc`.
  * `prometheus.deckhouse.io/allow-unready-pod` — разрешает сбор метрик с подов в любом состоянии (по умолчанию метрики собираются только с подов в состоянии Ready). Эта полезно в редких случаях, когда приложение запускается долго, например, при старте загружаются данные в базу или прогреваются кэши (warm cache), но в процессе запуска отдаются полезные метрики, которые помогают следить за запуском приложения.
  * `prometheus.deckhouse.io/sample-limit` — сколько семплов разрешено собирать с пода (по умолчанию 5000). Значение по умолчанию защищает от ситуации, когда приложение начинает отдавать увеличенное количество метрик, что может нарушить работу мониторинга. Аннотация должна быть размещена на том же ресурсе, на котором находится лейбл `prometheus.deckhouse.io/custom-target`.

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
        prometheus.deckhouse.io/sample-limit: "5000"  # По умолчанию принимается не больше 5000 метрик от одного пода.
    spec:
      containers:
      - name: my-app
        image: my-app:1.7.9
        ports:
        - name: https-metrics
          containerPort: 443
```
