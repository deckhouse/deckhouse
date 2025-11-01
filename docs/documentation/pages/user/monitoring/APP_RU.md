---
title: "Настройка мониторинга приложений"
permalink: ru/user/monitoring/app.html
lang: ru
---

Чтобы организовать сбор метрик с любых приложений в кластере, выполните следующие шаги:

1. Включите модуль `monitoring-custom`, если он не включен.

   Включить мониторинг кластера можно в [веб-интерфейсе Deckhouse](/modules/console/), или с помощью следующей команды:

   ```shell
   d8 platform module enable monitoring-custom
   ```
  
   > У текущего пользователя платформы может не быть прав на включение или выключение модулей. Если прав нет, необходимо обратиться к администратору платформы.

1. Убедитесь, что приложение, с которого будут собираться метрики, отдает их в формате Prometheus.

1. Поставьте лейбл `prometheus.deckhouse.io/custom-target` на Service или под, которые необходимо подключить к мониторингу. Значение лейбла определит имя в списке target'ов Prometheus.
  
   Пример:

   ```yaml
   labels:
     prometheus.deckhouse.io/custom-target: my-app
   ```

   В качестве значения лейбла `prometheus.deckhouse.io/custom-target` рекомендуется использовать название приложения, которое позволяет его уникально идентифицировать в кластере.

   Формат лейбла должен соответствовать [требованиям Kubernetes](https://kubernetes.io/ru/docs/concepts/overview/working-with-objects/labels/): не более 63 символов, среди которых могут быть буквенно-цифровые символы (`[a-z0-9A-Z]`), а также дефисы (`-`), знаки подчеркивания (`_`), точки (`.`).

   Если приложение ставится в кластер больше одного раза (staging, testing и т. д.) или даже ставится несколько раз в одно пространство имён, достаточно одного общего названия, так как у всех метрик в любом случае будут лейблы `namespace`, `pod` и, если доступ осуществляется через Service, лейбл `service`. Это название, уникально идентифицирующее приложение в кластере, а не его единичную инсталляцию.

1. Для порта, с которого нужно собирать метрики, укажите имя `http-metrics` и `https-metrics` для подключения по HTTP или HTTPS соответственно.

   Если это невозможно (например, порт уже определен и назван другим именем), необходимо воспользоваться аннотациями: `prometheus.deckhouse.io/port: номер_порта` — для указания порта и `prometheus.deckhouse.io/tls: "true"` — если сбор метрик будет проходить по HTTPS.

   > При указании аннотации на Service в качестве значения порта необходимо использовать `targetPort`. То есть тот порт, что открыт и слушается приложением, а не порт Service'а.

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

1. При использовании service mesh [Istio](../../admin/configuration/network/internal/encrypting-pods.html) в режиме STRICT mTLS укажите для сбора метрик следующую аннотацию у Service или Pod: `prometheus.deckhouse.io/istio-mtls: "true"`. Важно, что метрики приложения должны экспортироваться по протоколу HTTP без TLS.

   Пример:

   ```yaml
   annotations:
     prometheus.deckhouse.io/istio-mtls: "true"
   ```

## Пример: Service

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

## Пример: Deployment

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

## Дополнительные аннотации для тонкой настройки

Для более точной настройки мониторинга приложения можно указать дополнительные аннотации для пода или сервиса, для которых настраивается мониторинг:

- `prometheus.deckhouse.io/path` — путь для сбора метрик (по умолчанию: `/metrics`).
- `prometheus.deckhouse.io/query-param-$name` — GET-параметры, которые будут преобразованы в map вида `$name=$value` (по умолчанию: '').
  Можно указать несколько таких аннотаций.
  Например, `prometheus.deckhouse.io/query-param-foo=bar` и `prometheus.deckhouse.io/query-param-bar=zxc` будут преобразованы в запрос вида `http://...?foo=bar&bar=zxc`.
- `prometheus.deckhouse.io/allow-unready-pod` — разрешает сбор метрик с подов в любом состоянии (по умолчанию метрики собираются только с подов в состоянии Ready). Эта опция полезна в редких случаях. Например, если ваше приложение запускается очень долго (при старте загружаются данные в базу или прогреваются кэши), но в процессе запуска уже отдаются полезные метрики, которые помогают следить за запуском приложения.
- `prometheus.deckhouse.io/sample-limit` — сколько семплов разрешено собирать с пода (по умолчанию 5000). Значение по умолчанию защищает от ситуации, когда приложение внезапно начинает отдавать слишком большое количество метрик, что может нарушить работу всего мониторинга. Аннотация должна быть размещена на том же ресурсе, на котором висит лейбл  `prometheus.deckhouse.io/custom-target`.
