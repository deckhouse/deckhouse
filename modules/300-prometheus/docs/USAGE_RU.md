---
title: "Prometheus-мониторинг: примеры конфигурации"
type:
  - instruction
search: prometheus remote write, как подключится к Prometheus, пользовательская Grafana, prometheus remote write
---

## Пример конфигурации модуля

```yaml
prometheus: |
  auth:
    password: xxxxxx
  retentionDays: 7
  storageClass: rbd
  nodeSelector:
    node-role/example: ""
  tolerations:
  - key: dedicated
    operator: Equal
    value: example
```

## Запись данных Prometheus в longterm storage

У Prometheus есть поддержка remote_write данных из локального Prometheus в отдельный longterm storage (например: [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics)). В Deckhouse поддержка данного механизма реализована с помощью Custom Resource `PrometheusRemoteWrite`.

### Пример минимального PrometheusRemoteWrite

```yaml
apiVersion: deckhouse.io/v1
kind: PrometheusRemoteWrite
metadata:
  name: test-remote-write
spec:
  url: https://victoriametrics-test.domain.com/api/v1/write
```

### Пример расширенного PrometheusRemoteWrite

```yaml
apiVersion: deckhouse.io/v1
kind: PrometheusRemoteWrite
metadata:
  name: test-remote-write
spec:
  url: https://victoriametrics-test.domain.com/api/v1/write
  basicAuth:
    username: username
    password: password
  writeRelabelConfigs:
  - sourceLabels: [__name__]
    action: keep
    regex: prometheus_build_.*|my_cool_app_metrics_.*
  - sourceLabels: [__name__]
    action: drop
    regex: my_cool_app_metrics_with_sensitive_data
```

## Подключение Prometheus к сторонней Grafana

У каждого `ingress-nginx-controller` есть сертификаты, при указании которых в качестве клиентских будет разрешено подключение к Prometheus. Всё что нужно - создать дополнительный `Ingress`-ресурс.

> В приведенном ниже примере предполагается, что Secret `example-com-tls` уже существует в namespace d8-monitoring.
>
> Имена для Ingress `my-prometheus-api` и Secret `my-basic-auth-secret` указаны для примера. Замените их на более подходящие в вашем случае.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-prometheus-api
  namespace: d8-monitoring
  annotations:
    nginx.ingress.kubernetes.io/backend-protocol: HTTPS
    nginx.ingress.kubernetes.io/auth-type: basic
    nginx.ingress.kubernetes.io/auth-secret: my-basic-auth-secret
    nginx.ingress.kubernetes.io/app-root: /graph
    nginx.ingress.kubernetes.io/configuration-snippet: |
      proxy_ssl_certificate /etc/nginx/ssl/client.crt;
      proxy_ssl_certificate_key /etc/nginx/ssl/client.key;
      proxy_ssl_protocols TLSv1.2;
      proxy_ssl_session_reuse on;
spec:
  ingressClassName: nginx
  rules:
  - host: prometheus-api.example.com
    http:
      paths:
      - backend:
          service:
            name: prometheus
            port:
              name: https
        path: /
        pathType: ImplementationSpecific
  tls:
  - hosts:
    - prometheus-api.example.com
    secretName: example-com-tls
---
apiVersion: v1
kind: Secret
metadata:
  name: my-basic-auth-secret
  namespace: d8-monitoring
type: Opaque
data:
  auth: Zm9vOiRhcHIxJE9GRzNYeWJwJGNrTDBGSERBa29YWUlsSDkuY3lzVDAK  # foo:bar
```

Далее остается только добавить data source в Grafana:

**В качестве URL необходимо указать `https://prometheus-api.<домен-вашего-кластера>`**

<img src="../../images/300-prometheus/prometheus_connect_settings.png" height="500">

* **Basic-авторизация** не является надежной мерой безопасности. Рекомендуется ввести дополнительные меры безопасности, например указать аннотацию `nginx.ingress.kubernetes.io/whitelist-source-range`.

* **Огромный минус** подключения таким способом - необходимость создания Ingress-ресурса в системном namespace'е.
Deckhouse **не гарантирует** сохранение работоспособности данной схемы подключения в связи с его активными постоянными обновлениями.

* Этот Ingress-ресурс может быть использован для доступа к Prometheus API не только для Grafana, но и для других интеграций, например, для федерации Prometheus.

## Подключение стороннего приложения к Prometheus

Подключение к Prometheus защищено при помощи [kube-rbac-proxy](https://github.com/brancz/kube-rbac-proxy). Для подключения понадобится создать `ServiceAccount` с необходимыми правами.

```yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: app
  namespace: default
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: app:prometheus-access
rules:
- apiGroups: ["monitoring.coreos.com"]
  resources: ["prometheuses/http"]
  resourceNames: ["main", "longterm"]
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: app:prometheus-access
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: app:prometheus-access
subjects:
- kind: ServiceAccount
  name: app
  namespace: default
```

Далее сделаем запрос используя `curl`:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: app-curl
  namespace: default
spec:
  template:
    metadata:
      name: app-curl
    spec:
      serviceAccountName: app
      containers:
      - name: app-curl
        image: curlimages/curl:7.69.1
        command: ["sh", "-c"]
        args:
        - >-
          curl -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" -k -f
          https://prometheus.d8-monitoring:9090/api/v1/query_range?query=up\&start=1584001500\&end=1584023100\&step=30
      restartPolicy: Never
  backoffLimit: 4
```

`Job` должен завершиться успешно.

## Отправка алертов в Telegram

Prometheus-operator не поддерживает прямую отправку алертов в Telegram, поэтому Alertmanager настраивается на отправку алертов через webhook в приложение, которое отправляет полученные данные в Telegram.

Задеплойте приложение, которое отправляет полученные от webhook данные в Telegram:

```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
   name: telegram-alertmanager
   namespace: d8-monitoring
   labels:
     app: telegram
spec:
   template:
     metadata:
       name: telegram-alertmanager
       labels:
         app: telegram
     spec:
       containers:
         - name: telegram-alertmanager
           image: janwh/alertmanager-telegram
           ports:
             - containerPort: 8080
           env:
             - name: TELEGRAM_CHAT_ID
               value: "-30490XXXXX"
             - name: TELEGRAM_TOKEN
               value: "562696849:AAExcuJ8H6z4pTlPuocbrXXXXXXXXXXXx"
   replicas: 1
   selector:
     matchLabels:
       app: telegram
---
apiVersion: v1
kind: Service
metadata:
 labels:
   app: telegram
 name: telegram-alertmanager
 namespace: d8-monitoring
spec:
 type: ClusterIP
 selector:
   app: telegram
 ports:
   - protocol: TCP
     port: 8080
```

`TELEGRAM_CHAT_ID` и `TELEGRAM_TOKEN` необходимо поставить свои. [Подробнее](https://core.telegram.org/bots) о Telegram API.

Задеплойте CRD CustomAlertManager:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: webhook
spec:
  internal:
    receivers:
    - name: webhook
      webhookConfigs:
      - sendResolved: true
        url: http://telegram-alertmanager:8080/alerts
    route:
      groupBy:
      - job
      groupInterval: 5m
      groupWait: 30s
      receiver: webhook
      repeatInterval: 12h
  type: Internal
```
