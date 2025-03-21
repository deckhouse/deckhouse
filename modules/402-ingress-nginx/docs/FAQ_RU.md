---
title: "Модуль ingress-nginx: FAQ"
---

## Как разрешить доступ к приложению внутри кластера только от ingress controller?

Если необходимо ограничить доступ к вашему приложению внутри кластера исключительно от подов ingress, необходимо в под с приложением добавить контейнер с kube-rbac-proxy:

### Пример Deployment для защищенного приложения

{% raw %}

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: my-namespace
spec:
  selector:
    matchLabels:
      app: my-app
  replicas: 1
  template:
    metadata:
      labels:
        app: my-app
    spec:
      serviceAccountName: my-sa
      containers:
      - name: my-cool-app
        image: mycompany/my-app:v0.5.3
        args:
        - "--listen=127.0.0.1:8080"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 443
            scheme: HTTPS
      - name: kube-rbac-proxy
        image: flant/kube-rbac-proxy:v0.1.0 # Рекомендуется использовать прокси из нашего репозитория.
        args:
        - "--secure-listen-address=0.0.0.0:443"
        - "--config-file=/etc/kube-rbac-proxy/config-file.yaml"
        - "--v=2"
        - "--logtostderr=true"
        # Если kube-apiserver недоступен, аутентификация и авторизация пользователей невозможна.
        # Stale Cache хранит только результаты успешной авторизации и используется, только если apiserver недоступен.
        - "--stale-cache-interval=1h30m"
        ports:
        - containerPort: 443
          name: https
        volumeMounts:
        - name: kube-rbac-proxy
          mountPath: /etc/kube-rbac-proxy
      volumes:
      - name: kube-rbac-proxy
        configMap:
          name: kube-rbac-proxy
```

{% endraw %}

Приложение принимает запросы на адресе `127.0.0.1`, это означает, что по незащищенному соединению к нему можно подключиться только внутри пода.
Прокси слушает на адресе `0.0.0.0` и перехватывает весь внешний трафик к поду.

### Как дать минимальные права для Service Account?

Чтобы аутентифицировать и авторизовывать пользователей с помощью kube-apiserver, у прокси должны быть права на создание `TokenReview` и `SubjectAccessReview`.

В кластерах DKP [уже есть готовая ClusterRole](https://github.com/deckhouse/deckhouse/blob/main/modules/002-deckhouse/templates/common/rbac/kube-rbac-proxy.yaml) — **d8-rbac-proxy**.
Создавать ее самостоятельно не нужно! Необходимо прикрепить ее к Service Account'у вашего Deployment'а.
{% raw %}

```yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-sa
  namespace: my-namespace
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: my-namespace:my-sa:d8-rbac-proxy
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:rbac-proxy
subjects:
- kind: ServiceAccount
  name: my-sa
  namespace: my-namespace
```

### Конфигурация Kube-RBAC-Proxy

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-rbac-proxy
data:
  config-file.yaml: |+
    excludePaths:
    - /healthz # Не требуем авторизацию для liveness пробы.
    upstreams:
    - upstream: http://127.0.0.1:8081/ # Куда проксируем.
      path: / # Location прокси, с которого запросы будут проксированы на upstream.
      authorization:
        resourceAttributes:
          namespace: my-namespace
          apiGroup: apps
          apiVersion: v1
          resource: deployments
          subresource: http
          name: my-app
```

{% endraw %}
Согласно конфигурации, у пользователя должны быть права доступа к Deployment с именем `my-app`
и его дополнительному ресурсу `http` в пространство имён `my-namespace`.

Выглядят такие права в виде RBAC следующим образом:
{% raw %}

```yaml
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: kube-rbac-proxy:my-app
  namespace: my-namespace
rules:
- apiGroups: ["apps"]
  resources: ["deployments/http"]
  resourceNames: ["my-app"]
  verbs: ["get", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: kube-rbac-proxy:my-app
  namespace: my-namespace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: kube-rbac-proxy:my-app
subjects:
# Все пользовательские сертификаты ingress-контроллеров выписаны для одной конкретной группы.
- kind: Group
  name: ingress-nginx:auth
```

Для ingress-ресурса необходимо добавить параметры:

```yaml
nginx.ingress.kubernetes.io/backend-protocol: HTTPS
nginx.ingress.kubernetes.io/configuration-snippet: |
  proxy_ssl_certificate /etc/nginx/ssl/client.crt;
  proxy_ssl_certificate_key /etc/nginx/ssl/client.key;
  proxy_ssl_protocols TLSv1.2;
  proxy_ssl_session_reuse on;
```

{% endraw %}
Подробнее о том, как работает аутентификация по сертификатам, можно прочитать в [документации Kubernetes](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#x509-client-certs).

## Как сконфигурировать балансировщик нагрузки для проверки доступности IngressNginxController?

В ситуации, когда `IngressNginxController` размещен за балансировщиком нагрузки, рекомендуется сконфигурировать балансировщик для проверки доступности
узлов `IngressNginxController` с помощью HTTP-запросов или TCP-подключений. В то время как тестирование с помощью TCP-подключений представляет собой простой и универсальный механизм проверки доступности, мы рекомендуем использовать проверку на основе HTTP-запросов со следующими параметрами:

- протокол: `HTTP`;
- путь: `/healthz`;
- порт: `80` (в случае использования инлета `HostPort` нужно указать номер порта, соответствующий параметру [httpPort](cr.html#ingressnginxcontroller-v1-spec-hostport-httpport).

## Как настроить работу через MetalLB с доступом только из внутренней сети?

Пример MetalLB с доступом только из внутренней сети:

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: "nginx"
  inlet: "LoadBalancer"
  loadBalancer:
    sourceRanges:
    - 192.168.0.0/24
```

{% alert level="warning" %}
Для работы необходимо включить параметр [`svcSourceRangeCheck`](../cni-cilium/configuration.html#parameters-svcsourcerangecheck) в модуле cni-cilium.
{% endalert %}

## Как добавить дополнительные поля для логирования в nginx-controller?

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: "nginx"
  inlet: "LoadBalancer"
  additionalLogFields:
    my-cookie: "$cookie_MY_COOKIE"
```

## Как включить HorizontalPodAutoscaling для IngressNginxController?

{% alert level="warning" %}
Режим HPA возможен только для контроллеров с инлетом `LoadBalancer` или `LoadBalancerWithProxyProtocol`.

Режим HPA возможен только при `minReplicas` != `maxReplicas`, в противном случае deployment `hpa-scaler` не создается.
{% endalert %}

HPA выставляется с помощью аттрибутов `minReplicas` и `maxReplicas` в [IngressNginxController CR](cr.html#ingressnginxcontroller).

IngressNginxController разворачивается с помощью DaemonSet. DaemonSet не предоставляет возможности горизонтального масштабирования, поэтому создается дополнительный deployment `hpa-scaler` и HPA resource, который следит за предварительно созданной метрикой `prometheus-metrics-adapter-d8-ingress-nginx-cpu-utilization-for-hpa`. Если CPU utilization превысит 50%, HPA закажет новую реплику для `hpa-scaler` (с учетом minReplicas и maxReplicas).

Deployment `hpa-scaler` обладает HardPodAntiAffinity, поэтому он попытается выделить для себя новый узел (если это возможно
в рамках своей группы узлов), куда автоматически будет размещен еще один instance Ingress-контроллера.

{% alert level="info" %}

- Минимальное реальное количество реплик IngressNginxController не может быть меньше минимального количества узлов в группе узлов, в которую разворачивается IngressNginxController.
- Максимальное реальное количество реплик IngressNginxController не может быть больше максимального количества узлов в группе узлов, в которую разворачивается IngressNginxController.

{% endalert %}

## Как использовать IngressClass с установленными IngressClassParameters?

Начиная с версии 1.1 IngressNginxController, Deckhouse создает объект IngressClass самостоятельно. Если вы хотите использовать свой IngressClass с установленными IngressClassParameters, достаточно добавить к нему label `ingress-class.deckhouse.io/external: "true"`:

```yaml
apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  labels:
    ingress-class.deckhouse.io/external: "true"
  name: my-super-ingress
spec:
  controller: ingress-nginx.deckhouse.io/my-super-ingress
  parameters:
    apiGroup: elbv2.k8s.aws
    kind: IngressClassParams
    name: awesome-class-cfg
```

В этом случае, при указании данного IngressClass в CRD IngressNginxController, Deckhouse не будет создавать объект, а использует существующий.

## Как отключить сборку детализированной статистики Ingress-ресурсов?

По умолчанию Deckhouse собирает подробную статистику со всех Ingress-ресурсов в кластере. Этот процесс может приводить к высокой нагрузке системы мониторинга.

Для отключения сбора статистики добавьте лейбл `ingress.deckhouse.io/discard-metrics: "true"` к соответствующему пространству имён или Ingress-ресурсу.

Пример отключения сбора статистики (метрик) для всех Ingress-ресурсов в пространстве имен `review-1`:

```shell
kubectl label ns review-1 ingress.deckhouse.io/discard-metrics=true
```

Пример отключения сбора статистики (метрик) для всех Ingress-ресурсов `test-site` в пространстве имен `development`:

```shell
kubectl label ingress test-site -n development ingress.deckhouse.io/discard-metrics=true
```

## Как корректно вывести из эксплуатации (drain) узел с запущенным IngressNginxController?

Доступно два способа вывода такого узла из эксплуатации - или с помощью аннотации узла (аннотация будет удалена после завершения операции):

```shell
kubectl annotate node <node_name> update.node.deckhouse.io/draining=user
```

или с помощью базового функционала kubectl drain (тут стоит отметить, что необходимо указать флаг --force, несмотря на то, что указан флаг --ignore-daemonsets, так как IngressNginxController
разворачивается с помощью Advanced DaemonSet):

```shell
kubectl drain <node_name> --delete-emptydir-data --ignore-daemonsets --force
```
