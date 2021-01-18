---
title: "Разработка target'ов Prometheus"
type:
  - instruction
search: Разработка target'ов Prometheus, prometheus target
---

Общая информация
----------------

* Наиболее частая операция — добавление target'а для нового application'а (redis, rabbitmq и др.). Скорей всего для этого будет достаточно просто скопировать один из существующих service monitor'ов в директории `applications` и поправить названия.
* Но если вам нужно сделать что-то более сложное, или если простое копирование не дает ожидаемого результата — придется разбираться и читать документацию модуля [Prometheus Operator](/modules/200-operator-prometheus/).
* Все существующие target'ы лежат в директории `prometheus-targets`, они обычно состоят из service monitor'а, некоторого exporter'а для Prometheus и необходимой обвязки, которая их стыкует.

Лучшие практики
---------------

### Лейблы pod-ориентированных метрик

Абсолютное большинство метрик, хранимых в Prometheus, или содержит информацию о параметрах самого pod'а, или информацию о параметрах приложения, запущенного в pod'е. Мы называем все такие метрики pod-ориентированными, и относим к ним (преимущественно, но не исключительно):
* системные метрики, отражающие параметры производительности самого пода (экспортируются kubelet'ом)
* прикладные метрики:
    * метрики поддерживаемых application'ов (redis, rabbitmq и др.)
    * custom-метрики

У всех pod-ориентированных метрик обязательно есть лейбл с именем pod'а (обычно он называется `instance`, но у метрик получаемых из kubelet'а — `pod_name`, а у kube-state-metrics — `pod`), но работать с именами pod'ов не удобно, а удобно нам работать с `service` и `namespace`, поэтому:
* у всех без исключения pod-ориентированных метрик есть лейбл `namespace`,
* у прикладных (applications и custom) pod-ориентированных метрик есть лейбл `service`, определяющий группу pod'ов под одним понятным названием.

### Авторизация для доступа к экспортируемым метрикам

Настоятельно рекомендуется настраивать экспортеры метрик так, чтобы получить данные мог только проверенный и авторизованный пользователь.

Для предоставления безопасного доступа к метрикам в Kubernetes существует [kube-rbac-proxy](https://github.com/brancz/kube-rbac-proxy) — написанный на go прокси, который достает из запросов аутентифицирует пользователя при помощи `TokenReview` или клиентского сертификата. 
Авторизация осуществляется при помощи `SubjectAccessReview` согласно описанным для пользователя RBAC-правилам.

#### Пример Deployment для защищенного экспортера: 
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-exporter
  namespace: my-namespace
spec:
  selector:
    matchLabels:
      app: my-exporter
  replicas: 1
  template:
    metadata:
      labels:
        app: my-exporter
    spec:
      serviceAccountName: my-sa
      containers:
      - name: my-cool-app
        image: mycompany/my-cool-exporter:v0.5.3
        args:
        - "--listen=127.0.0.1:8081"
      - name: kube-rbac-proxy
        image: flant/kube-rbac-proxy:v0.1.0 # рекомендуется использовать прокси из нашего репозитория
        args:
        - "--secure-listen-address=0.0.0.0:8080"
        - "--config-file=/etc/kube-rbac-proxy/config-file.yaml"
        # Сертификат для проверки пользователя, указывает стандартный клиентский CA Kubernetes
        # (есть в каждом поде)
        - "--client-ca-file=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
        - "--v=2"
        - "--logtostderr=true"
        # Если kube-apiserver не доступен, мы не сможем аутентифицировать и авторизовывать пользователей.
        # Stale Cache хранит только результаты успешной авторизации и используется только если apiserver не доступен. 
        - "--stale-cache-interval=1h30m"
        ports:
        - containerPort: 8080
          name: https-metrics
        volumeMounts:
        - name: kube-rbac-proxy
          mountPath: /etc/kube-rbac-proxy
      volumes:
      - name: kube-rbac-proxy
        configMap:
          name: kube-rbac-proxy
```
Экспортер метрик принимает запросы на адресе 127.0.0.1, что означает, что по незащищенному соединению к нему можно подключиться только изнутри пода.
Прокси же слушает на адресе 0.0.0.0 и перехватывает весь внешний трафик к поду.

### Минимальные права для Service Account

Чтобы аутентифицировать и авторизовывать пользователей при помощи kube-apiserver, у прокси должны быть права на создание `TokenReview` и `SubjectAccessReview`.

В наших кластерах [уже есть готовая ClusterRole](https://github.com/deckhouse/deckhouse/tree/master/modules/020-deckhouse/templates/common/rbac/kube-rbac-proxy.yaml) - **d8-rbac-proxy**.
Создавать её самостоятельно не нужно! Нужно только прикрепить её к serviceaccount'у вашего Deployment'а.
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
    upstreams:
    - upstream: http://127.0.0.1:8081/metrics # куда проксируем
      path: /metrics # location прокси, с которого запросы будут проксированы на upstream
      authorization:
        resourceAttributes:
          namespace: my-namespace
          apiGroup: apps
          apiVersion: v1
          resource: deployments
          subresource: prometheus-metrics
          name: my-exporter
```
Согласно конфигурации, у пользователя должны быть права на доступ к Deployment с именем `my-exporter` 
и его дополнительному ресурсу `prometheus-metrics` в неймспейсе `my-namespace`.

Выглядят такие права в виде RBAC так: 
```yaml
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: kube-rbac-proxy:my-exporter
  namespace: my-namespace
rules:
- apiGroups: ["apps"]
  resources: ["deployments/prometheus-metrics"]
  resourceNames: ["my-exporter"]
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: kube-rbac-proxy:my-exporter
  namespace: my-namespace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: kube-rbac-proxy:my-exporter
subjects:
- kind: User
  name: my-user
```

Теперь my-user сможет получать метрики из вашего пода.
