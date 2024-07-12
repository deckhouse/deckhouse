---
title: "Prometheus-мониторинг: FAQ"
type:
  - instruction
search: prometheus мониторинг, prometheus custom alert, prometheus кастомный алертинг
---

{% raw %}

## Как собирать метрики с приложений, расположенных вне кластера?

1. Сконфигурировать Service по аналогии с сервисом для [сбора метрик с вашего приложения](../../modules/340-monitoring-custom/#пример-service), но без указания параметра `spec.selector`.
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

Добавление пользовательских dashboard'ов для Grafana в Deckhouse реализовано с помощью подхода Infrastructure as a Code.
Чтобы ваш dashboard появился в Grafana, необходимо создать в кластере специальный ресурс — [`GrafanaDashboardDefinition`](cr.html#grafanadashboarddefinition).

Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: GrafanaDashboardDefinition
metadata:
  name: my-dashboard
spec:
  folder: My folder # Папка, в которой в Grafana будет отображаться ваш dashboard.
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

**Важно!** Системные и добавленные через [GrafanaDashboardDefinition](cr.html#grafanadashboarddefinition) dashboard'ы нельзя изменить через интерфейс Grafana.

## Как добавить алерты и/или recording-правила для вашего проекта?

Для добавления алертов существует специальный ресурс — `CustomPrometheusRules`.

Параметры:
- `groups` — единственный параметр, в котором необходимо описать группы алертов. Структура групп полностью совпадает [с аналогичной в prometheus-operator](https://github.com/prometheus-operator/prometheus-operator/blob/ed9e365370603345ec985b8bfb8b65c242262497/Documentation/api.md#rulegroup).

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

Параметры ресурса подробно описаны [в документации к Grafana](https://grafana.com/docs/grafana/latest/administration/provisioning/#example-datasource-config-file). Тип ресурса смотрите в документации по конкретному [datasource](https://grafana.com/docs/grafana/latest/datasources/).

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

Для обеспечения безопасности настоятельно рекомендуем использовать `kube-rbac-proxy`.

### Пример безопасного сбора метрик с приложения, расположенного в кластере

Для настройки защиты метрик приложения с использованием `kube-rbac-proxy` и последующей сборки метрик с него средствами Prometheus выполните следующие шаги:

1. Создайте `ServiceAccount` с указанными ниже правами:

   ```yaml
   ---
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: rbac-proxy-test
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRoleBinding
   metadata:
     name: rbac-proxy-test
   roleRef:
     apiGroup: rbac.authorization.k8s.io
     kind: ClusterRole
     name: d8:rbac-proxy
   subjects:
   - kind: ServiceAccount
     name: rbac-proxy-test
     namespace: default
   ```

   > Обратите внимание, что используется встроенная в Deckhouse ClusterRole `d8:rbac-proxy`.

2. Создайте конфигурацию для `kube-rbac-proxy`:

   ```yaml
   ---
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: rbac-proxy-config-test
     namespace: rbac-proxy-test
   data:
     config-file.yaml: |+
       authorization:
         resourceAttributes:
           namespace: default
           apiVersion: v1
           resource: services
           subresource: proxy
           name: rbac-proxy-test
   ```

   > Более подробную информацию по атрибутам можно найти [в документации Kubernetes](https://kubernetes.io/docs/reference/access-authn-authz/authorization).

3. Создайте `Service` и `Deployment` для вашего приложения, где `kube-rbac-proxy` займет позицию sidecar-контейнера:

   ```yaml
   ---
   apiVersion: v1
   kind: Service
   metadata:
     name: rbac-proxy-test
     labels:
       prometheus.deckhouse.io/custom-target: rbac-proxy-test
   spec:
     ports:
     - name: https-metrics
       port: 8443
       targetPort: https-metrics
     selector:
       app: rbac-proxy-test
   ---
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: rbac-proxy-test
   spec:
     replicas: 1
     selector:
       matchLabels:
         app: rbac-proxy-test
     template:
       metadata:
         labels:
           app: rbac-proxy-test
       spec:
         securityContext:
           runAsUser: 65532
         serviceAccountName: rbac-proxy-test
         containers:
         - name: kube-rbac-proxy
           image: quay.io/brancz/kube-rbac-proxy:v0.14.0
           args:
           - "--secure-listen-address=0.0.0.0:8443"
           - "--upstream=http://127.0.0.1:8081/"
           - "--config-file=/kube-rbac-proxy/config-file.yaml"
           - "--logtostderr=true"
           - "--v=10"
           ports:
           - containerPort: 8443
             name: https-metrics
           volumeMounts:
           - name: config
             mountPath: /kube-rbac-proxy
         - name: prometheus-example-app
           image: quay.io/brancz/prometheus-example-app:v0.1.0
           args:
           - "--bind=127.0.0.1:8081"
         volumes:
         - name: config
           configMap:
             name: rbac-proxy-config-test
   ```

4. Назначьте необходимые права на ресурс для Prometheus:

   ```yaml
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: rbac-proxy-test-client
   rules:
   - apiGroups: [""]
     resources: ["services/proxy"]
     verbs: ["get"]
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRoleBinding
   metadata:
     name: rbac-proxy-test-client
   roleRef:
     apiGroup: rbac.authorization.k8s.io
     kind: ClusterRole
     name: rbac-proxy-test-client
   subjects:
   - kind: ServiceAccount
     name: prometheus
     namespace: d8-monitoring
   ```

После шага 4 метрики вашего приложения должны появиться в Prometheus.

### Пример безопасного сбора метрик с приложения, расположенного вне кластера

Предположим, что есть доступный через интернет сервер, на котором работает `node-exporter`. По умолчанию `node-exporter` слушает на порту `9100` и доступен на всех интерфейсах. Необходимо обеспечить контроль доступа к `node-exporter` для безопасного сбора метрик. Ниже приведен пример такой настройки.

Требования:
- Из кластера должен быть доступ до сервиса `kube-rbac-proxy`, запущенного на *удаленном сервере*.
- От *удаленного сервера* должен быть доступ до API-сервера кластера.

Выполните следующие шаги:
1. Создайте `ServiceAccount` с указанными ниже правами:

   ```yaml
   ---
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: prometheus-external-endpoint-server-01
     namespace: d8-service-accounts
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: prometheus-external-endpoint
   rules:
   - apiGroups: ["authentication.k8s.io"]
     resources:
     - tokenreviews
     verbs: ["create"]
   - apiGroups: ["authorization.k8s.io"]
     resources:
     - subjectaccessreviews
     verbs: ["create"]
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRoleBinding
   metadata:
     name: prometheus-external-endpoint-server-01
   roleRef:
     apiGroup: rbac.authorization.k8s.io
     kind: ClusterRole
     name: prometheus-external-endpoint
   subjects:
   - kind: ServiceAccount
     name: prometheus-external-endpoint-server-01
     namespace: d8-service-accounts
   ```

2. Сгенерируйте `kubeconfig` для созданного `ServiceAccount` ([пример генерации kubeconfig для `ServiceAccount`](https://deckhouse.ru/documentation/v1/modules/140-user-authz/usage.html#создание-serviceaccount-для-сервера-и-предоставление-ему-доступа)).

3. Положите получившийся `kubeconfig` на *удаленный сервер*. В дальнейшем понадобится указать путь к этому `kubeconfig` в настройках `kube-rbac-proxy` (в примере используется путь `${PWD}/.kube/config`).

4. Настройте `node-exporter` на *удаленном сервере*, чтобы он был доступен только на локальном интерфейсе (слушал `127.0.0.1:9100`).
5. Запустите `kube-rbac-proxy` на *удаленном сервере*:

   ```shell
   docker run --network host -d -v ${PWD}/.kube/config:/config quay.io/brancz/kube-rbac-proxy:v0.14.0 --secure-listen-address=0.0.0.0:8443 \
     --upstream=http://127.0.0.1:9100 --kubeconfig=/config --logtostderr=true --v=10
   ```

6. Проверьте, что порт `8443` доступен по внешнему адресу *удаленного сервера*.

7. Создайте в кластере `Service` и `Endpoint`, указав в качестве `<server_ip_address>` внешний адрес *удаленного сервера*:

   ```yaml
   ---
   apiVersion: v1
   kind: Service
   metadata:
     name: prometheus-external-endpoint-server-01
     labels:
       prometheus.deckhouse.io/custom-target: prometheus-external-endpoint-server-01
   spec:
     ports:
     - name: https-metrics
       port: 8443
   ---
   apiVersion: v1
   kind: Endpoints
   metadata:
     name: prometheus-external-endpoint-server-01
   subsets:
     - addresses:
       - ip: <server_ip_address>
       ports:
       - name: https-metrics
         port: 8443
   ```

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

Подробно о всех параметрах можно прочитать в описании custom resource [CustomAlertmanager](cr.html#customalertmanager).

## Как в Alertmanager игнорировать лишние алерты?

Решение сводится к настройке маршрутизации алертов в вашем Alertmanager.

Потребуется:

1. Завести получателя без параметров.
1. Смаршрутизировать лишние алерты в этого получателя.

Ниже приведены примеры настройки `CustomAlertmanager`.

Чтобы получать только алерты с лейблами `service: foo|bar|baz`:

```yaml
receivers:
  # Получатель, определенный без параметров, будет работать как "/dev/null".
  - name: blackhole
  # Действующий получатель  
  - name: some-other-receiver
    # ...
route:
  # receiver по умолчанию.
  receiver: blackhole
  routes:
    # Дочерний маршрут
    - matchers:
        - matchType: =~
          name: service
          value: ^(foo|bar|baz)$
      receiver: some-other-receiver
```

Чтобы получать все алерты, кроме `DeadMansSwitch`:

```yaml
receivers:
  # Получатель, определенный без параметров, будет работать как "/dev/null".
  - name: blackhole
  # Действующий получатель.
  - name: some-other-receiver
  # ...
route:
  # receiver по умолчанию.
  receiver: some-other-receiver
  routes:
    # Дочерний маршрут.
    - matchers:
        - matchType: =
          name: alertname
          value: DeadMansSwitch
      receiver: blackhole
```

С подробным описанием всех параметров можно ознакомиться [в официальной документации](https://prometheus.io/docs/alerting/latest/configuration/#configuration-file).

## Почему нельзя установить разный scrapeInterval для отдельных таргетов?

Наиболее [полный ответ](https://www.robustperception.io/keep-it-simple-scrape_interval-id) на этот вопрос дает разработчик Prometheus Brian Brazil.
Если коротко, разные scrapeInterval'ы принесут следующие проблемы:
* увеличение сложности конфигурации;
* проблемы при написании запросов и создании графиков;
* короткие интервалы больше похожи на профилирование приложения, и, скорее всего, Prometheus — не самый подходящий инструмент для этого.

Наиболее разумное значение для scrapeInterval находится в диапазоне 10–60 секунд.

## Как ограничить потребление ресурсов Prometheus?

Чтобы избежать ситуаций, когда VPA запрашивает для Prometheus или Longterm Prometheus ресурсов больше, чем есть на выделенном для этого узле, можно явно ограничить VPA с помощью [параметров модуля](configuration.html):
- `vpa.longtermMaxCPU`;
- `vpa.longtermMaxMemory`;
- `vpa.maxCPU`;
- `vpa.maxMemory`.

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

## Как настроить Probe для работы с Prometheus?

Добавьте лейбл `prometheus: main` к Probe.
Добавьте в namespace, в котором находится Probe, лейбл `prometheus.deckhouse.io/probe-watcher-enabled: "true"`.

Пример:

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: frontend
  labels:
    prometheus.deckhouse.io/probe-watcher-enabled: "true"
---
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

## Как настроить PrometheusRules для работы с Prometheus?

Добавьте в namespace, в котором находятся PrometheusRules, лейбл `prometheus.deckhouse.io/rules-watcher-enabled: "true"`.

Пример:

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: frontend
  labels:
    prometheus.deckhouse.io/rules-watcher-enabled: "true"
```

## Как увеличить размер диска

1. Для увеличения размера отредактируйте PersistentVolumeClaim, указав новый размер в поле `spec.resources.requests.storage`.
   * Увеличение размера возможно, если в StorageClass поле `allowVolumeExpansion` установлено в `true`.
2. Если используемое хранилище не поддерживает изменение диска на лету, в статусе PersistentVolumeClaim появится сообщение `Waiting for user to (re-)start a pod to finish file system resize of volume on node.`.
3. Перезапустите под для завершения изменения размера файловой системы.

## Как получить информацию об алертах в кластере?

Информацию об активных алертах можно получить не только в веб-интерфейсе Grafana/Prometheus, но и в CLI. Это может быть полезным, если у вас есть только доступ к API-серверу кластера и нет возможности открыть веб-интерфейс Grafana/Prometheus.

Выполните следующую команду для получения списка алертов в кластере:

```shell
kubectl get clusteralerts
```

Пример:

```shell
# kubectl get clusteralerts
NAME               ALERT                                      SEVERITY   AGE     LAST RECEIVED   STATUS
086551aeee5b5b24   ExtendedMonitoringDeprecatatedAnnotation   4          3h25m   38s             firing
226d35c886464d6e   ExtendedMonitoringDeprecatatedAnnotation   4          3h25m   38s             firing
235d4efba7df6af4   D8SnapshotControllerPodIsNotReady          8          5d4h    44s             firing
27464763f0aa857c   D8PrometheusOperatorPodIsNotReady          7          5d4h    43s             firing
ab17837fffa5e440   DeadMansSwitch                             4          5d4h    41s             firing
```

Выполните следующую команду для просмотра конкретного алерта:

```shell
kubectl get clusteralerts <ALERT_NAME> -o yaml
```

Пример:

```shell
# kubectl get clusteralerts 235d4efba7df6af4 -o yaml
alert:
  description: |
    The recommended course of action:
    1. Retrieve details of the Deployment: `kubectl -n d8-snapshot-controller describe deploy snapshot-controller`
    2. View the status of the Pod and try to figure out why it is not running: `kubectl -n d8-snapshot-controller describe pod -l app=snapshot-controller`
  labels:
    pod: snapshot-controller-75bd776d76-xhb2c
    prometheus: deckhouse
    tier: cluster
  name: D8SnapshotControllerPodIsNotReady
  severityLevel: "8"
  summary: The snapshot-controller Pod is NOT Ready.
apiVersion: deckhouse.io/v1alpha1
kind: ClusterAlert
metadata:
  creationTimestamp: "2023-05-15T14:24:08Z"
  generation: 1
  labels:
    app: prometheus
    heritage: deckhouse
  name: 235d4efba7df6af4
  resourceVersion: "36262598"
  uid: 817f83e4-d01a-4572-8659-0c0a7b6ca9e7
status:
  alertStatus: firing
  lastUpdateTime: "2023-05-15T18:10:09Z"
  startsAt: "2023-05-10T13:43:09Z"
```

Помните о специальном алерте `DeadMansSwitch` — его присутствие в кластере говорит о работоспособности Prometheus.

## Как добавить дополнительные эндпоинты в scrape config?

Добавьте в namespace, в котором находится ScrapeConfig, лейбл `prometheus.deckhouse.io/scrape-configs-watcher-enabled: "true"`.

Пример:

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: frontend
  labels:
    prometheus.deckhouse.io/scrape-configs-watcher-enabled: "true"
```

Добавьте ScrapeConfig, который имеет обязательный лейбл `prometheus: main`:

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
