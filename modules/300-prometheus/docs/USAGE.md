---
title: "The Prometheus monitoring module: usage"
type:
  - instruction
search: prometheus remote write, how to connect to Prometheus, custom Grafana, prometheus remote write
---

{% raw %}

## An example of the module configuration

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
      node-role/example: ""
    tolerations:
    - key: dedicated
      operator: Equal
      value: example
```

## Writing Prometheus data to the longterm storage

Prometheus supports remote_write'ing data from the local Prometheus to a separate longterm storage (e.g., [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics)). In Deckhouse, this mechanism is implemented using the `PrometheusRemoteWrite` custom resource.

### Example of the basic PrometheusRemoteWrite

```yaml
apiVersion: deckhouse.io/v1
kind: PrometheusRemoteWrite
metadata:
  name: test-remote-write
spec:
  url: https://victoriametrics-test.domain.com/api/v1/write
```

### Example of the expanded PrometheusRemoteWrite

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

## Connecting Prometheus to an external Grafana instance

Each `ingress-nginx-controller` has certificates that can be used to connect to Prometheus. All you need is to create an additional `Ingress` resource.

> For the example below, it is presumed that Secret `example-com-tls` already exist in namespace d8-monitoring.
>
> Names for Ingress `my-prometheus-api` and Secret `my-basic-auth-secret` are there for example. Change them to the most suitable names for your case.

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
  # Basic-auth string is hashed using htpasswd.
  auth: Zm9vOiRhcHIxJE9GRzNYeWJwJGNrTDBGSERBa29YWUlsSDkuY3lzVDAK  # foo:bar
```

Next, you only need to add the data source to Grafana:

**Set `https://prometheus-api.<cluster-domain>` as the URL**.

<img src="../../images/300-prometheus/prometheus_connect_settings.png" height="500">

* Note that **basic authorization** is not sufficiently secure and safe. You are encouraged to implement additional safety measures, e.g., attach the `nginx.ingress.kubernetes.io/whitelist-source-range` annotation.

* A **considerable disadvantage** of this method is the need to create an Ingress resource in the system namespace.
Deckhouse does **not guarantee** the functionality of this connection method due to its regular updates.

* This Ingress resource can be used to access the Prometheus API not only from Grafana but for other integrations, e.g., the Prometheus federation.

## Connecting an external app to Prometheus

The connection to Prometheus is protected using [kube-rbac-proxy](https://github.com/brancz/kube-rbac-proxy). To connect, you need to create a `ServiceAccount` with the necessary permissions.

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

Next, define the following job containing the `curl` request:

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

The `job` must complete successfully.

## Sending alerts to Telegram

Alertmanager supports sending alerts to Telegram directly.

Create the Secret in the `d8-monitoring` namespace:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: telegram-bot-secret
  namespace: d8-monitoring
stringData:
  token: "562696849:AAExcuJ8H6z4pTlPuocbrXXXXXXXXXXXx"
```

Deploy CustomAlertManager CR:

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

The fields `token` in the Secret and `chatID` in the `CustomAlertmanager` custom resource must be set on your own. [Read more](https://core.telegram.org/bots) about Telegram API.

## Example of sending alerts to Slack with a filter

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: slack
spec:
  internal:
    receivers:
    - name: devnull
    - name: slack
      slackConfigs:
      - apiURL:
          key: apiURL
          name: slack-apiurl
        channel: {{ dig .Values.werf.env .Values.slack.channel._default .Values.slack.channel }} 
        fields:
        - short: true
          title: Severity
          value: '{{`{{  .CommonLabels.severity_level }}`}}'
        - short: true
          title: Status
          value: '{{`{{ .Status }}`}}'
        - title: Summary
          value: '{{`{{ range .Alerts }}`}}{{`{{ .Annotations.summary }}`}} {{`{{ end }}`}}'
        - title: Description
          value: '{{`{{ range .Alerts }}`}}{{`{{ .Annotations.description }}`}} {{`{{ end }}`}}'
        - title: Labels
          value: '{{`{{ range .Alerts }}`}} {{`{{ range .Labels.SortedPairs }}`}}{{`{{ printf "%s:
            %s\n" .Name .Value }}`}}{{`{{ end }}`}}{{`{{ end }}`}}'
        - title: Links
          value: '{{`{{ (index .Alerts 0).GeneratorURL }}`}}'
        title: '{{`{{ .CommonLabels.alertname }}`}}'
    route:
      groupBy:
      - '...'  
      receiver: devnull
      routes:
        - matchers:
          - matchType: =~
            name: severity_level
            value: "^[4-9]$"
          receiver: slack
      repeatInterval: 12h
  type: Internal
```

## Example of sending alerts to Opsgenie

```yaml
- name: opsgenie
        opsgenieConfigs:
          - apiKey:
              key: data
              name: opsgenie
            description: |
              {{ range .Alerts }}{{ .Annotations.summary }} {{ end }}
              {{ range .Alerts }}{{ .Annotations.description }} {{ end }}
            message: '{{ .CommonLabels.alertname }}'
            priority: P1
            responders:
              - id: team_id
                type: team
```

{% endraw %}
