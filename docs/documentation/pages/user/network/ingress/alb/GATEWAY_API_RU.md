---
title: "Публикация приложений средствами Kubernetes Gateway API"
description: "Публикация приложений с Kubernetes Gateway API в Deckhouse Kubernetes Platform. ListenerSet, HTTPRoute, GRPCRoute, TLSRoute, TCPRoute, BackendTLSPolicy, аннотации HTTPRoute и WAF."
permalink: ru/user/network/ingress/alb/gateway-api.html
lang: ru
extractedLinksMax: 4
relatedLinks:
  - title: "Использование Application Load Balancer (ALB)"
    url: /products/kubernetes-platform/documentation/v1/user/network/ingress/alb.html
  - title: "ALB средствами Kubernetes Gateway API"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html
  - title: "Миграция с ingress-nginx на alb"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/migration.html
  - title: "Документация модуля alb"
    url: /modules/alb/
  - title: "Custom Resources модуля alb"
    url: /modules/alb/cr.html
  - title: "Параметры модуля alb"
    url: /modules/alb/configuration.html
  - title: "FAQ модуля alb"
    url: /modules/alb/faq.html
  - title: "Примеры модуля alb"
    url: /modules/alb/examples.html
---

## Публикация приложений средствами Kubernetes Gateway API

Публикация приложения возможна через общекластерный шлюз (ClusterALBInstance, создаёт администратор кластера) или через отдельный шлюз в неймспейсе приложения (ALBInstance).

Создание управляемого Gateway (ClusterALBInstance или ALBInstance, инлеты, включение модуля) — задача администратора. Настройка инфраструктуры описана в разделе [«Включение модуля и создание Gateway»](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#создание-управляемого-объекта-gateway).

Этот сценарий предполагает, что объект ClusterALBInstance или ALBInstance уже создан и перешёл в состояние `Ready`. Запросите у администратора имя и неймспейс управляемого Gateway или получите имя Gateway из поля [`status.gateway`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-status) инстанса:

```shell
d8 k get clusteralbinstance <CLUSTER_ALB_INSTANCE_NAME> \
  -o jsonpath='{.status.gateway}{"\n"}'
d8 k -n <NAMESPACE> get albinstance <ALB_INSTANCE_NAME> \
  -o jsonpath='{.status.gateway}{"\n"}'
```

Для ClusterALBInstance управляемый Gateway обычно находится в неймспейсе `d8-alb`. Для ALBInstance — в том же неймспейсе, что и объект ALBInstance. Описание полей статуса — в [`status`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-status).

Администратор неймспейса создаёт ListenerSet, привязанный к этому Gateway ([`spec.parentRef`](https://gateway-api.sigs.k8s.io/guides/user-guides/listener-set/)). Разработчики приложения создают объекты HTTPRoute, привязанные к ListenerSet.

Слушатели `d8-http` и `d8-https` предназначены для служебных задач — например, для проверки доступности шлюза или запросов HTTP-01 от `cert-manager`. Не привязывайте к ним маршруты приложений — для публикации приложений используйте ListenerSet.

### Публикация приложения с ListenerSet и HTTPRoute {#publishing-with-listenerset-and-httproute}

Пример ListenerSet и HTTPRoute для публикации приложения через управляемый Gateway:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: app-listeners
  namespace: prod
spec:
  parentRef:
    name: public-gw   # Имя объекта Gateway из status ClusterALBInstance, предоставленное администратором.
    namespace: d8-alb
  listeners:
    - name: app-http
      port: 80 # В ListenerSet для HTTP указывайте порт 80. Это не параметр hostPort.httpPort инлета HostPort на узле.
      protocol: HTTP
      hostname: app.example.com
    - name: app-https
      port: 443 # В ListenerSet для HTTPS указывайте порт 443. Это не параметр hostPort.httpsPort инлета HostPort на узле.
      protocol: HTTPS
      hostname: app.example.com
      tls:
        mode: Terminate
        certificateRefs:
          - name: app-tls   # Secret с TLS-сертификатом (выпустите через cert-manager или создайте вручную).
            namespace: prod
---
# Маршрут для HTTP-трафика
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: app-http-route
  namespace: prod
spec:
  parentRefs:
    - name: app-listeners # Имя ListenerSet.
      namespace: prod
      kind: ListenerSet
      group: gateway.networking.k8s.io
      sectionName: app-http
      port: 80
  hostnames:
    - app.example.com
  rules:
    - backendRefs:
        - name: app-svc # Наименование сервиса приложения.
          port: 8080
---
# Маршрут для HTTPS-трафика
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: app-https-route
  namespace: prod
spec:
  parentRefs:
    - name: app-listeners # Имя ListenerSet.
      namespace: prod
      kind: ListenerSet
      group: gateway.networking.k8s.io
      sectionName: app-https
      port: 443
  hostnames:
    - app.example.com
  rules:
    - backendRefs:
        - name: app-svc # Наименование сервиса приложения
          port: 8080
```

### Проверка публикации {#checking-publication}

После применения ListenerSet и HTTPRoute проверьте статус объектов:

```shell
d8 k -n <NAMESPACE> get listenerset
d8 k -n <NAMESPACE> describe listenerset <LISTENERSET_NAME>
d8 k -n <NAMESPACE> get httproute
d8 k -n <NAMESPACE> describe httproute <HTTPROUTE_NAME>
```

Проверьте следующее:

- В статусе ListenerSet слушатели в состоянии `Programmed` или `Accepted`.
- У HTTPRoute в `parentRefs` указан нужный ListenerSet и условия `Accepted` выполнены.
- Адрес точки входа и DNS для hostname согласованы с администратором (для инлета `LoadBalancer` адрес обычно берётся из Service в неймспейсе `d8-alb`).

Проверьте доступность приложения с клиента, подставив адрес точки входа и hostname (успешный ответ — код `200` или другой ожидаемый код приложения):

```shell
curl -vk \
  --resolve app.example.com:443:<ENTRY_POINT_ADDRESS> \
  https://app.example.com/
```

Если маршрут не принимается, проверьте имя Gateway из `status` инстанса, неймспейс ListenerSet, порт и `sectionName` в `parentRefs`. Конфликты hostname и порта с другими объектами ListenerSet видны в выводе команды `describe listenerset` из предыдущего шага — в статусе конфликтующего слушателя появляется условие с описанием причины.

### Работа с объектами GRPCRoute, TLSRoute, TCPRoute и UDPRoute {#grpcroute-tlsroute-tcproute-and-udproute-objects}

Помимо HTTPRoute, для публикации приложений можно использовать GRPCRoute (gRPC-трафик), TLSRoute (сквозная передача TLS) и TCPRoute/UDPRoute (произвольный TCP и UDP-трафик).

#### GRPCRoute для gRPC-трафика

Объект GRPCRoute предназначен для маршрутизации gRPC-трафика. Для него создаётся объект ListenerSet со слушателем HTTPS, а затем добавляется объект GRPCRoute:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: grpc-listeners
  namespace: prod
spec:
  parentRef:
    name: app-gw   # Имя объекта Gateway из поля status ALBInstance.
    namespace: prod
  listeners:
    - name: grpc-https
      port: 443 # В ListenerSet для HTTPS указывайте порт 443. Это не параметр hostPort.httpsPort инлета HostPort на узле.
      protocol: HTTPS
      hostname: grpc.example.com
      tls:
        mode: Terminate
        certificateRefs:
          - name: grpc-tls   # Наименование секрета содержащего необходимый TLS-сертификат.
            namespace: prod
---
apiVersion: gateway.networking.k8s.io/v1
kind: GRPCRoute
metadata:
  name: grpc-route
  namespace: prod
spec:
  parentRefs:
    - name: grpc-listeners # Имя ListenerSet.
      namespace: prod
      kind: ListenerSet
      group: gateway.networking.k8s.io
      sectionName: grpc-https
      port: 443
  hostnames:
    - grpc.example.com
  rules:
    - backendRefs:
        - name: grpc-svc # Наименование сервиса приложения.
          port: 9090
```

#### TLS passthrough с TLSRoute

Для TLS passthrough, когда расшифровка трафика должна выполняться на стороне приложения, можно использовать либо слушателя TLS, либо слушателя HTTPS.

Дополнительные порты задаются в [`spec.inlet.additionalPorts`](/modules/alb/cr.html#albinstance-v1alpha1-spec-inlet-additionalports) объекта, который владеет шлюзом:

- для общекластерного Gateway — в ClusterALBInstance (это делает администратор кластера);
- для шлюза в неймспейсе — в ALBInstance (администратор неймспейса или вы, если есть права на объект).

Пример для ClusterALBInstance — в разделе [«Открытие дополнительного TCP/UDP-порта на общекластерном Gateway»](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#tcp-port). Ниже показан пример для ALBInstance.

{% tabs TLS passthrough %}
{% tab "Слушатель TLS" %}

Поскольку в этом примере слушатель TLS использует дополнительный порт, попросите администратора добавить `additionalPorts` в объект, владеющий шлюзом (ClusterALBInstance или ALBInstance), или добавьте параметр сами, если у вас есть права:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ALBInstance
metadata:
  name: app-gw
  namespace: prod
spec:
  gatewayName: app-gw
  inlet:
    type: LoadBalancer
    additionalPorts:
      - port: 8443    # Дополнительный TCP-порт для TLS-трафика.
        protocol: TCP
```

Далее настройте объекты ListenerSet и TLSRoute:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: tls-pass-listeners
  namespace: prod
spec:
  parentRef:
    name: app-gw   # Имя объекта Gateway из поля status ALBInstance.
    namespace: prod
  listeners:
    - name: tls-pass
      port: 8443 # В данном примере для TLS трафика переиспользуется порт 8443.
      protocol: TLS
      hostname: pass.example.com
      tls:
        mode: Passthrough # Режим TLS — сквозной.
---
apiVersion: gateway.networking.k8s.io/v1
kind: TLSRoute
metadata:
  name: tls-pass-route
  namespace: prod
spec:
  parentRefs:
    - name: tls-pass-listeners # Имя ListenerSet.
      namespace: prod
      kind: ListenerSet
      group: gateway.networking.k8s.io
      sectionName: tls-pass
      port: 8443 # В данном примере для TLS трафика переиспользуется порт 8443.
  hostnames:
    - pass.example.com
  rules:
    - backendRefs:
        - name: tls-pass-svc # Наименование сервиса приложения.
          port: 8443
```

{% endtab %}
{% tab "Слушатель HTTPS" %}

Вариант со слушателем HTTPS удобен, когда нужно использовать стандартный обработчик на порту `443`: дополнительный порт для TLS passthrough открывать не требуется.

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: https-pass-listeners
  namespace: prod
spec:
  parentRef:
    name: app-gw   # Имя объекта Gateway из поля status ALBInstance.
    namespace: prod
  listeners:
    - name: https-pass
      port: 443 # В данном примере для TLS трафика переиспользуется порт 443.
      protocol: HTTPS
      hostname: pass.example.com
      tls:
        mode: Passthrough # Режим TLS — сквозной.
---
apiVersion: gateway.networking.k8s.io/v1
kind: TLSRoute
metadata:
  name: https-pass-route
  namespace: prod
spec:
  parentRefs:
    - name: https-pass-listeners # Имя ListenerSet.
      namespace: prod
      kind: ListenerSet
      group: gateway.networking.k8s.io
      sectionName: https-pass
      port: 443 # В данном примере для TLS трафика переиспользуется порт 443.
  hostnames:
    - pass.example.com
  rules:
    - backendRefs:
        - name: tls-pass-svc # Наименование сервиса приложения.
          port: 8443
```

{% endtab %}
{% endtabs %}

#### TLS-терминация на шлюзе с TCPRoute

Если TLS нужно терминировать на шлюзе, а затем передать трафик дальше как TCP-поток к бэкенду (например, когда приложение принимает уже расшифрованный TCP, а не HTTPS), создайте объект ListenerSet со слушателем TLS и режимом `Terminate`, после чего подключите объект TCPRoute:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: tls-term-listeners
  namespace: prod
spec:
  parentRef:
    name: app-gw   # Имя объекта Gateway из поля status ALBInstance.
    namespace: prod
  listeners:
    - name: tls-term
      port: 443 # В данном примере для TLS трафика переиспользуется порт 443.
      protocol: TLS
      hostname: term.example.com
      tls:
        mode: Terminate     # Режим TLS — терминация.
        certificateRefs:
          - name: term-tls  # Наименование секрета содержащего необходимый TLS-сертификат.
            namespace: prod
---
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: TCPRoute
metadata:
  name: tls-term-route
  namespace: prod
spec:
  parentRefs:
    - name: tls-term-listeners # Имя ListenerSet.
      namespace: prod
      kind: ListenerSet
      group: gateway.networking.k8s.io
      sectionName: tls-term
      port: 443
  rules:
    - backendRefs:
        - name: tls-svc # Наименование сервиса приложения.
          port: 8080
```

#### Дополнительные TCP- и UDP-порты

Для портов TCP и UDP из [`additionalPorts`](/modules/alb/cr.html#albinstance-v1alpha1-spec-inlet-additionalports) маршрут привязывается напрямую к слушателю управляемого Gateway, без отдельного ListenerSet. Иначе контроллер отклонит конфигурацию из-за пересечения обработчиков.

Дополнительные порты задаются в объекте, который владеет шлюзом: в ClusterALBInstance для общекластерного Gateway или в ALBInstance для шлюза в неймспейсе. Для ClusterALBInstance используйте пример в разделе [«Открытие дополнительного TCP/UDP-порта на общекластерном Gateway»](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#tcp-port).

{% tabs TCP и UDP %}
{% tab "TCP" %}

Для публикации TCP-сервиса попросите администратора открыть дополнительный TCP-порт в объекте, владеющем шлюзом (ClusterALBInstance или ALBInstance), или откройте его сами, если у вас есть права:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ALBInstance
metadata:
  name: app-gw
  namespace: prod
spec:
  gatewayName: app-gw
  inlet:
    type: LoadBalancer
    additionalPorts:
      - port: 9000
        protocol: TCP
```

Контроллер модуля `alb` автоматически создаёт на управляемом объекте Gateway TCP-слушатель на основе `spec.inlet.additionalPorts`. Привяжите объект TCPRoute к этому слушателю:

```yaml
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: TCPRoute
metadata:
  name: tcp-route
  namespace: prod
spec:
  parentRefs:
    - name: app-gw # Имя объекта Gateway из поля status ALBInstance.
      namespace: prod
      kind: Gateway
      group: gateway.networking.k8s.io
      sectionName: tcp-port-9000
      port: 9000
  rules:
    - backendRefs:
        - name: tcp-svc # Наименование сервиса приложения.
          port: 9000
```

{% endtab %}
{% tab "UDP" %}

Для публикации UDP-сервиса попросите администратора открыть дополнительный UDP-порт в объекте, владеющем шлюзом (ClusterALBInstance или ALBInstance), или откройте его сами, если у вас есть права:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ALBInstance
metadata:
  name: app-gw
  namespace: prod
spec:
  gatewayName: app-gw
  inlet:
    type: LoadBalancer
    additionalPorts:
      - port: 5353
        protocol: UDP
```

Контроллер модуля `alb` автоматически создаёт на управляемом объекте Gateway UDP-слушатель на основе `spec.inlet.additionalPorts`. Привяжите объект UDPRoute к этому слушателю:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: UDPRoute
metadata:
  name: udp-route
  namespace: prod
spec:
  parentRefs:
    - name: app-gw # Имя объекта Gateway из поля status ALBInstance.
      namespace: prod
      kind: Gateway
      group: gateway.networking.k8s.io
      sectionName: udp-port-5353
      port: 5353
  rules:
    - backendRefs:
        - name: udp-svc # Наименование сервиса приложения.
          port: 5353
```

{% endtab %}
{% endtabs %}

### Перевод приложения на публикацию через другой Gateway

Если приложение нужно опубликовать через другой объект Gateway, выполните следующие шаги:

1. Получите у администратора кластера новый управляемый Gateway (новый ClusterALBInstance или ALBInstance), чтобы контроллер создал новый объект Gateway. Создание управляемых Gateway описано в разделе [«Включение модуля и создание Gateway»](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#создание-управляемого-объекта-gateway).
1. Создайте объект ListenerSet с теми же именами хостов, портами и TLS-настройками. В [`spec.parentRef`](https://gateway-api.sigs.k8s.io/guides/user-guides/listener-set/) укажите новый объект Gateway.
1. В существующий объект HTTPRoute, в `parentRefs` добавьте ещё один объект, который указывает на новый объект ListenerSet.
1. Проверьте доступность приложения через новый шлюз.
1. После проверки удалите из `parentRefs` объекта HTTPRoute ссылку на неактуальные ListenerSet.

### Привязка маршрута в одном неймспейсе к объекту ListenerSet в другом неймспейсе

Gateway API по умолчанию запрещает маршрутам ссылаться на объекты в чужих неймспейсах — это нужно разрешить явно. Если объект HTTPRoute должен подключаться к ListenerSet из другого неймспейса, в неймспейсе целевого ListenerSet добавьте ReferenceGrant.

В примере ниже — общий ListenerSet в `shared-gw`, прикладной HTTPRoute в `prod` и ReferenceGrant в `shared-gw`, разрешающий такую привязку:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: shared-listeners
  namespace: shared-gw
spec:
  parentRef:
    name: public-gw
    namespace: d8-alb
  listeners:
    - name: app-https
      port: 443
      protocol: HTTPS
      hostname: app.example.com
      tls:
        mode: Terminate
        certificateRefs:
          - name: app-tls
            namespace: shared-gw
---
apiVersion: gateway.networking.k8s.io/v1
kind: ReferenceGrant
metadata:
  name: allow-prod-httproute-to-shared-listeners
  namespace: shared-gw
spec:
  from:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      namespace: prod
  to:
    - group: gateway.networking.k8s.io
      kind: ListenerSet
      name: shared-listeners
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: app-route
  namespace: prod
spec:
  parentRefs:
    - name: shared-listeners
      namespace: shared-gw
      kind: ListenerSet
      group: gateway.networking.k8s.io
      sectionName: app-https
      port: 443
  hostnames:
    - app.example.com
  rules:
    - backendRefs:
        - name: app-svc
          port: 8080
```

### Настройка параметров TLS через BackendTLSPolicy

Если трафик от шлюза к бэкенду должен идти по TLS, создайте BackendTLSPolicy в неймспейсе бэкенд-объекта Service. В примере ниже — HTTPRoute, бэкенд Service с именованным портом, ConfigMap с CA bundle и BackendTLSPolicy с TLS-валидацией для этого бэкенда:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: app-route
  namespace: prod
spec:
  parentRefs:
    - name: app-listeners
      namespace: prod
      kind: ListenerSet
      group: gateway.networking.k8s.io
      sectionName: app-https
      port: 443
  hostnames:
    - app.example.com
  rules:
    - backendRefs:
        - name: app-svc
          port: 8443
---
apiVersion: v1
kind: Service
metadata:
  name: app-svc
  namespace: prod
spec:
  selector:
    app: app
  ports:
    - name: https
      port: 8443
      targetPort: 8443
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-backend-ca
  namespace: prod
data:
  ca.crt: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
---
apiVersion: gateway.networking.k8s.io/v1
kind: BackendTLSPolicy
metadata:
  name: app-svc-tls
  namespace: prod
spec:
  targetRefs:
    - group: ""
      kind: Service
      name: app-svc
      sectionName: https
  validation:
    hostname: app.internal.example.com
    caCertificateRefs:
      - group: ""
        kind: ConfigMap
        name: app-backend-ca
```

GeoIP и трассировку OpenTelemetry настраивает администратор на ClusterALBInstance или ALBInstance, по инструкциям [«Использование GeoIP и GeoLite2»](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#geoip) и [«Настройка трассировки OpenTelemetry»](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#tracing).

### Поддерживаемые аннотации HTTPRoute {#поддерживаемые-аннотации-httproute}

Так как текущая спецификация Gateway API пока не покрывает все возможности, нужные для работы кластера DKP, модуль предоставляет аннотации HTTPRoute для недостающих параметров. Контроллер читает эти ключи из `HTTPRoute.metadata.annotations`.

| Аннотация | Описание |
| :--- | :--- |
| `alb.network.deckhouse.io/tls-disable-protocol` | Отключает протокол обработчика для маршрута с указанным именем хоста (например, значение `http2`). Может быть необходимо в редких случаях, когда используется общий сертификат с несколькими DNS-именами в сочетании с перенаправлением запросов |
| `alb.network.deckhouse.io/whitelist-source-range` | Ожидает список подсетей в формате CIDR через запятую: фильтр по IP на уровне маршрута; переопределяет глобальный whitelist (например, `10.1.1.10/32, 10.2.2.2/32`) |
| `alb.network.deckhouse.io/response-headers-to-add` | JSON-объект дополнительных заголовков ответа (например, `{"Strict-Transport-Security": "max-age=31536000; includeSubDomains"}`) |
| `alb.network.deckhouse.io/session-affinity` | JSON для закрепления сессии (session affinity) с режимом cookie (`mode`, `path`, `cookieName`, `ttl` и др.); не все поля обязательны (например, `{"mode": "cookie", "path": "/path", "cookieName": "mycookie", "ttl": 0}`) |
| `alb.network.deckhouse.io/hash-key` | Консистентный хеш для бэкендов Service у объекта HTTPRoute (например, `source-ip`) |
| `alb.network.deckhouse.io/service-upstream` | `"true"`: трафик к upstream идёт через соответствующий сервис, а не напрямую к подам |
| `alb.network.deckhouse.io/basic-auth-secret` | `namespace/secret` с данными htpasswd для HTTP Basic Auth на этом маршруте |
| `alb.network.deckhouse.io/satisfy` | `all` или `any`: определяет, нужно ли пройти обе проверки (whitelist и basic-auth) или достаточно одной (по умолчанию `all`) |
| `alb.network.deckhouse.io/auth-url` | Определяет URL внешнего сервиса аутентификации |
| `alb.network.deckhouse.io/auth-signin` | Определяет URL редиректа для авторизации в случае получения `401` от внешней аутентификации |
| `alb.network.deckhouse.io/auth-response-headers` | Список через запятую: дополнительные заголовки из ответа auth для передачи в upstream (поверх стандартного allowlist) |
| `alb.network.deckhouse.io/mod-security` | JSON-конфигурация для WAF ModSecurity/Coraza на уровне маршрута |
| `alb.network.deckhouse.io/rewrite-target` | Позволяет переопределять URL-путь для правил с типом `RegularExpression` с использованием совпавших частей регулярного выражения (например, `/my-path/\1`) |
| `alb.network.deckhouse.io/buffer-max-request-bytes` | Максимальный размер буфера для буферизации запроса, в байтах (целое число). По умолчанию Envoy Proxy не буферизует запросы |
| `alb.network.deckhouse.io/limit-rps` | Лимит RPS на маршрут |
| `alb.network.deckhouse.io/backend-tls-settings` | Например, `{"mode": "SIMPLE", "insecureSkipVerify": true, "clientCertificate": "", "privateKey": "", "caCertificates": "", "sni": "example.com", "secret": "<NAMESPACE>/<SECRET_NAME>"}`; позволяет явно указать параметры TLS подключения к upstream. `<NAMESPACE>` — неймспейс секрета; `<SECRET_NAME>` — имя секрета |
| `alb.network.deckhouse.io/idle-timeout` | Устанавливает per-route Envoy `idle_timeout`, в секундах. Аналогично таймаутам `proxy-read-timeout`/`proxy-send-timeout` в `ingress-nginx`: это таймаут неактивности, а не таймаут общей длительности запроса |
| `alb.network.deckhouse.io/proxy-buffer-size` | Задаёт максимальный размер заголовков ответа в настройках upstream-кластера; при превышении этого значения Envoy возвращает `503`. Аналогично `nginx.ingress.kubernetes.io/proxy-buffer-size` |

### Публикация приложения при включённом Istio-сайдкаре {#publishing-with-istio-sidecar}

Если для прокси шлюза включён Istio-сайдкар с помощью параметра [`istioSidecar`](/modules/alb/cr.html#albinstance-v1alpha1-spec-istiosidecar) объекта ALBInstance или [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-istiosidecar), трафик к бэкенду должен попадать в сайдкар через Service и содержать FQDN этого Service в заголовке `Host`.

Настройте HTTPRoute следующим образом:

- Добавьте аннотацию `alb.network.deckhouse.io/service-upstream: "true"`, чтобы трафик шёл через объект Service, а не напрямую к подам. Это эквивалент аннотации `nginx.ingress.kubernetes.io/service-upstream: "true"` из `ingress-nginx`.
- Добавьте фильтр `URLRewrite`, который задаёт в поле `hostname` FQDN объекта бэкенд-сервиса. Он заменяет аннотацию `nginx.ingress.kubernetes.io/upstream-vhost` из `ingress-nginx`.

Пример HTTPRoute для шлюза с Istio-сайдкаром:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: myservice
  namespace: myns
  annotations:
    alb.network.deckhouse.io/service-upstream: "true" # Трафик идёт через объект Service, чтобы его мог обработать Istio-сайдкар.
spec:
  parentRefs:
    - name: app-listeners # Имя объекта ListenerSet.
      namespace: myns
      kind: ListenerSet
      group: gateway.networking.k8s.io
      sectionName: app-https
      port: 443
  hostnames:
    - myservice.example.com
  rules:
    - filters:
        - type: URLRewrite
          urlRewrite:
            hostname: myservice.myns.svc # FQDN объекта backend-сервиса, чтобы сайдкар определил назначение.
      backendRefs:
        - name: myservice
          port: 80
```

### WAF на HTTPRoute {#waf-on-httproute}

Аннотация `alb.network.deckhouse.io/mod-security` позволяет включить WAF на базе ModSecurity/Coraza для отдельного HTTPRoute. Конфигурация WAF применяется только к маршруту, для которого указана аннотация, и не влияет на остальные маршруты.

Аннотация поддерживает следующие поля:

| Поле | Описание |
| :--- | :--- |
| `mode` | Режим работы WAF: `on`, `off`, либо любое другое значение для `DetectionOnly` |
| `preset` | Необязательный набор правил. Сейчас поддерживается только `owasp-crs`. Если поле не указано, набор правил не загружается |
| `paranoiaLevel` | Необязательный уровень CRS paranoia от `1` до `4`. Применяется только если `preset` равен `owasp-crs` |
| `configRef.namespace` | Необязательный неймспейс ConfigMap с пользовательскими правилами. По умолчанию используется неймспейс HTTPRoute |
| `configRef.name` | Имя ConfigMap с пользовательскими правилами |
| `configRef.key` | Необязательный ключ в ConfigMap. Если не указан, читаются все ключи в отсортированном порядке |
| `directives` | Необязательный список директив ModSecurity/Coraza, заданных непосредственно в аннотации; они добавляются после правил из набора и ConfigMap |

Директивы и правила применяются в следующем порядке:

1. Базовые директивы, поставляемые модулем (`@coraza.conf`, `SecRuleEngine`, `SecResponseBodyAccess Off`).
1. Правила из набора, заданного в `preset`.
1. Пользовательские правила из ConfigMap, указанного в `configRef`.
1. Директивы из поля `directives` аннотации.

Директивы из аннотации применяются последними и могут переопределять набор правил или правила из ConfigMap.

Ниже — примеры аннотации `alb.network.deckhouse.io/mod-security` для HTTPRoute.

{% tabs Примеры WAF %}
{% tab "mode: on" %}

Минимальная конфигурация: WAF включён без набора правил и без пользовательских директив.

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: app
  namespace: prod
  annotations:
    alb.network.deckhouse.io/mod-security: |
      {
        "mode": "on"
      }
spec:
  hostnames:
    - app.example.com
  parentRefs:
    - group: gateway.networking.k8s.io
      kind: ListenerSet
      name: app-listeners
      namespace: prod
      sectionName: app-https
      port: 443
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: app-svc
          port: 8080
```

{% endtab %}
{% tab "OWASP CRS" %}

WAF с предустановленным набором правил OWASP CRS и уровнем `paranoiaLevel`:

```yaml
metadata:
  annotations:
    alb.network.deckhouse.io/mod-security: |
      {
        "mode": "on",
        "preset": "owasp-crs",
        "paranoiaLevel": 1
      }
```

{% endtab %}
{% tab "ConfigMap" %}

WAF с OWASP CRS, пользовательскими правилами из ConfigMap и дополнительными директивами в аннотации:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: waf-rules
  namespace: prod
data:
  rules.conf: |
    SecRule ARGS:test "@streq block" \
      "id:1000001,phase:2,deny,status:403,msg:'test waf block'"
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: app
  namespace: prod
  annotations:
    alb.network.deckhouse.io/mod-security: |
      {
        "mode": "on",
        "preset": "owasp-crs",
        "paranoiaLevel": 1,
        "configRef": {
          "name": "waf-rules",
          "key": "rules.conf"
        },
        "directives": [
          "SecResponseBodyAccess Off"
        ]
      }
```

{% endtab %}
{% endtabs %}

Подробнее о синтаксисе правил и директив в документации Coraza и ModSecurity:

- [синтаксис правил Coraza и директива `SecRule`](https://www.coraza.io/docs/seclang/syntax/);
- [переменные ModSecurity](https://github.com/SpiderLabs/ModSecurity/wiki/Reference-Manual-%28v2.x%29-Variables);
- [операторы ModSecurity](https://github.com/SpiderLabs/ModSecurity/wiki/Reference-Manual-%28v2.x%29-Operators);
- [директива `SecRuleEngine` и другие директивы ModSecurity](https://github.com/owasp-modsecurity/ModSecurity/wiki/Reference-Manual-%28v2.x%29).

Текущие особенности и ограничения WAF:

- Поддерживается только набор правил `owasp-crs`.
- Параметр `paranoiaLevel` применяется только при использовании `preset: owasp-crs`. Если параметр `preset` не указан или имеет другое значение, параметр `paranoiaLevel` игнорируется.
- Допустимые значения `paranoiaLevel`: от `1` до `4`. На практике рекомендуется начинать со значения `1`.
- WAF проверяет только входящие запросы к приложению и при необходимости блокирует их. Ответы приложения клиенту не анализируются.
- Правила, заданные через ConfigMap, могут быть многострочными: строки, завершающиеся символом `\`, автоматически объединяются.

