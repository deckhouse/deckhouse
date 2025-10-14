---
title: "Prometheus-мониторинг: примеры конфигурации"
type:
  - instruction
search: prometheus remote write, как подключиться к Prometheus, пользовательская Grafana, prometheus remote write
---

{% raw %}

## Пример конфигурации модуля

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

## Запись данных Prometheus в longterm storage

У Prometheus есть поддержка remote_write данных из локального Prometheus в отдельный longterm storage.

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

У каждого `ingress-nginx-controller` есть сертификаты, при указании которых в качестве клиентских будет разрешено подключение к Prometheus. Все, что нужно, — создать дополнительный `Ingress`-ресурс.

{% endraw -%}
{% alert level="info" %}
В приведенном ниже примере предполагается, что Secret `example-com-tls` уже существует в namespace d8-monitoring.

Имена для Ingress `my-prometheus-api` и Secret `my-basic-auth-secret` указаны для примера. Замените их на более подходящие для вас.
{% endalert %}
{% raw -%}

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
  # Строка basic-auth хешируется с помощью htpasswd.
  auth: Zm9vOiRhcHIxJE9GRzNYeWJwJGNrTDBGSERBa29YWUlsSDkuY3lzVDAK  # foo:bar
```

Добавьте data source в Grafana:

{% endraw -%}
{% alert level="info" %}
В качестве URL необходимо указать `https://prometheus-api.<домен-вашего-кластера>`**
{% endalert %}
{% raw -%}

<img src="images/prometheus_connect_settings.png" height="500">

* **Basic-авторизация** не является надежной мерой безопасности. Рекомендуется ввести дополнительные меры безопасности, например указать аннотацию `nginx.ingress.kubernetes.io/whitelist-source-range`.

* Из-за необходимости создания Ingress-ресурса в системном пространстве имён подключение таким способом **не рекомендуется**.
  Deckhouse **не гарантирует** сохранение работоспособности данной схемы подключения в связи с его активными постоянными обновлениями.

* Этот Ingress-ресурс может быть использован для доступа к Prometheus API не только для Grafana, но и для других интеграций, например, для федерации Prometheus.

## Подключение стороннего приложения к Prometheus

Подключение к Prometheus защищено с помощью kube-rbac-proxy. Для подключения создайте `ServiceAccount` с необходимыми правами.

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

Выполните запрос, используя команду `curl`:

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

Alertmanager поддерживает прямую отправку алертов в Telegram.

Создайте Secret в пространстве имен `d8-monitoring`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: telegram-bot-secret
  namespace: d8-monitoring
stringData:
  token: "562696849:AAExcuJ8H6z4pTlPuocbrXXXXXXXXXXXx"
```

Разверните кастомный ресурс `CustomAlertManager`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: telegram
spec:
  type: Internal
  internal:
    receivers:
      - name: telegram
        telegramConfigs:
          - botToken:
              name: telegram-bot-secret
              key: token
            chatID: -30490XXXXX
    route:
      groupBy:
        - job
      groupInterval: 5m
      groupWait: 30s
      receiver: telegram
      repeatInterval: 12h
```

Поля `token` в Secret'е и `chatID` в ресурсе `CustomAlertmanager` необходимо поставить свои.


## Пример отправки алертов по электронной почте

Создайте Secret с паролем от аккаунта электронной почты. Пароль, закодированный в формате base64, укажите в поле `password`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: am-mail-server-pass
  namespace: d8-monitoring
data:
  password: BASE64_ENCODED_PASSWORD_HERE
```

Измените значения в примере `CustomAlertManager` на необходимые вам и примените ресурс:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: mail
spec:
  type: Internal
  internal:
    receivers:
      - name: devnull
      - name: mail
        emailConfigs:
          - to: oncall@example.com
            from: prom@example.com
            smarthost: mx.example.com:587
            authIdentity: prom@example.com
            authUsername: prom@example.com
            authPassword:
              key: password
              name: am-mail-server-pass
            # Если вы используете custom CA на сервере, можете поместить публичную часть CA в ConfigMap в пространстве имен d8-monitoring
            # tlsConfig:
            #   insecureSkipVerify: true
            #   ca:
            #     configMap:
            #       key: ca.pem
            #       name: alertmanager-mail-server-ca
            sendResolved: true
            requireTLS: true
    route:
      groupBy:
        - job
      groupInterval: 5m
      groupWait: 30s
      receiver: devnull
      repeatInterval: 24h
      routes:
        - matchers:
          - matchType: =~
            name: severity_level
            value: "^[1-4]$"
          receiver: mail
```

{% endraw %}
