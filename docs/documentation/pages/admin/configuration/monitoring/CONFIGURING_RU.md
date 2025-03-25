---
title: "Настройка мониторинга в Deckhouse Kubernetes Platform"
permalink: ru/admin/configuration/monitoring/configuring.html
lang: ru
---

{% raw %}

## Мониторинг сетевого взаимодействия

DKP может выполнять мониторинг сетевого взаимодействия между всеми узлами кластера, а также между узлами кластера и внешними хостами. При настроенном мониторинге, каждый узел два раза в секунду отправляет ICMP-пакеты на все другие узлы кластера (и на опциональные внешние узлы) и экспортирует данные в систему мониторинга.

Анализ результатов мониторинга можно выполнять с помощью дашбордов мониторинга:
- todo Какие-то концы по дашбордам

Модуль отслеживает любые изменения поля `.status.addresses` узла. Если они обнаружены, срабатывает хук, который собирает полный список имен узлов и их адресов, и передает в DaemonSet, который заново создает поды. Таким образом, `ping` проверяет всегда актуальный список узлов.

### Добавление дополнительных IP-адресов для мониторинга

Для добавления дополнительных IP-адресов мониторинга используйте параметр [externalTargets](configuration.html#parameters-externaltargets) модуля.

Пример конфигурации модуля:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: monitoring-ping
spec:
  version: 1
  enabled: true
  settings:
    externalTargets:
    - name: google-primary
      host: 8.8.8.8
    - name: yaru
      host: ya.ru
    - host: youtube.com
```

> Поле `name` используется в Grafana для отображения связанных данных. Если поле `name` не указано, используется обязательное поле `host`.

## Мониторинг кластера

DKP безопасно собирает метрики мониторинга и настраивает правила.

Возможности мониторинга DKP:
- мониторинг текущей версии container runtime (containerd) на узле и ее соответствия версиям, разрешенным для использования в DKP;
- мониторинг работоспособности подсистемы мониторинга кластера (Dead man's switch);
- мониторинг доступных файловых дескрипторов, сокетов, свободного места и inode;
- мониторинг состояния узлов кластера (NotReady, drain, cordon);
- работы `kube-state-metrics`, `node-exporter`, `kube-dns`;
- мониторинг состояния синхронизации времени на узлах;
- мониторинг случаев продолжительного превышения CPU steal;
- мониторинг состояния таблицы Conntrack на узлах;
- мониторинг подов с некорректным состоянием (как возможное следствие проблем с kubelet);
- мониторинг компонентов control plane (реализуется модулем `monitoring-kubernetes-control-plane`);
- мониторинг секретов в Кластере (объекты Secret) и срока действия TLS-сертификатов в них (реализуется модулем `extended-monitoring`);
- сбор событий в кластере Kubernetes в виде метрик (реализуется модулем `extended-monitoring`);
- мониторинг доступности образов контейнеров в registry, используемых контроллерах (Deployments, StatefulSets, DaemonSets, CronJobs) (реализуется модулем `extended-monitoring`);
- мониторинг объектов в пространствах имен, у которых есть лейбл `extended-monitoring.deckhouse.io/enabled=""` (реализуется модулем `extended-monitoring`).

Чтобы включить мониторинг узлов кластера, необходимо включить модуль `monitoring-kubernetes`, если он не включен. Включить мониторинг кластера можно в веб-интерфейсе (Deckhouse Console), или с помощью следующей команды:

```shell
d8 platform module enable monitoring-kubernetes
```

Аналогично можно включить модули `monitoring-kubernetes-control-plane` и `extended-monitoring`.

## Мониторинг приложения

Чтобы организовать сбор метрик с любых приложений в кластере, необходимо:

- Включить модуль `monitoring-custom`, если он не включен. 

  Включить мониторинг кластера можно в веб-интерфейсе (Deckhouse Console), или с помощью следующей комнанды:

  ```shell
  d8 platform module enable monitoring-custom
  ```

- Поставить лейбл `prometheus.deckhouse.io/custom-target` на Service или под. Значение лейбла определит имя в списке target'ов Prometheus.
  - В качестве значения лейбла `prometheus.deckhouse.io/custom-target` рекомендуется использовать название приложения (маленькими буквами, разделитель `-`), которое позволяет его уникально идентифицировать в кластере.

     Если приложение ставится в кластер больше одного раза (staging, testing и т. д.) или даже ставится несколько раз в одно пространство имён, достаточно одного общего названия, так как у всех метрик в любом случае будут лейблы `namespace`, `pod` и, если доступ осуществляется через Service, лейбл `service`. Это название, уникально идентифицирующее приложение в кластере, а не его единичную инсталляцию.
- Порту, с которого нужно собирать метрики, указать имя `http-metrics` и `https-metrics` для подключения по HTTP или HTTPS соответственно.

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

- При использовании service mesh [Istio](../istio/) в режиме STRICT mTLS указать для сбора метрик следующую аннотацию у Service или Pod: `prometheus.deckhouse.io/istio-mtls: "true"`. Важно, что метрики приложения должны экспортироваться по протоколу HTTP без TLS.

- *(Необязательно)* Укажите дополнительные аннотации для более тонкой настройки:

  * `prometheus.deckhouse.io/path` — путь для сбора метрик (по умолчанию: `/metrics`).
  * `prometheus.deckhouse.io/query-param-$name` — GET-параметры, будут преобразованы в map вида `$name=$value` (по умолчанию: ''):
    - возможно указать несколько таких аннотаций.

      Например, `prometheus.deckhouse.io/query-param-foo=bar` и `prometheus.deckhouse.io/query-param-bar=zxc` будут преобразованы в query: `http://...?foo=bar&bar=zxc`.
  * `prometheus.deckhouse.io/allow-unready-pod` — разрешает сбор метрик с подов в любом состоянии (по умолчанию метрики собираются только с подов в состоянии Ready). Эта опция полезна в редких случаях. Например, если ваше приложение запускается очень долго (при старте загружаются данные в базу или прогреваются кэши), но в процессе запуска уже отдаются полезные метрики, которые помогают следить за запуском приложения.
  * `prometheus.deckhouse.io/sample-limit` — сколько семплов разрешено собирать с пода (по умолчанию 5000). Значение по умолчанию защищает от ситуации, когда приложение внезапно начинает отдавать слишком большое количество метрик, что может нарушить работу всего мониторинга. Аннотация должна быть размещена на том же ресурсе, на котором висит лейбл  `prometheus.deckhouse.io/custom-target`.

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

## Запись данных Prometheus в longterm storage

У Prometheus есть поддержка remote_write данных из локального Prometheus в отдельный longterm storage (например, [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics)). В Deckhouse поддержка этого механизма реализована с помощью кастомного ресурса `PrometheusRemoteWrite`.

{% endraw -%}
{% alert level="info" %}
Для VictoriaMetrics подробную информацию о способах передачи данные в vmagent можно получить в [документации](https://docs.victoriametrics.com/vmagent/index.html#how-to-push-data-to-vmagent) VictoriaMetrics.
{% endalert %}
{% raw %}

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

{% alert level="info" %}
В приведенном ниже примере предполагается, что Secret `example-com-tls` уже существует в namespace d8-monitoring.

Имена для Ingress `my-prometheus-api` и Secret `my-basic-auth-secret` указаны для примера. Замените их на более подходящие для вас.
{% endalert %}

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

{% alert level="info" %}
В качестве URL необходимо указать `https://prometheus-api.<домен-вашего-кластера>`**
{% endalert %}

<img src="../../images/prometheus/prometheus_connect_settings.png" height="500">

* **Basic-авторизация** не является надежной мерой безопасности. Рекомендуется ввести дополнительные меры безопасности, например указать аннотацию `nginx.ingress.kubernetes.io/whitelist-source-range`.

* Из-за необходимости создания Ingress-ресурса в системном пространстве имён подключение таким способом **не рекомендуется**.
  Deckhouse **не гарантирует** сохранение работоспособности данной схемы подключения в связи с его активными постоянными обновлениями.

* Этот Ingress-ресурс может быть использован для доступа к Prometheus API не только для Grafana, но и для других интеграций, например, для федерации Prometheus.

## Подключение стороннего приложения к Prometheus

Подключение к Prometheus защищено с помощью [kube-rbac-proxy](https://github.com/brancz/kube-rbac-proxy). Для подключения создайте `ServiceAccount` с необходимыми правами.

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

## Сбор метрик через шлюз (Pushgateway)

Prometheus, находящийся в основе системы мониторинга DKP, преимущественно использует pull-модель для сбора метрик. При таком подходе происходит опрос экспортеров метрик со стороны DKP. Когда применение pull-модели затруднено, например, для сервисов без постоянного сетевого интерфейса, можно использовать сбор метрик через шлюз (Pushgateway). Pushgateway позволяет таким задачам самим отправлять метрики, которые затем могут быть собраны Prometheus. Важно отметить, что Pushgateway может стать единой точкой отказа и узким местом в системе. Как отправлять метрики из приложения в Pushgateway можно узнать [в документации Prometheus](https://prometheus.io/docs/instrumenting/pushing/).


Пример настройки сбора метрик через шлюз (Pushgateway):
- Включите и настройте модуль `prometheus-pushgateway`.

  Включить модуль можно в веб-интерфейсе (Deckhouse Console), или с помощью следующей команды:

  ```shell
  d8 platform module enable prometheus-pushgateway
  ```

- Укажите названия шлюзов в параметре `instances` модуля `prometheus-pushgateway` через веб-интерфейс, или с помощью следующей команды:

  ```shell
  kubectl edit mc prometheus-pushgateway
  ``` 

  Пример конфигурации модуля:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ModuleConfig
  metadata:
    name: prometheus-pushgateway
  spec:
    version: 1
    enabled: true
    settings:
      instances:
      - first
      - second
      - another
  ```
  
  Адрес экземпляра PushGateway с именем `first` из контейнера пода будет: `http://first.kube-prometheus-pushgateway:9091`.

- Проверьте отправку метрик.

  Пример отправки метрики через curl:

  ```shell
  echo "test_metric{env="dev"} 3.14" | curl --data-binary @- http://first.kube-prometheus-pushgateway:9091/metrics/job/myapp
  ```

- Проверьте что метрика появилась в системе мониторинга. Она будет доступна через 30 секунд после сбора данных.

  Пример PromQL запроса:

  ```text
  test_metric{container="prometheus-pushgateway", env="dev", exported_job="myapp", 
      instance="10.244.1.155:9091", job="prometheus-pushgateway", pushgateway="prometheus-pushgateway", tier="cluster"} 3.14
  ```


### Удаление метрик из шлюза (Pushgateway)

Пример удаления всех метрик группы `{instance="10.244.1.155:9091",job="myapp"}` через curl:

```shell
curl -X DELETE http://first.kube-prometheus-pushgateway:9091/metrics/job/myapp/instance/10.244.1.155:9091
```

Обратите внимание, что PushGateway хранит полученные метрики в памяти. При рестарте пода PushGateway все метрики будут утеряны.
{% endraw %}
