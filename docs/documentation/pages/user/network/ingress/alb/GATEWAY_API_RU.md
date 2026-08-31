---
title: "Публикация приложений средствами Kubernetes Gateway API"
description: "Публикация приложений с Kubernetes Gateway API в Deckhouse Kubernetes Platform. ListenerSet, HTTPRoute, GRPCRoute, TLSRoute, TCPRoute, BackendTLSPolicy, аннотации HTTPRoute, WAF и GeoIP."
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

Создание управляемого Gateway (`ClusterALBInstance` или `ALBInstance`, инлеты, включение модуля) — задача администратора. Настройка инфраструктуры описана в разделе [«Включение модуля и создание Gateway»](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#создание-управляемого-объекта-gateway).

Этот сценарий предполагает, что объект ClusterALBInstance или ALBInstance уже создан и перешёл в состояние `Ready`. Запросите у администратора имя и неймспейс управляемого Gateway из [`status`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-status) инстанса.

Администратор неймспейса создаёт ListenerSet, привязанный к этому Gateway (`spec.parentRef`). Разработчики приложения создают объекты HTTPRoute, привязанные к ListenerSet.

Не привязывайте маршруты приложений к служебным слушателям Gateway `d8-http` / `d8-https`. Для публикации приложений используйте ListenerSet.

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

### Работа с объектами GRPCRoute, TLSRoute, TCPRoute и UDPRoute {#grpcroute-tlsroute-tcproute-and-udproute-objects}

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

Для TLS passthrough, когда расшифровка трафика должна выполняться на стороне приложения, можно использовать либо слушателя TLS, либо слушателя HTTPS.

{% tabs TLS passthrough %}
{% tab "Слушатель TLS" %}

Поскольку в этом примере слушатель TLS использует дополнительный порт, сначала настройте в ALBInstance параметр `additionalPorts`:

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

Если TLS нужно терминировать на шлюзе, а затем передать трафик дальше как TCP-поток к бэкенду, создайте объект ListenerSet со слушателем TLS и режимом `Terminate`, после чего подключите объект TCPRoute:

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
        mode: Terminate     # Режим TLS - терминация.
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

Для портов TCP и UDP из [`additionalPorts`](/modules/alb/cr.html#albinstance-v1alpha1-spec-inlet-additionalports) маршрут привязывается напрямую к слушателю управляемого Gateway, без отдельного ListenerSet. Иначе контроллер отклонит конфигурацию из-за пересечения обработчиков.

{% tabs TCP и UDP %}
{% tab "TCP" %}

Для публикации TCP-сервиса сначала откройте дополнительный TCP-порт в ALBInstance:

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

Для публикации UDP-сервиса сначала откройте дополнительный UDP-порт в ALBInstance:

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
1. Создайте объект ListenerSet с теми же именами хостов, портами и TLS-настройками. В `spec.parentRef` укажите новый объект Gateway.
1. В существующий объект HTTPRoute, в `parentRefs` добавьте ещё один объект, который указывает на новый объект ListenerSet.
1. Проверьте доступность приложения через новый шлюз.
1. После проверки удалите из `parentRefs` объекта HTTPRoute ссылку на неактуальные ListenerSet.

### Привязка маршрута в одном неймспейсе к ListenerSet объекту в другом неймспейсе

Если объект HTTPRoute должен подключаться к ListenerSet из другого неймспейса, в неймспейсе целевого ListenerSet добавьте ReferenceGrant.

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

### Настройка TLS для OpenTelemetry tracing

Чтобы передавать данные трассировки OpenTelemetry по TLS, создайте секрет с CA-сертификатом и укажите его в параметре `spec.openTelemetry.tracing.tls.caSecretName`.

- Для ClusterALBInstance или шлюза DKP по умолчанию разместите секрет в неймспейсе `d8-alb`.
- Для ALBInstance разместите секрет в том же неймспейсе, что и объект ALBInstance.

CA-сертификат должен быть сохранён в ключе `cacert`.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: otel-tracing-ca
  namespace: d8-alb
type: Opaque
stringData:
  cacert: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
---
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterALBInstance
metadata:
  name: proxy-gw
spec:
  gatewayName: proxy-gw
  openTelemetry:
    tracing:
      service:
        name: otel-collector
        namespace: monitoring
      port: 4318
      protocol: HTTP
      path: /v1/traces
      tls:
        sni: otel-collector.monitoring.svc.cluster.local
        caSecretName: otel-tracing-ca
```

### Поддерживаемые аннотации HTTPRoute {#поддерживаемые-аннотации-httproute}

Так как текущая спецификация Gateway API пока не покрывает все возможности, нужные для работы кластера DKP, модуль предоставляет аннотации HTTPRoute для недостающих параметров. Контроллер читает эти ключи из `HTTPRoute.metadata.annotations`.

| Аннотация | Описание |
| :--- | :--- |
| `alb.network.deckhouse.io/tls-disable-protocol` | Отключает протокол обработчика для маршрута с указанным именем хоста (например, значение `http2`). Может быть необходимо в редких случаях, когда используется общий сертификат с несколькими DNS-именами в сочетании с перенаправлением запросов |
| `alb.network.deckhouse.io/whitelist-source-range` | Ожидает список подсетей в формате CIDR через запятую: фильтр по IP на уровне маршрута; переопределяет глобальный whitelist (например, `10.1.1.10/32, 10.2.2.2/32`) |
| `alb.network.deckhouse.io/response-headers-to-add` | JSON-объект дополнительных заголовков ответа (например, `{"Strict-Transport-Security": "max-age=31536000; includeSubDomains"}`) |
| `alb.network.deckhouse.io/session-affinity` | JSON для закрепления сессии (session affinity) с режимом cookie (`mode`, `path`, `cookieName`, `ttl` и др.); не все поля обязательны (например, `{"mode": "cookie", "path": "/path", "cookieName": "mycookie", "ttl": 0}`) |
| `alb.network.deckhouse.io/hash-key` | Например, `source-ip`: консистентный хеш для бэкендов Service у объекта HTTPRoute |
| `alb.network.deckhouse.io/service-upstream` | `"true"`: трафик к upstream идёт через соответствующий сервис, а не напрямую к подам |
| `alb.network.deckhouse.io/basic-auth-secret` | `namespace/secret` с данными htpasswd для HTTP Basic Auth на этом маршруте |
| `alb.network.deckhouse.io/satisfy` | `all` или `any`: определяет необходимость удовлетворения обеих проверок (whitelist и basic-auth) или какой-либо одной (по умолчанию `all`) |
| `alb.network.deckhouse.io/auth-url` | Определяет URL внешнего сервиса аутентификации |
| `alb.network.deckhouse.io/auth-signin` | Определяет URL редиректа для авторизации в случае получения `401` от внешней аутентификации |
| `alb.network.deckhouse.io/auth-response-headers` | Список через запятую: дополнительные заголовки из ответа auth для передачи в upstream (поверх стандартного allowlist) |
| `alb.network.deckhouse.io/mod-security` | JSON-конфигурация для WAF ModSecurity/Coraza на уровне маршрута |
| `alb.network.deckhouse.io/rewrite-target` | Позволяет переопределять пути для правил с типом `RegularExpression` используя regex capture groups (например, `/my-path/\1`) |
| `alb.network.deckhouse.io/buffer-max-request-bytes` | Определяет размер буфера, который допускается использовать в случае буферизации запросов; значение указывается в байтах (целое число). По умолчанию Envoy Proxy не буферизует запросы |
| `alb.network.deckhouse.io/limit-rps` | Лимит RPS на маршрут |
| `alb.network.deckhouse.io/backend-tls-settings` | Например, `{"mode": "SIMPLE", "insecureSkipVerify": true, "clientCertificate": "", "privateKey": "", "caCertificates": "", "sni": "example.com", "secret": "<NAMESPACE>/<SECRET_NAME>"}`; позволяет явно указать параметры TLS подключения к upstream. `<NAMESPACE>` — неймспейс секрета; `<SECRET_NAME>` — имя секрета |
| `alb.network.deckhouse.io/idle-timeout` | Устанавливает per-route Envoy `idle_timeout`, в секундах. Схоже с `ingress-nginx` `proxy-read-timeout`/`proxy-send-timeout`; это таймаут неактивности, а не таймаут общей длительности запроса |
| `alb.network.deckhouse.io/proxy-buffer-size` | Задаёт максимальный размер заголовков ответа при настройке на upstream-кластере; при превышении этого значения Envoy возвращает `503`. Аналогично `nginx.ingress.kubernetes.io/proxy-buffer-size` |

### Публикация приложения при включённом Istio-сайдкаре {#publishing-with-istio-sidecar}

Если для прокси шлюза включён Istio-сайдкар с помощью параметра [`istioSidecar`](/modules/alb/cr.html#albinstance-v1alpha1-spec-istiosidecar) объекта ALBInstance или [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-istiosidecar), трафик к бэкенду должен попадать в сайдкар через Service и содержать FQDN этого Service в заголовке `Host`.

Настройте HTTPRoute следующим образом:

- добавьте аннотацию `alb.network.deckhouse.io/service-upstream: "true"`, чтобы трафик шёл через объект Service, а не напрямую к подам. Это эквивалент аннотации `nginx.ingress.kubernetes.io/service-upstream: "true"` из `ingress-nginx`;
- добавьте фильтр `URLRewrite`, который задаёт в поле `hostname` FQDN объекта бэкенд-сервиса. Он заменяет аннотацию `nginx.ingress.kubernetes.io/upstream-vhost` из `ingress-nginx`.

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

- поддерживается только набор правил `owasp-crs`;
- параметр `paranoiaLevel` применяется только при использовании `preset: owasp-crs`. Если параметр `preset` не указан или имеет другое значение, параметр `paranoiaLevel` игнорируется;
- допустимые значения `paranoiaLevel`: от `1` до `4`. На практике рекомендуется начинать со значения `1`;
- WAF проверяет только входящие запросы к приложению и при необходимости блокирует их. Ответы приложения клиенту не анализируются;
- правила, заданные через ConfigMap, могут быть многострочными: строки, завершающиеся символом `\`, автоматически объединяются.

### Использование GeoIP и GeoLite2 {#geoip}

Модуль `alb` поддерживает добавление полей GeoIP в заголовки HTTP-запросов на основе данных баз [MaxMind GeoIP/GeoLite2](https://dev.maxmind.com/geoip/).

На данный момент возможно подключение следующих редакций баз:

- GeoIP2-Anonymous-IP;
- GeoIP2-City;
- GeoIP2-ISP;
- GeoIP2-ASN;
- GeoLite2-ASN;
- GeoLite2-City.

{% alert level="info" %}
Текущая интеграция GeoIP поддерживает одновременное использование до 4 баз.
{% endalert %}

#### Скачивание баз GeoIP с MaxMind {#maxmind}

Для подключения GeoIP и скачивания баз непосредственно с серверов MaxMind необходимо предварительно создать секрет, содержащий лицензионный ключ, например:

```bash
d8 k -n prod create secret generic geoip-license --from-literal=licenseKey='<MAXMIND_LICENSE_KEY>'
```

{% alert level="info" %}
При настройке GeoIP для ClusterALBInstance секрет может быть размещён в любом неймспейсе, но рекомендуется разместить его в `d8-alb`.

Для объектов ALBInstance секрет должен располагаться строго в том же неймспейсе, что и объект ALBInstance.
{% endalert %}

После создания секрета необходимо указать его в объекте ClusterALBInstance или ALBInstance, например:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ALBInstance
metadata:
  name: main
  namespace: prod
spec:
  envoyLogLevel: Warning
  gatewayName: custom-gateway
  geoIP:
    licenseKeySecretRef:
      name: geoip-license
```

#### Скачивание баз GeoIP с локального зеркала {#local}

Для подключения GeoIP и скачивания баз с локального зеркала необходимо указать адрес зеркала в формате URL, например:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ALBInstance
metadata:
  name: main
  namespace: prod
spec:
  envoyLogLevel: Warning
  gatewayName: custom-gateway
  geoIP:
    maxmindMirror:
      url: "https://local.geoip:8443"
```

В качестве URL допускается указание адреса локального кеширующего сервера GeoIP в другом неймспейсе, например:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ALBInstance
metadata:
  name: main
  namespace: prod
spec:
  envoyLogLevel: Warning
  gatewayName: custom-gateway
  geoIP:
    maxmindMirror:
      url: "http://geoproxy-cluster.d8-alb.svc:8080/download"
```

#### Использование заголовков GeoIP {#headers}

В результате настройки GeoIP в неймспейсе, где работают прокси ClusterALBInstance или ALBInstance, будет запущен сервер кеширования и обновления баз GeoIP. Поды Envoy Proxy затем поочерёдно перезапускаются, чтобы скачивать базы с локального сервера GeoIP.

Для добавления полей GeoIP в заголовки HTTP-запросов необходимо указать имена HTTP-заголовков, которые будут содержать соответствующую информацию, например:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ALBInstance
metadata:
  name: main
  namespace: prod
spec:
  envoyLogLevel: Warning
  gatewayName: custom-gateway
  geoIP:
    headers:
      city: geoip_city
      country: geoip_country
    licenseKeySecretRef:
      name: geoip-license
    maxmindEditionIDs:
      - GeoLite2-City
```

Обновление баз GeoIP осуществляется раз в сутки как на кеширующем сервере, так и в каждом отдельном поде Envoy Proxy с использованием кеширующего сервера.

Параметр модуля [`storageClass`](/modules/alb/configuration.html#parameters-storageclass) задаёт PVC для компонентов GeoIP.

### Настройка трассировки OpenTelemetry {#tracing}

Модуль `alb` поддерживает экспорт трассировок OpenTelemetry из Envoy-прокси.

Для включения экспорта задайте в `spec.openTelemetry.tracing` адрес OpenTelemetry Collector:

- `service.name` и `service.namespace` — имя и неймспейс сервиса коллектора;
- `port` — порт;
- `protocol` — протокол (`HTTP` или `gRPC`);
- `path` — путь для OTLP/HTTP.

Альтернативно можно указать единый параметр [`url`](/modules/alb/cr.html#albinstance-v1alpha1-spec-opentelemetry-tracing-url).

При необходимости настройте подключение с использованием [TLS](/modules/alb/cr.html#albinstance-v1alpha1-spec-opentelemetry-tracing-tls).

При использовании TLS рекомендуется явно задать параметр [`sni`](/modules/alb/cr.html#albinstance-v1alpha1-spec-opentelemetry-tracing-tls-sni), если OpenTelemetry Collector находится за прокси или балансировщиком, который выбирает upstream на основе Server Name Indication.
