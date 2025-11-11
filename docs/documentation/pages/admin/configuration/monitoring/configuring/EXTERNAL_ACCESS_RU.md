---
title: "Настройка внешнего доступа"
permalink: ru/admin/configuration/monitoring/configuring/external-access.html
lang: ru
---

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
В качестве URL необходимо указать `https://prometheus-api.<домен-вашего-кластера>`**.
{% endalert %}

<img src="../../../../images/prometheus/prometheus_connect_settings.png" height="500">

* **Basic-авторизация** не является надежной мерой безопасности. Рекомендуется ввести дополнительные меры безопасности, например указать аннотацию `nginx.ingress.kubernetes.io/whitelist-source-range`.

* Из-за необходимости создания Ingress-ресурса в системном пространстве имён подключение таким способом **не рекомендуется**.
  DKP **не гарантирует** сохранение работоспособности данной схемы подключения в связи с его активными постоянными обновлениями.

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
- Включите и настройте [модуль `prometheus-pushgateway`](/modules/prometheus-pushgateway/).

  Включить модуль можно в веб-интерфейсе (Deckhouse Console), или с помощью следующей команды:

  ```shell
  d8 system module enable prometheus-pushgateway
  ```

- Укажите названия шлюзов в параметре `instances` модуля `prometheus-pushgateway` через веб-интерфейс, или с помощью следующей команды:

  ```shell
  d8 k edit mc prometheus-pushgateway
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
