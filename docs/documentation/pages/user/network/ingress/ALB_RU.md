---
title: "Использование Application Load Balancer (ALB)"
description: "Настройка Application Load Balancer для HTTP/HTTPS/gRPC трафика в Deckhouse Kubernetes Platform. Использование ingress-nginx и istio для маршрутизации запросов, терминации SSL/TLS и публикации приложений."
permalink: ru/user/network/ingress/alb.html
lang: ru
extractedLinksMax: 2
relatedLinks:
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
  - title: "ALB средствами Kubernetes Gateway API"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html
  - title: "ALB средствами Ingress NGINX Controller"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/nginx.html
  - title: "ALB средствами Istio"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/istio.html
---

Публикация приложений и балансировка трафика на прикладном уровне может выполняться средствами:

- [Ingress NGINX Controller](#публикация-приложений-средствами-ingress-nginx-controller) (модуль `ingress-nginx`).
- [Kubernetes Gateway API](#публикация-приложений-средствами-kubernetes-gateway-api) (модуль `alb`).
- [Istio](#публикация-приложений-средствами-istio) (модуль `istio`).

## Сравнение вариантов ALB

### Ingress-nginx

ALB средствами Ingress NGINX Controller основана на базе веб-сервера nginx и реализуется модулем [`ingress-nginx`](/modules/ingress-nginx/).
Этот вариант подходит для:

- базовой маршрутизации трафика на основе доменов или URL;
- использования SSL/TLS для защиты трафика.

### Kubernetes Gateway API

ALB средствами [Kubernetes Gateway API](https://kubernetes.io/docs/concepts/services-networking/gateway/) реализуется [модулем `alb`](/modules/alb/). Шлюзы работают на Envoy Proxy, а приём и маршрутизация описываются стандартными объектами API (Gateway, ListenerSet, HTTPRoute и при необходимости GRPCRoute, TLSRoute, TCPRoute, UDPRoute, BackendTLSPolicy). Контроллер разворачивает необходимую инфраструктуру входа и проверяет конфигурацию, чтобы не допускать конфликтующих обработчиков.

Модель Gateway API разделяет ответственность между администратором кластера (ClusterALBInstance), администратором неймспейса (ALBInstance/ListenerSet) и командой приложения (команда разработчиков приложений, владельцы маршрутов HTTPRoute и других ресурсов маршрутизации).

Используйте этот вариант для:

- публикации приложений в модели Gateway API вместо классического Ingress;
- общекластерной точки входа или отдельного шлюза для приложения или команды в своём неймспейсе;
- маршрутизации HTTP/HTTPS, gRPC, TCP, UDP, а также терминации или сквозной передачи TLS;
- WAF на уровне маршрута, добавления полей GeoIP в заголовки HTTP-запросов, трассировки OpenTelemetry или Istio-сайдкара на прокси шлюза;
- параметров маршрута, которых нет в спецификации, через [аннотации `HTTPRoute`](#поддерживаемые-аннотации-httproute).

Сравнение с `ingress-nginx` и пояснения по терминологии — в разделе [«Балансировка входящего трафика»](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/).

### Istio

ALB на основе модуля [`istio`](/modules/istio/) позволяет получить расширенные возможности по управлению трафиком. Используйте этот вариант для:

- продвинутой маршрутизации, например для реализации [canary deployment](../canary-deployment.html);
- распределения трафика между версиями приложения и микросервисами;
- mTLS для шифрования трафика между подами;
- трассировки запросов.

## Публикация приложений средствами Ingress NGINX Controller

Для публикации приложений администратор кластера должен создать Ingress-контроллер. Имя этого объекта укажите в манифесте ресурса Ingress, который используется для маршрутизации входящего трафика для вашего приложения.

Пример базового Ingress-ресурса для публикации приложения.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-ingress
spec:
  ingressClassName: nginx # Имя Ingress-контроллера, предоставленного администратором кластера.
  rules:
  - host: application.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: productpage
            port:
              number: 80
```

## Публикация приложений средствами Kubernetes Gateway API

Публикация приложения возможна через общекластерный шлюз (используется объект ClusterALBInstance, который создается администратором кластера) или через отдельный шлюз для приложения или команды в выделенном неймспейсе (используется объект ALBInstance).

### Публикация приложения через объект ClusterALBInstance

Этот сценарий предполагает, что объект ClusterALBInstance уже создан администратором кластера и перешёл в состояние `Ready`. Запросите у администратора имя и неймспейс управляемого объекта Gateway (шлюза), через который будет публиковаться приложение.

Затем создайте объект ListenerSet, который будет привязан к нужному Gateway (параметр `spec.parentRef.name`) и объекты (маршруты) HTTPRoute для маршрутизации входящих запросов к приложению.

Пример ListenerSet и HTTPRoute для публикации приложения через общекластерный шлюз:

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
      port: 80 # Для HTTP трафика необходимо указывать 80 порт.
      protocol: HTTP
      hostname: app.example.com
    - name: app-https
      port: 443 # Для HTTPS трафика необходимо указывать 443 порт.
      protocol: HTTPS
      hostname: app.example.com
      tls:
        mode: Terminate
        certificateRefs:
          - name: app-tls   # Наименование секрета, содержащего необходимый TLS-сертификат.
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

### Публикация приложения через объект ALBInstance

В этом сценарии объекты ALBInstance, Gateway, ListenerSet и HTTPRoute находятся в одном неймспейсе.

Для публикации приложения через объект ALBInstance выполните следующие действия:

1. Создайте объект ALBInstance с учетом необходимых [настроек](/modules/alb/cr.html#albinstance):

   Пример минимальной рабочей конфигурации:

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
   ```

1. После того как объект ALBInstance перейдёт в состояние `Ready`, создайте объект ListenerSet, который будет привязан к нужному Gateway (параметр `spec.parentRef.name`) и объекты (маршруты) HTTPRoute для маршрутизации входящих запросов к приложению.

   Пример:

   ```yaml
   apiVersion: gateway.networking.k8s.io/v1
   kind: ListenerSet
   metadata:
     name: app-listeners
     namespace: prod
   spec:
     parentRef:
       name: app-gw   # Имя объекта Gateway из поля status ALBInstance.
       namespace: prod
     listeners:
       - name: app-https
         port: 443 # Для HTTPS трафика необходимо указывать 443 порт.
         protocol: HTTPS
         hostname: app.example.com
         tls:
           mode: Terminate
           certificateRefs:
             - name: app-tls   # Наименование секрета содержащего необходимый TLS-сертификат.
               namespace: prod
   ---
   apiVersion: gateway.networking.k8s.io/v1
   kind: HTTPRoute
   metadata:
     name: app-route
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
           - name: app-svc # Наименование сервиса приложения.
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
      port: 443 # Для HTTPS трафика необходимо указывать 443 порт.
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

Для TLS passthrough, когда расшифровка трафика должна выполняться на стороне приложения, можно использовать либо слушатель TLS, либо слушатель HTTPS. Ниже показан вариант со слушателем TLS.

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
apiVersion: gateway.networking.k8s.io/v1alpha3
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

Тот же сценарий можно реализовать и через слушатель HTTPS. Этот вариант особенно удобен, когда нужно использовать стандартный обработчик на порту `443`, так как не требуется открывать дополнительный порт для TLS passthrough:

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
        mode: Passthrough # Режим TLS - сквозной.
---
apiVersion: gateway.networking.k8s.io/v1alpha3
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

Контроллер модуля `alb` автоматически создаёт на управляемом объекте Gateway TCP-слушатель на основе `spec.inlet.additionalPorts`. Объект TCPRoute нужно привязывать напрямую к этому слушателю Gateway, без отдельного ListenerSet — иначе контроллер отклонит конфигурацию из-за пересечения обработчиков:

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

Контроллер модуля `alb` автоматически создаёт на управляемом объекте Gateway UDP-слушатель на основе `spec.inlet.additionalPorts`. Объект UDPRoute нужно привязывать напрямую к этому слушателю Gateway, без отдельного ListenerSet — иначе контроллер отклонит конфигурацию из-за пересечения обработчиков:

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

### Перевод приложения на публикацию через другой Gateway

Если приложение нужно опубликовать через другой объект Gateway, выполните следующие шаги:

1. Получите у администратора кластера имя и неймспейс объекта ClusterALBInstance или создайте объект ALBInstance, чтобы контроллер создал новый объект Gateway.
1. Создайте объект ListenerSet с теми же именами хостов, портами и TLS-настройками. В `spec.parentRef` укажите новый объект Gateway.
1. В существующий объект HTTPRoute, в `parentRefs` добавьте ещё один объект, который указывает на новый объект ListenerSet.
1. Проверьте доступность приложения через новый шлюз.
1. После проверки удалите из `parentRefs` объекта HTTPRoute ссылку на неактуальные ListenerSet.

### Привязка маршрута в одном неймспейсе к ListenerSet объекту в другом неймспейсе

Если объект HTTPRoute должен подключаться к объекту ListenerSet из другого неймспейса, в неймспейсе целевого ListenerSet добавьте объект ReferenceGrant. В примере ниже — общий ListenerSet в `shared-gw`, прикладной HTTPRoute в `prod` и ReferenceGrant в `shared-gw`, разрешающий такую привязку между неймспейсами:

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

Если трафик от шлюза к бэкенду должен идти по TLS, необходимо создать объект BackendTLSPolicy в неймспейсе бэкенд-объекта Service. В примере ниже показаны объект HTTPRoute, бэкенд-объект Service с именованным портом, ConfigMap с CA bundle и объект BackendTLSPolicy, который задаёт TLS-валидацию для этого бэкенда:

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

При использовании ClusterALBInstance или шлюза Deckhouse по умолчанию разместите секрет в неймспейсе `d8-alb`. CA-сертификат должен быть сохранён в ключе `cacert`.

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

Так как текущая спецификация Gateway API пока не покрывает все возможности, необходимые для корректной работы кластера DKP, модуль предоставляет постепенно расширяющийся набор аннотаций объекта HTTPRoute, который добавляет недостающие параметры конфигурации. Контроллер читает эти ключи из `HTTPRoute.metadata.annotations`.

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

Если для прокси шлюза включён Istio-сайдкар с помощью параметра [`istioSidecar`](/modules/alb/cr.html#albinstance-v1alpha1-spec-istiosidecar) объекта ALBInstance или [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-istiosidecar), трафик к бэкенду должен попадать в сайдкар через объект Service и содержать FQDN этого Service в заголовке `Host`. Настройте объект HTTPRoute следующим образом:

- добавьте аннотацию `alb.network.deckhouse.io/service-upstream: "true"`, чтобы трафик шёл через объект Service, а не напрямую к подам. Это эквивалент аннотации `nginx.ingress.kubernetes.io/service-upstream: "true"` из `ingress-nginx`;
- добавьте фильтр `URLRewrite`, который задаёт в поле `hostname` FQDN объекта бэкенд-сервиса. Он заменяет аннотацию `nginx.ingress.kubernetes.io/upstream-vhost` из `ingress-nginx`.

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

Пример HTTPRoute с включённым WAF (`"mode": "on"`):

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

Пример с набором правил OWASP CRS:

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

Пример с пользовательскими правилами из ConfigMap:

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

В результате настройки GeoIP в неймспейсе, где работают прокси ClusterALBInstance или ALBInstance, будет запущен сервер кеширования и обновления баз GeoIP, а поды Envoy Proxy будут поочередно перезапущены с добавлением функциональности скачивания баз GeoIP с локального сервера GeoIP.

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

При необходимости настройте подключение с использованием [TLS](/modules/alb/cr.html#albinstance-v1alpha1-spec-opentelemetry-tracing-tls). При использовании TLS рекомендуется явно задать параметр [`sni`](/modules/alb/cr.html#albinstance-v1alpha1-spec-opentelemetry-tracing-tls-sni), если OpenTelemetry Collector находится за прокси или балансировщиком, который выбирает upstream на основе Server Name Indication.

## Публикация приложений средствами Istio

При публикации приложения средствами Istio можно выбрать один из вариантов:

- [Использование Ingress NGINX](#публикация-приложений-с-использованием-ingress-nginx).
- [Использование Istio Ingress Gateway](#публикация-приложений-с-использованием-ресурса-istio-ingress-gateway).

### Публикация приложений с использованием Ingress NGINX

Для публикации приложения средствами Ingress NGINX администратор Deckhouse Kubernetes Platform должен настроить Ingress-контроллер, добавив к нему сайдкар от Istio.

Для публикации приложения подготовьте Ingress-ресурс, который ссылается на сервис. Обязательные аннотации для Ingress-ресурса:
  
- `nginx.ingress.kubernetes.io/service-upstream: "true"` — с этой аннотацией Ingress-контроллер будет отправлять запросы на ClusterIP сервиса (из диапазона Service CIDR) вместо того, чтобы отправлять их напрямую в поды приложения. Сайдкар-контейнер `istio-proxy` перехватывает трафик только в сторону диапазона Service CIDR, остальные запросы отправляются напрямую.
- `nginx.ingress.kubernetes.io/upstream-vhost: productpage.bookinfo.svc` — с этой аннотацией сайдкар сможет идентифицировать прикладной сервис, для которого предназначен запрос.

Примеры:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: productpage
  namespace: bookinfo
  annotations:
    # Включает проксирование трафика через nginx на ClusterIP вместо собственных IP подов.
    nginx.ingress.kubernetes.io/service-upstream: "true"
    # В Istio вся маршрутизация осуществляется на основе `Host:` заголовка запросов.
    # Это позволяет избежать необходимости указывать Istio о существовании внешнего домена `productpage.example.com`,
    # используется внутренний домен, известный Istio.
    nginx.ingress.kubernetes.io/upstream-vhost: productpage.bookinfo.svc
spec:
  rules:
    - host: productpage.example.com
      http:
        paths:
        - path: /
          pathType: Prefix
          backend:
            service:
              name: productpage
              port:
                number: 9080
```

```yaml
apiVersion: v1
kind: Service
metadata:
  name: productpage
  namespace: bookinfo
spec:
  ports:
  - name: http
    port: 9080
  selector:
    app: productpage
  type: ClusterIP
```

### Публикация приложений с использованием ресурса Istio Ingress Gateway

Для публикации приложения средствами Istio Ingress Gateway администратор Deckhouse Kubernetes Platform должен создать ресурс IngressIstioController.

Для публикации приложения с использованием ресурса Istio Ingress Gateway:

1. Создайте ресурс Gateway (шлюз). В нём, в поле `spec.selector` укажите лейбл, ссылающийся на ingressGatewayClass, и имя секрета, полученные от администратора кластера:

   ```yaml
   apiVersion: networking.istio.io/v1beta1
   kind: Gateway
   metadata:
     name: gateway-app
     namespace: app-ns
   spec:
     selector:
       # Селектор лейблов для использования Istio Ingress Gateway main-hp.
       istio.deckhouse.io/ingress-gateway-class: istio-hp
     servers:
       - port:
           # Стандартный шаблон для использования протокола HTTP.
           number: 80
           name: http
           protocol: HTTP
         hosts:
           - app.example.com
       - port:
           # Стандартный шаблон для использования протокола HTTPS.
           number: 443
           name: https
           protocol: HTTPS
         tls:
           mode: SIMPLE
           # Ресурс Secret с сертификатом и ключом, который должен быть создан администратором в неймспейсе d8-ingress-istio.
           # Поддерживаемые форматы Secret можно посмотреть по ссылке https://istio.io/latest/docs/tasks/traffic-management/ingress/secure-ingress/#key-formats.
           credentialName: app-tls-secret
         hosts:
           - app.example.com
   ```

1. Определите правила маршрутизации с помощью VirtualService, который связывает шлюз и обслуживаемый им сервис:

   ```yaml
   apiVersion: networking.istio.io/v1alpha3
   kind: VirtualService
   metadata:
     name: vs-app
     namespace: app-ns
   spec:
     gateways:
       - gateway-app # Имя ресурса Gateway, созданного на предыдущем шаге.
     hosts:
       - app.example.com
     http:
       - route:
           - destination:
               host: app-svc # Имя сервиса, на который нужно направить трафик.
   ```

## Балансировка gRPC

Чтобы балансировка gRPC-сервисов заработала автоматически, присвойте имя с префиксом или значением `grpc` для порта соответствующему объекту Service.
