---
title: "Prometheus-мониторинг: FAQ"
type:
  - instruction
search: prometheus мониторинг, prometheus custom alert, prometheus кастомный алертинг
---


## Как собирать метрики с приложений, расположенных вне кластера?

1. Сконфигурировать Service, по аналогии с сервисом для [сбора метрик с вашего приложения](../../modules/340-monitoring-custom/#пример-service), но без указания параметра `spec.selector`.
1. Создать Endpoints для этого Service, явно указав в них `IP:PORT`, по которым ваши приложения отдают метрики.
> Важный момент: имена портов в Endpoints должны совпадать с именами этих портов в Service.

### Пример

Метрики приложения доступны без TLS, по адресу `http://10.182.10.5:9114/metrics`.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  namespace: my-namespace
  labels:
    prometheus.deckhouse.io/custom-target: my-app
spec:
  ports:
  - name: http-metrics
    port: 9114
---
apiVersion: v1
kind: Endpoints
metadata:
  name: my-app
  namespace: my-namespace
subsets:
  - addresses:
    - ip: 10.182.10.5
    ports:
    - name: http-metrics
      port: 9114
```

## Как добавить дополнительные dashboard'ы в вашем проекте?

Добавление пользовательских dashboard'ов для Grafana в Deckhouse реализовано при помощи подхода infrastructure as a code.
Чтобы ваш dashboard появился в Grafana, необходимо создать в кластере специальный ресурс — [`GrafanaDashboardDefinition`](cr.html#grafanadashboarddefinition).

Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: GrafanaDashboardDefinition
metadata:
  name: my-dashboard
spec:
  folder: My folder # Папка, в которой в Grafana будет отображаться ваш dashboard
  definition: |
    {
      "annotations": {
        "list": [
          {
            "builtIn": 1,
            "datasource": "-- Grafana --",
            "enable": true,
            "hide": true,
            "iconColor": "rgba(0, 211, 255, 1)",
            "limit": 100,
...
```

**Важно!** Системные и добавленные через [GrafanaDashboardDefinition](cr.html#grafanadashboarddefinition) dashboard нельзя изменить через интерфейс Grafana.

## Как добавить алерты и/или recording правила для вашего проекта?

Для добавления алертов существует специальный ресурс — `CustomPrometheusRules`.

Параметры:
- `groups` — единственный параметр, в котором необходимо описать группы алертов. Структура групп полностью совпадает с [аналогичной в prometheus-operator](https://github.com/coreos/prometheus-operator/blob/ed9e365370603345ec985b8bfb8b65c242262497/Documentation/api.md#rulegroup).

Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  name: my-rules
spec:
  groups:
  - name: cluster-state-alert.rules
    rules:
    - alert: CephClusterErrorState
      annotations:
        description: Storage cluster is in error state for more than 10m.
        summary: Storage cluster is in error state
        plk_markup_format: markdown
      expr: |
        ceph_health_status{job="rook-ceph-mgr"} > 1
```

### Как подключить дополнительные data source для Grafana?

Для подключения дополнительных data source к Grafana существует специальный ресурс — `GrafanaAdditionalDatasource`.

Параметры ресурса подробно описаны в [документации к Grafana](https://grafana.com/docs/grafana/latest/administration/provisioning/#example-datasource-config-file). Тип ресурса, смотрите в документации по конкретному [datasource](https://grafana.com/docs/grafana/latest/datasources/).

Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: GrafanaAdditionalDatasource
metadata:
  name: another-prometheus
spec:
  type: prometheus
  access: Proxy
  url: https://another-prometheus.example.com/prometheus
  basicAuth: true
  basicAuthUser: foo
  jsonData:
    timeInterval: 30s
    httpMethod: POST
  secureJsonData:
    basicAuthPassword: bar
```

## Как обеспечить безопасный доступ к метрикам?

Для обеспечения безопасности настоятельно рекомендуем использовать **kube-rbac-proxy**.

## Как добавить Alertmanager?

Создайте custom resource `CustomAlertmanager` с типом `Internal`.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: webhook
spec:
  type: Internal
  internal:
    route:
      groupBy: ['job']
      groupWait: 30s
      groupInterval: 5m
      repeatInterval: 12h
      receiver: 'webhook'
    receivers:
    - name: 'webhook'
      webhookConfigs:
      - url: 'http://webhookserver:8080/'
```

Подробно о всех параметрах можно прочитать в описании custom resource [CustomAlertmanager](cr.html#customalertmanager).

## Как добавить внешний дополнительный Alertmanager?

Создайте custom resource `CustomAlertmanager` с типом `External`, который может указывать на Alertmanager по FQDN или через сервис в Kubernetes-кластере.

Пример FQDN Alertmanager:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: my-fqdn-alertmanager
spec:
  external:
    address: https://alertmanager.mycompany.com/myprefix
  type: External
```

Пример Alertmanager с Kubernetes service:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: my-service-alertmanager
spec:
  external:
    service: 
      namespace: myns
      name: my-alertmanager
      path: /myprefix/
  type: External
```

Подробно о всех параметрах можно прочитать в описании Custom Resource [CustomAlertmanager](cr.html#customalertmanager)

## Как в Alertmanager игнорировать лишние алерты?

Решение сводится к настройке маршрутизации алертов в вашем Alertmanager.

Потребуется:
1. Завести получателя без параметров.
1. Смаршрутизировать лишние алерты в этого получателя.

В `alertmanager.yaml` это будет выглядеть так:

```yaml
receivers:
- name: blackhole
  # Получатель определенный без параметров будет работать как "/dev/null".
- name: some-other-receiver
  # ...
route:
  routes:
  - match:
      alertname: DeadMansSwitch
    receiver: blackhole
  - match_re:
      service: ^(foo1|foo2|baz)$
    receiver: blackhole
  - receiver: some-other-receiver
```

С подробным описанием всех параметров можно ознакомиться в [официальной документации](https://prometheus.io/docs/alerting/latest/configuration/#configuration-file).

## Почему нельзя установить разный scrapeInterval для отдельных таргетов?

Наиболее [полный ответ](https://www.robustperception.io/keep-it-simple-scrape_interval-id) на этот вопрос даёт разработчик Prometheus Brian Brazil.
Если коротко, то разные scrapeInterval'ы принесут следующие проблемы:
* Увеличение сложности конфигурации;
* Проблемы при написании запросов и создании графиков;
* Короткие интервалы больше похожи на профилирование приложения, и, скорее всего, Prometheus — не самый подходящий инструмент для этого.

Наиболее разумное значение для scrapeInterval находится в диапазоне 10-60 секунд.

## Как ограничить потребление ресурсов Prometheus?

Чтобы избежать ситуаций, когда VPA запрашивает для Prometheus или Longterm Prometheus ресурсов больше чем есть на выделенном для этого узле, можно явно ограничить VPA с помощью [параметров модуля](configuration.html):
- `vpa.longtermMaxCPU`
- `vpa.longtermMaxMemory`
- `vpa.maxCPU`
- `vpa.maxMemory`

## Как получить доступ к метрикам Prometheus из Lens?

> ⛔ **_Внимание!!!_** Использование данной конфигурации создает сервис в котором метрики Prometheus доступны без авторизации.

Для обеспечения доступа Lens к метрикам Prometheus, необходимо создать в кластере ряд ресурсов.

{% offtopic title="Шаблоны ресурсов, которые необходимо применить..." %}

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: lens-proxy
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: prometheus-lens-proxy
  namespace: lens-proxy
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: prometheus-lens-proxy:prometheus-access
rules:
- apiGroups: ["monitoring.coreos.com"]
  resources: ["prometheuses/http"]
  resourceNames: ["main", "longterm"]
  verbs: ["get", "create", "update"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: prometheus-lens-proxy:prometheus-access
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: prometheus-lens-proxy:prometheus-access
subjects:
- kind: ServiceAccount
  name: prometheus-lens-proxy
  namespace: lens-proxy
---
apiVersion: v1
kind: Secret
metadata:
  name: prometheus-lens-proxy-sa
  namespace: lens-proxy
  annotations:
    kubernetes.io/service-account.name: prometheus-lens-proxy
type: kubernetes.io/service-account-token
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-lens-proxy-conf
  namespace: lens-proxy
data:
  "39-log-format.sh": |
    cat > /etc/nginx/conf.d/log-format.conf <<"EOF"
    log_format  body  '$remote_addr - $remote_user [$time_local] "$request" '
                      '$status $body_bytes_sent "$http_referer" '
                      '"$http_user_agent" "$http_x_forwarded_for"'
                      ' req body: $request_body';
    EOF
  "40-prometheus-proxy-conf.sh": |
    #!/bin/sh
    prometheus_service="$(getent hosts prometheus.d8-monitoring | awk '{print $2}')"
    nameserver="$(awk '/nameserver/{print $2}' < /etc/resolv.conf)"
    cat > /etc/nginx/conf.d/prometheus.conf <<EOF
    server {
      listen 80 default_server;
      resolver ${nameserver} valid=30s;
      set \$upstream ${prometheus_service};
      location / {
        proxy_http_version 1.1;
        proxy_set_header Authorization "Bearer ${BEARER_TOKEN}";
        proxy_pass https://\$upstream:9090$request_uri;
      }
      access_log /dev/stdout body;
    }
    EOF
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: prometheus-lens-proxy
  namespace: lens-proxy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: prometheus-lens-proxy
  template:
    metadata:
      labels:
        app: prometheus-lens-proxy
    spec:
      containers:
      - name: nginx
        image: nginx:1.21.4-alpine
        env:
        - name: BEARER_TOKEN
          valueFrom:
            secretKeyRef:
              name: prometheus-lens-proxy-sa
              key: token
        ports:
        - containerPort: 80
        volumeMounts:
        - mountPath: /docker-entrypoint.d/40-prometheus-proxy-conf.sh
          subPath: "40-prometheus-proxy-conf.sh"
          name: prometheus-lens-proxy-conf
        - mountPath: /docker-entrypoint.d/39-log-format.sh
          name: prometheus-lens-proxy-conf
          subPath: 39-log-format.sh
      serviceAccountName: prometheus-lens-proxy
      volumes:
      - name: prometheus-lens-proxy-conf
        configMap:
          name: prometheus-lens-proxy-conf
          defaultMode: 0755
---
apiVersion: v1
kind: Service
metadata:
  name: prometheus-lens-proxy
  namespace: lens-proxy
spec:
  selector:
    app: prometheus-lens-proxy
  ports:
    - protocol: TCP
      port: 8080
      targetPort: 80
```

{% endofftopic %}

После деплоя ресурсов, метрики Prometheus будут доступны по адресу `lens-proxy/prometheus-lens-proxy:8080`.
Тип Prometheus в Lens — `Prometheus Operator`.

Начиная с версии `5.2.7`, Lens требует наличия меток `pod` и `namespace` в метриках node-exporter'а.
В противном случае потребление ресурсов узла не будет отображаться на диаграммах Lens.

Чтобы исправить это, примените следующий ресурс:

{% offtopic title="Ресурс, исправляющий отображение метрик..." %}

```yaml
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  name: lens-hack
spec:
  groups:
  - name: lens-hack
    rules:
    - expr: node_cpu_seconds_total{mode=~"user|system", pod!~".+", namespace!~".+"}
        * on(node) group_left(namespace, pod) kube_pod_info{namespace="d8-monitoring",
        created_by_name="node-exporter"}
      record: node_cpu_seconds_total
    - expr: node_filesystem_size_bytes{mountpoint="/", pod!~".+", namespace!~".+"}
        * on(node) group_left(namespace, pod) kube_pod_info{namespace="d8-monitoring",
        created_by_name="node-exporter"}
      record: node_filesystem_size_bytes
    - expr: node_filesystem_avail_bytes{mountpoint="/", pod!~".+", namespace!~".+"}
        * on(node) group_left(namespace, pod) kube_pod_info{namespace="d8-monitoring",
        created_by_name="node-exporter"}
      record: node_filesystem_avail_bytes
    - expr: node_memory_MemTotal_bytes{pod!~".+", namespace!~".+"} * on(node) group_left(namespace,
        pod) kube_pod_info{namespace="d8-monitoring", created_by_name="node-exporter"}
      record: node_memory_MemTotal_bytes
    - expr: node_memory_MemFree_bytes{pod!~".+", namespace!~".+"} * on(node) group_left(namespace,
        pod) kube_pod_info{namespace="d8-monitoring", created_by_name="node-exporter"}
      record: node_memory_MemFree_bytes
    - expr: node_memory_Buffers_bytes{pod!~".+", namespace!~".+"} * on(node) group_left(namespace,
        pod) kube_pod_info{namespace="d8-monitoring", created_by_name="node-exporter"}
      record: node_memory_Buffers_bytes
    - expr: node_memory_Cached_bytes{pod!~".+", namespace!~".+"} * on(node) group_left(namespace,
        pod) kube_pod_info{namespace="d8-monitoring", created_by_name="node-exporter"}
      record: node_memory_Cached_bytes
```

{% endofftopic %}

## Как настроить ServiceMonitor или PodMonitor для работы с Prometheus?

Добавьте лейбл `prometheus: main` к Pod/Service Monitor.
Добавьте в namespace, в котором находится Pod/Service Monitor, лейбл `prometheus.deckhouse.io/monitor-watcher-enabled: "true"`.

Пример:

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: frontend
  labels:
    prometheus.deckhouse.io/monitor-watcher-enabled: "true"
---
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
```

## Как увеличить размер диска

1. Для увеличения размера отредактируйте PersistentVolumeClaim, указав новый размер в поле `spec.resources.requests.storage`.
    * Увеличение размера возможно если в StorageClass поле `allowVolumeExpansion` установлено в `true`.
2. Когда размер диска будет изменен, в статусе PersistentVolumeClaim появится сообщение `Waiting for user to (re-)start a pod to finish file system resize of volume on node.`.
3. Перезапустите Pod для завершения изменения размера файловой системы.
