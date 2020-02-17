Модуль ingress-nginx
====================
Модуль позволяет установить в кластер один или несколько [ingress-nginx controller](https://github.com/kubernetes/ingress-nginx/)'ов при помощи CRD. 

Конфигурация
------------

### Включение модуля

Модуль по-умолчанию **включен** в кластерах начиная с версии 1.15 (данный модуль работает только в кластерах начиная с версии 1.15). Для выключения добавьте в CM `d8-system/deckhouse`:
```yaml
ingressNginxEnabled: "false"
```

### Параметры модуля

* `defaultControllerVersion` — версия контроллера ingress-nginx, которая будет использоваться для всех контроллеров по-умолчанию, если не был задан параметр `controllerVersion` в IngressNginxController CRD. 
    * По-умолчанию `0.25`,
    * Доступные варианты: `0.25`, `0.26`.

Параметры ресурса IngressNginxController
----------------------------------------
Параметры указываются в поле `spec`.

**Обязательные параметры:**
* `ingressClass` — имя ingress-класса для обслуживания ingress nginx controller. При помощи данной опции можно создать несколько контроллеров для обслуживания одного ingress-класса. 
    * **Важно!** Если указать значение "nginx", то дополнительно будут обрабатываться ingress ресурсы без аннотации `kubernetes.io/ingress.class`.
* `inlet` — способ поступления трафика из внешнего мира.
    * `LoadBalancer` — устанавливается ingress controller и заказывается сервис с типом LoadBalancer. 

**Необязательные параметры:**
* `controllerVersion` — версия ingress-nginx контроллера;
    * По-умолчанию берется версия из настроек модуля.
    * Доступные варианты: `"0.25"`, `"0.26"`.
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/README.md#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/README.md#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.
* `loadBalancer` — секция настроек для inlet'а `LoadBalancer`:
    * `annotations` — аннотации, которые будут проставлены сервису для гибкой настройки балансировщика.
    * `sourceRanges` — список CIDR, которым разрешен доступ на балансировщик.
        * Облачный провайдер может не поддерживать данную опцию и игнорировать её. 
    * `behindL7Proxy` — включает обработку и передачу X-Forwarded-* заголовков.
        * **Внимание!** При использовании этой опции вы должны быть уверены, что запросы к ingress'у направляются только от доверенных источников.
    * `realIPHeader` — заголовок, из которого будет получен настоящий IP-адрес клиента.
        * По-умолчанию `X-Forwarded-For`.
        * Опция работает только при включении `behindL7Proxy`.
* `hsts` — bool, включен ли hsts.
    * По-умолчанию — выключен (`false`).
* `legacySSL` — bool, включены ли старые версии TLS. Также опция разрешает legacy cipher suites для поддержки старых библиотек и программ: [OWASP Cipher String 'C' ](https://www.owasp.org/index.php/TLS_Cipher_String_Cheat_Sheet). Подробнее [здесь](templates/ingress/configmap.yaml).
    * По-умолчанию включён только TLSv1.2 и самые новые cipher suites.
* `disableHTTP2` — bool, выключить ли HTTP/2.
    * По-умолчанию HTTP/2 включен (`false`).
* `underscoresInHeaders` — bool, разрешены ли нижние подчеркивания в хедерах. Подробнее [здесь](http://nginx.org/en/docs/http/ngx_http_core_module.html#underscores_in_headers). Почему не стоит бездумно включать написано [здесь](https://www.nginx.com/resources/wiki/start/topics/tutorials/config_pitfalls/#missing-disappearing-http-headers).
    * По-умолчанию `false`.
* `customErrors` — секция с настройкой кастомизации HTTP ошибок (если секция определена, то все параметры в ней являются обязательными, изменение любого параметра **приводит к перезапуску всех ingress-nginx контроллеров**);
    * `serviceName` — имя сервиса, который будет использоваться, как custom default backend.
    * `namespace` — имя namespace, в котором будет находится сервис, используемый, как custom default backend.
    * `codes` — список кодов ответа (массив), при которых запрос будет перенаправляться на custom default backend.
* `config` — секция настроек ingress controller, в которую в формате `ключ: значение(строка)` можно записать [любые возможные опции](https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/configmap/);
    * **Внимание!** Ошибка в указании опций может привести к отказу в работе ingress controller'а.
    * **Внимание!** Не рекомендуется использовать данную опцию, не гарантируется обратная совместимость или работоспособность ingress controller'а с использованием данной опции.
* `additionalHeaders` — дополнительные header'ы, которые будут добавлены к каждому запросу. Указываются в формате `ключ: значение(строка)`.


### Пример
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  controllerVersion: "0.26"
  hsts: true
  config:
    gzip-level: "4"
    worker-processes: "8"
  additionalHeaders:
    X-Different-Name: "true"  
    Host: "$proxy_host"
```

Примеры использования ресурса IngressNginxController
----------------------------------------------------


#### AWS (Network Load Balancer)
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: "nginx"
  inlet: "LoadBalancer"
  loadBalancer:
    annotations:
      service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
```

#### GCP
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: "nginx"
  inlet: "LoadBalancer"
```

#### MetalLB с доступом только из внутренней сети
```yaml
apiVersion: deckhouse.io/v1alpha1
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

Статистика
----------

### Основные принципы сбора статистики

1. На каждый запрос, на стадии `log_by_lua`, вызывается наш модуль, который рассчитывает необходимые данные и шлет их через [datagram socket](https://en.wikipedia.org/wiki/Network_socket#Datagram_socket) в `statsd`.
2. Вместо обычного `statsd` у нас в pod'е с ingress-controller'ом запущен sidecar-контейнер с [statsd_exporter'ом](https://github.com/prometheus/statsd_exporter), который принимает данные в формате `statsd`, разбирает, агрегирует их по установленным нами правилам и экспортирует в формате для Prometheus.
3. Prometheus каждые 30 секунд scrape'ает как сам ingress-controller (там есть небольшое количество нужных нам метрик), так и statsd_exporter, и на основании этих данных все и работает!

### Какую статистику мы собираем и как она представлена?

У всех собираемых метрик есть служебные лейблы, позволяющие идентифицировать экземпляр контроллера: `controller`, `app`, `instance` и `endpoint` (они видны в `/prometheus/targets`).

* Все метрики (кроме geo), экспортируемые statsd_exporter'ом, представлены в трех уровнях детализации:
    * `ingress_nginx_overall_*` — "вид с вертолета", у всех метрик есть лейблы `namespace`, `vhost` и `content_kind`.
    * `ingress_nginx_detail_*` — кроме лейблов уровня overall добавляются: `ingress`, `service`, `service_port` и `location`.
    * `ingress_nginx_detail_backend_*` — ограниченная часть данных, собирается в разрезе по бекендам. У этих метрик, кроме лейблов уровня detail, добавляется лейбл `pod_ip`.

* Для уровней overall и detail собираются следующие метрики:
    * `*_requests_total` — counter количества запросов (дополнительные лейблы: `scheme`, `method`).
    * `*_responses_total` — counter количества ответов (дополнительные лейблы: `status`).
    * `*_request_seconds_{sum,count,bucket}` — histogram времени ответа.
    * `*_bytes_received_{sum,count,bucket}` — histogram размера запроса.
    * `*_bytes_sent_{sum,count,bucket}` — histogram размера ответа.
    * `*_upstream_response_seconds_{sum,count,bucket}` — histogram времени ответа upstream'а (используется сумма времен ответов всех upstream'ов, если их было несколько).
    * `*_lowres_upstream_response_seconds_{sum,count,bucket}` — то же самое, что предыдущая метрика, только с меньшей детализацией (подходит для визуализации, но не подходит для расчета quantile).
    * `*_upstream_retries_{count,sum}` — количество запросов, при обработке которых были retry бекендов, и сумма retry'ев.

* Для уровня overall собираются следующие метрики:
    * `*_geohash_total` — counter количества запросов с определенным geohash (дополнительные лейблы: `geohash`, `place`).

* Для уровня detail_backend собираются следующие метрики:
    * `*_lowres_upstream_response_seconds` — то же самое, что аналогичная метрика для overall и detail.
    * `*_responses_total` — counter количества ответов (дополнительный лейбл `status_class`, а не просто `status`).
    *  `*_upstream_bytes_received_sum` — counter суммы размеров ответов backend'а.

Как разрешить доступ к приложению внутри кластера только от ingress'ов
----------------------------------------------------------------------

В случае, если вы хотите ограничить доступ к вашему приложению внутри кластера ТОЛЬКО от подов ingress'а, 
вам необходимо в под с приложением добавить контейнер с kube-rbac-proxy:

### Пример Deployment для защищенного приложения: 
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
        image: flant/kube-rbac-proxy:v0.1.0 # рекомендуется использовать прокси из нашего репозитория
        args:
        - "--secure-listen-address=0.0.0.0:443"
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
Приложение принимает запросы на адресе 127.0.0.1, что означает, что по незащищенному соединению к нему можно подключиться только изнутри пода.
Прокси же слушает на адресе 0.0.0.0 и перехватывает весь внешний трафик к поду.

### Минимальные права для Service Account

Чтобы аутентифицировать и авторизовывать пользователей при помощи kube-apiserver, у прокси должны быть права на создание `TokenReview` и `SubjectAccessReview`.

В наших кластерах [уже есть готовая ClusterRole](../../020-deckhouse/templates/kube-rbac-proxy.yaml) - **d8-rbac-proxy**.
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
  name: d8-rbac-proxy
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
    - /healthz # не требуем авторизацию для liveness пробы
    upstreams:
    - upstream: http://127.0.0.1:8081/ # куда проксируем
      path: / # location прокси, с которого запросы будут проксированы на upstream
      authorization:
        resourceAttributes:
          namespace: my-namespace
          apiGroup: apps
          apiVersion: v1
          resource: deployments
          subresource: http
          name: my-app
```
Согласно конфигурации, у пользователя должны быть права на доступ к Deployment с именем `my-app` 
и его дополнительному ресурсу `http` в неймспейсе `my-namespace`.

Выглядят такие права в виде RBAC так: 
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
# Все пользовательские сертификаты ingress-controller'ов выписаны для одной конкретной группы
- kind: Group
  name: ingress-nginx:auth
```

Для ingress'а ресурса необходимо добавить параметры:
```yaml
nginx.ingress.kubernetes.io/backend-protocol: HTTPS
nginx.ingress.kubernetes.io/configuration-snippet: |
  proxy_ssl_certificate /etc/nginx/ssl/client.crt;
  proxy_ssl_certificate_key /etc/nginx/ssl/client.key;
  proxy_ssl_protocols TLSv1.2;
  proxy_ssl_session_reuse on;
```
Подробнее о том, как работает аутентификация по сертификатам можно прочитать [по этой ссылке](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#x509-client-certs).
