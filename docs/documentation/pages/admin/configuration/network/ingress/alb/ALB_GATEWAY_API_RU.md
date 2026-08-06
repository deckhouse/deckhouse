---
title: "ALB средствами Kubernetes Gateway API"
permalink: ru/admin/configuration/network/ingress/alb/alb-gateway-api.html
description: "Публикация приложений с помощью Kubernetes Gateway API."
lang: ru
---

Для реализации ALB средствами [Kubernetes Gateway API](https://kubernetes.io/docs/concepts/services-networking/gateway/) используется модуль [`alb`](/modules/alb/).

Модуль `alb` реализует прикладной балансировщик нагрузки (Application Load Balancer, ALB) и позволяет публиковать приложения с помощью Kubernetes Gateway API. Он разворачивает и настраивает инфраструктуру для приёма и маршрутизации внешних запросов, а также проверяет пользовательскую конфигурацию Gateway API.

Модуль построен на Kubernetes Gateway API — современном стандарте управления входящим трафиком, который приходит на смену Ingress API. Он предоставляет:

- единый декларативный API для HTTP/HTTPS, gRPC, TCP, UDP и TLS passthrough;
- разделение ответственности между администратором кластера (ClusterALBInstance), администратором неймспейса (ALBInstance/ListenerSet) и командой приложения (маршруты);
- расширенные возможности обработки запросов: WAF на уровне маршрута, внешнюю аутентификацию, списки разрешённых IP-адресов, ограничение частоты запросов, session affinity, GeoIP, BackendTLSPolicy, Proxy Protocol и HTTP/3.

Несмотря на схожие названия, Kubernetes Gateway API и API Gateway — разные понятия. Kubernetes Gateway API — набор ресурсов Kubernetes, описывающих маршрутизацию входящего трафика к сервисам. API Gateway — архитектурный компонент, который объединяет API приложений за единой точкой входа. Модуль `alb` является реализацией Kubernetes Gateway API.

Сравнение возможностей с `ingress-nginx` — в разделе [Балансировка входящего трафика](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/#сравнение-возможностей-модулей-ingress-nginx-и-alb).

{% alert level="info" %}
ALB средствами Kubernetes Gateway API может использоваться в кластере совместно с ALB средствами Ingress NGINX Controller.
Подробнее — в разделе [«Совместное использование с другими модулями и сторонними решениями»](#совместное-использование-с-другими-модулями-и-сторонними-решениями).
{% endalert %}

## Валидация конфигурации Gateway API

Помимо настройки инфраструктуры Gateway API, модуль `alb` валидирует пользовательские настройки, чтобы не допустить применения конфликтующих конфигураций. Например, он выявляет конфликты между одинаковыми обработчиками трафика в разных объектах ListenerSet, если они ссылаются на один и тот же объект Gateway.

## Действия перед включением и настройкой ALB в кластере

Перед включением и настройкой ALB в кластере Deckhouse Kubernetes Platform (DKP):

- Если необходимо [публиковать служебные домены](#публикация-служебных-доменов) ([веб-интерфейсы служебных компонентов DKP](/products/kubernetes-platform/documentation/v1/user/web/ui.html) и других модулей), убедитесь, что указан глобальный параметр [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate). Если параметр `publicDomainTemplate` не указан, системные объекты HTTPRoute/Gateway/ListenerSet будут создаваться некорректно и веб-интерфейсы служебных компонентов DKP и других модулей не будут опубликованы.
- Если в кластере используются сторонние решения Gateway API, [проверьте совместимость](#совместное-использование-с-alb-на-основе-сторонних-решений-gateway-api) используемых версий API для объектов сторонних решений Gateway API с версиями, требуемыми для контроллера модуля `alb`.

## Совместное использование с другими модулями и сторонними решениями

ALB средствами Kubernetes Gateway API в кластере DKP можно использовать совместно с ALB средствами Ingress NGINX Controller, а также с ALB на основе сторонних решений Gateway API.

### Совместное использование с ALB средствами Ingress NGINX Controller

ALB средствами Kubernetes Gateway API может использоваться в кластере совместно с [ALB средствами Ingress NGINX Controller](nginx.html). В таком случае для каждого из типов ALB рекомендуется использовать отдельный объект ClusterIssuer, чтобы раздельно управлять настройками и жизненными циклами сертификатов для обоих типов ALB.

{% alert level="info" %}
Для шлюза DKP по умолчанию объект ClusterIssuer создаётся автоматически. Этот же объект ClusterIssuer используется для выпуска сертификатов системных доменов.
{% endalert %}

### Совместное использование с ALB на основе сторонних решений Gateway API

Использование сторонних решений Gateway API допускается при условии, что в кластере используются следующие, совместимые с контроллером модуля `alb`, версии API для объектов Gateway API:

- BackendTLSPolicy: v1;
- GatewayClass: v1;
- Gateway: v1;
- ListenerSet: v1;
- GRPCRoute: v1;
- HTTPRoute: v1;
- ReferenceGrant: v1beta1;
- TCPRoute: v1alpha2/v1;
- UDPRoute: v1alpha2/v1;
- TLSRoute: v1.

Контроллер модуля `alb` в процессе запуска проверяет текущие хранимые версии объектов Gateway API. В случае обнаружения расхождения между установленными и требуемыми версиями контроллер прекращает работу. Если же в кластере полностью отсутствует тот или иной тип объекта Gateway API, нужная версия будет создана контроллером автоматически и он продолжит работу.

Для ручной проверки совместимости версий установленных в кластере объектов Gateway API с требуемыми версиями используйте команду:

```bash
declare -A want=(
    [gatewayclasses.gateway.networking.k8s.io]=v1
    [gateways.gateway.networking.k8s.io]=v1
    [grpcroutes.gateway.networking.k8s.io]=v1
    [httproutes.gateway.networking.k8s.io]=v1
    [listenersets.gateway.networking.k8s.io]=v1
    [referencegrants.gateway.networking.k8s.io]=v1beta1
    [tcproutes.gateway.networking.k8s.io]="v1|v1alpha2"
    [udproutes.gateway.networking.k8s.io]="v1|v1alpha2"
    [tlsroutes.gateway.networking.k8s.io]=v1
    [backendtlspolicies.gateway.networking.k8s.io]=v1
)

for crd in "${!want[@]}"; do
    got="$(
        d8 k get crd "$crd" -o jsonpath='{.spec.versions[?(@.storage==true)].name}' 2>/dev/null || true
    )"
    if [[ -n "$got" && "$got" =~ ^(${want[$crd]})$ ]]; then
        echo "$crd OK storage=$got"
    else
        echo "$crd FAILED cluster=${got:-MISSING} expected=${want[$crd]}"
    fi
done | sort
```

В остальном модуль конфигурирует и управляет только объектами Gateway определённого GatewayClass, что минимизирует риск возникновения конфликтов при использовании сторонних решений Gateway API.

## Публикация приложений

Процесс публикации приложения включает следующие шаги:

1. [Создание управляемого объекта Gateway (шлюза)](#создание-управляемого-объекта-gateway) с помощью cluster-scoped (используется ресурс [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance)) или namespaced- (используется ресурс [ALBInstance](/modules/alb/cr.html#albinstance)) кастомного ресурса.
1. [Создание объекта ListenerSet](#создание-объектов-listenerset-для-управления-приёмом-входящих-запросов), который привязывается к созданному на предыдущем шаге объекту Gateway. ListenerSet управляет приёмом входящих запросов.
1. [Создание объектов (маршрутов)](#создание-маршрутов-и-настройка-маршрутизации) для маршрутизации входящих запросов к приложению. Объекты HTTPRoute, GRPCRoute и TLSRoute привязываются к ListenerSet. TCPRoute и UDPRoute для обычных TCP/UDP-портов из [`additionalPorts`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-additionalports) привязываются напрямую к слушателю объекта Gateway.

### Создание управляемого объекта Gateway

При создании управляемого объекта Gateway для публикации пользовательских приложений используются кастомные ресурсы [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance) (cluster-scoped-объект) и [ALBInstance](/modules/alb/cr.html#albinstance) (namespaced-ресурс).

Особенности этих ресурсов и разница между ними описаны в таблице:

| | **ClusterALBInstance** | **ALBInstance** |
| :--- | :--- | :--- |
| Назначение | Развёртывание cluster-scoped-объекта Gateway | Развёртывание namespaced-объекта Gateway |
| Сценарии использования | - Общая точка входа (общекластерный шлюз).<br> - Системный шлюз для публикации веб-интерфейсов служебных компонентов DKP и других модулей (может требоваться [подготовка кластера](#действия-перед-включением-и-настройкой-alb-в-кластере)).<br> - Платформенный шлюз | Отдельный шлюз для приложения или команды в выделенном неймспейсе |
| Поддерживаемые типы инлета | `LoadBalancer`, `HostPort` | `LoadBalancer` |
| Реализация прокси | Envoy Proxy | Envoy Proxy |
| Тип развёртывания | DaemonSet | Deployment |
| Локализация объектов ListenerSet и маршрутов | В любом пользовательском неймспейсе | В том же неймспейсе, что и объект ALBInstance |
| Права доступа | Администратор кластера | Администратор неймспейса |

После создания ClusterALBInstance или ALBInstance в кластере появляется управляемый объект Gateway (шлюз). При этом:

- Каждый объект Gateway обслуживается как минимум одним экземпляром Envoy Proxy.
- Трафик в него приходит через объект Service типа `LoadBalancer` или напрямую с использованием параметров `HostPort`.
- Каждый объект Gateway по умолчанию создаёт два обработчика: `d8-http` (порт `80`) и `d8-https` (порт `443`). Они предназначены для служебных целей. Например, для проверки доступности шлюза или работы cert-manager (HTTP-01). Для публикации приложений эти обработчики использовать не рекомендуется, используйте для этого ListenerSet.
- На один объект Gateway могут ссылаться несколько объектов ClusterALBInstance или ALBInstance (через поле `gatewayName`). В этом случае они описывают общий шлюз, но инфраструктура приёма запросов может отличаться в зависимости от настроек. Можно рассматривать `gatewayName` как аналог `ingressClass` для объектов [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller): данное поле определяет, информация о каких маршрутах будет включена в конфигурацию конкретного экземпляра ALB..

{% alert level="warning" %}
Ручная модификация объектов Gateway, управляемых модулем, не допускается.
{% endalert %}

Пример манифеста ресурса ClusterALBInstance для создания общекластерного шлюза:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterALBInstance
metadata:
  name: public-gw
spec:
  gatewayName: public-gw
  inlet:
    type: LoadBalancer
```

Пример манифеста ресурса ALBInstance для создания отдельного шлюза для приложения или команды в выделенном неймспейсе представлен в разделе [«Публикация приложения через объект ALBInstance»](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb.html#публикация-приложения-через-объект-albinstance).

### Создание объектов ListenerSet для управления приёмом входящих запросов

Объект ListenerSet описывает системные и пользовательские обработчики трафика, которые задают имя хоста, режим TLS, порт и протокол. Каждый объект ListenerSet связывается с конкретным родительским объектом Gateway через поле `spec.parentRef`, а затем к нему подключаются маршруты.

Расположение объектов ListenerSet зависит от используемого типа объекта Gateway:

- для ClusterALBInstance объекты ListenerSet могут располагаться в любом неймспейсе;
- для ALBInstance объекты ListenerSet рекомендуется располагать в том же неймспейсе, что и родительский ALBInstance.

В обоих случаях объект ListenerSet рекомендуется располагать в том же неймспейсе, что и подключаемые к нему объекты HTTPRoute, GRPCRoute и TLSRoute. Это упрощает читаемость конфигурации и позволяет избежать дополнительных настроек, например, объектов ReferenceGrant. TCPRoute и UDPRoute для обычных TCP/UDP-портов из `additionalPorts` привязываются напрямую к слушателю объекта Gateway.

Пример манифеста ресурса ListenerSet для управления приёмом входящих HTTP- и HTTPS-запросов через общекластерный шлюз:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: app-listeners
  namespace: prod
spec:
  parentRef:
    name: public-gw   # Имя общекластерного Gateway из status ClusterALBInstance.
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

```

### Создание маршрутов и настройка маршрутизации

Для маршрутизации входящих запросов используются следующие типы маршрутов:

- HTTPRoute — для маршрутизации HTTP/HTTPS/TLS запросов. Объекты HTTPRoute поддерживают расширенные настройки с помощью аннотаций, которые дополняют текущую спецификацию Gateway API.
- GRPCRoute — для маршрутизации gRPC-трафика.
- TLSRoute — для сквозной маршрутизации TLS-трафика.
- TCPRoute — для маршрутизации TCP-трафика. Для обычных TCP-портов из `additionalPorts` объект TCPRoute привязывается напрямую к слушателю Gateway, а не к ListenerSet.
- UDPRoute — для маршрутизации UDP-трафика. Для обычных UDP-портов из `additionalPorts` объект UDPRoute привязывается напрямую к слушателю Gateway, а не к ListenerSet.

Объекты HTTPRoute, GRPCRoute и TLSRoute привязываются к ListenerSet. TCPRoute и UDPRoute для обычных TCP/UDP-портов из `additionalPorts` привязываются напрямую к слушателю объекта Gateway.

Пример маршрута для HTTP-трафика:

```yaml
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
        - name: app-svc # Имя сервиса, обслуживающего приложение.
          port: 8080

```

Пример маршрута для HTTPS-трафика:

```yaml
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
        - name: app-svc # Имя сервиса, обслуживающего приложение.
          port: 8080

```

## Публикация служебных доменов

{% alert level="warning" %}
Если нужно публиковать служебные домены, убедитесь, что указан глобальный параметр [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate). Если параметр `publicDomainTemplate` не указан, системные объекты HTTPRoute/Gateway/ListenerSet будут создаваться некорректно и веб-интерфейсы служебных компонентов DKP и других модулей не будут опубликованы.
{% endalert %}

Для предоставления доступа к служебным доменам кластера DKP укажите шлюз по умолчанию. Для этого выполните следующие действия:

1. Создайте cluster-scoped объект ClusterALBInstance с нужным типом инлета и [настройками](/modules/alb/cr.html#clusteralbinstance). Установите параметр [`spec.defaultDeckhouseGateway: true`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-defaultdeckhousegateway) для этого ClusterALBInstance.

   Пример манифеста cluster-scoped-объекта ClusterALBInstance c параметром `spec.defaultDeckhouseGateway: true`:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: ClusterALBInstance
   metadata:
     name: public-gw
   spec:
     gatewayName: public-gw
     defaultDeckhouseGateway: true
     inlet:
       type: LoadBalancer
   ```

1. После применения изменений проверьте статус объекта ClusterALBInstance с помощью команды:

   ```bash
   d8 k get clusteralbinstances
   ```

   У объекта ClusterALBInstance должен появиться управляемый объект Gateway, а сам инстанс должен перейти в готовое состояние. После этого в соответствующих системных неймспейсах кластера должны появиться системные объекты ListenerSet и HTTPRoute.

### Алгоритм выбора шлюза DKP по умолчанию при использовании нескольких ClusterALBInstance

В кластере может быть одновременно несколько cluster-scoped Gateway, помеченных как шлюз по умолчанию (флаг [`spec.defaultDeckhouseGateway: true`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-defaultdeckhousegateway) в параметрах соответствующих ClusterALBInstance). В этом случае шлюзом по умолчанию становится Gateway, созданный самым старым объектом ClusterALBInstance (возраст определяется по `creationTimestamp`). Если ни один объект ClusterALBInstance не отмечен как шлюз по умолчанию, DKP допускает использование объекта Gateway, созданного модулем `alb` для инстанса ClusterALBInstance с именем `main`, в качестве шлюза по умолчанию.

### Смена шлюза DKP по умолчанию

Если системные домены DKP необходимо перевести на обслуживание другим объектом Gateway, выполните следующие шаги:

1. Создайте новый объект ClusterALBInstance, описывающий необходимые настройки, и задайте в нём параметр [`spec.defaultDeckhouseGateway: true`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-defaultdeckhousegateway).
1. В текущем объекте ClusterALBInstance, который предоставляет шлюз DKP по умолчанию, задайте `spec.defaultDeckhouseGateway: false`.
1. Проверьте, что все системные объекты ListenerSet теперь ссылаются на новый объект Gateway в `spec.parentRef`.

## Смена инлета с сохранением текущего Gateway {#change-inlet}

Чтобы сменить инлет, используемый для уже созданного объекта Gateway, выполните следующие действия:

1. Создайте новый объект ClusterALBInstance или объект ALBInstance с другим именем, но с тем же значением `spec.gatewayName`, используя нужный тип инлета.
1. Проверьте, что новый путь приёма трафика работает корректно.
1. Удалите неактуальный объект ClusterALBInstance или объект ALBInstance.

Так как `gatewayName` не меняется, объект Gateway остаётся прежним. В большинстве случаев объект ListenerSet и маршруты при этом можно не пересоздавать.

## Открытие дополнительного TCP/UDP-порта на общекластерном Gateway {#tcp-port}

Если кроме стандартных HTTP/HTTPS-слушателей на шлюзе нужен отдельный TCP/UDP-порт, добавьте в соответствующий ClusterALBInstance поле [`spec.inlet.additionalPorts`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-additionalports), например:

```yaml
spec:
  gatewayName: public-gw
  inlet:
    type: LoadBalancer
    loadBalancer: {}
    additionalPorts:
      - port: 9000
        protocol: TCP
```

Контроллер добавит на управляемый объект Gateway соответствующий обработчик TCP/UDP-трафика с именем секции (`sectionName`) вида `tcp-port-9000`. Затем можно создать объект TCPRoute, который ссылается на этот объект Gateway и этот `sectionName` напрямую:

```yaml
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: TCPRoute
metadata:
  name: app-tcp
  namespace: prod
spec:
  parentRefs:
    - name: public-gw
      namespace: d8-alb
      sectionName: tcp-port-9000
      port: 9000
  rules:
    - backendRefs:
        - name: tcp-svc
          port: 9000
```

{% alert level="info" %}
Если объект TCPRoute или UDPRoute создаётся в неймспейсе, отличном от неймспейса целевого Gateway, дополнительно необходимо создать соответствующий объект ReferenceGrant.
{% endalert %}

[Примеры UDPRoute и шаги публикации приложений](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb.html#grpcroute-tlsroute-tcproute-and-udproute-objects) приведены в пользовательской документации.

## Конфликты портов при использовании нескольких ClusterALBInstance/ALBInstance для одного Gateway {#conflicts}

Если один и тот же объект Gateway обслуживается несколькими объектами ClusterALBInstance или ALBInstance, итоговый набор слушателей, который попадает в объект Gateway, берётся из самого старого объекта ClusterALBInstance или ALBInstance. Для остальных инстансов в статусе появляется признак конфликта портов с указанием имени «управляющего» инстанса.

## Примеры конфигурации инфраструктуры {#infrastructure-examples}

Модуль поддерживает два типа инлета:

- `LoadBalancer` — приём трафика через объект Service с типом `LoadBalancer` (облачные провайдеры или bare metal с MetalLB). Доступен и для ClusterALBInstance, и для ALBInstance.
- `HostPort` — приём трафика на портах узлов без внешнего балансировщика. Доступен только для ClusterALBInstance.

Полное описание полей CR: [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance) и [ALBInstance](/modules/alb/cr.html#albinstance). Дополнительные примеры — в [примерах модуля `alb`](/modules/alb/examples.html).

### Облачный провайдер (инлет LoadBalancer) {#cloud-load-balancer}

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterALBInstance
metadata:
  name: main
spec:
  gatewayName: public-gw
  inlet:
    type: LoadBalancer
    loadBalancer: {}
```

Чтобы задать параметры облачного балансировщика, укажите нужные аннотации объекта Service в [`spec.inlet.loadBalancer.serviceAnnotations`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-loadbalancer-serviceannotations). Например, для Network Load Balancer в AWS:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterALBInstance
metadata:
  name: main
spec:
  gatewayName: public-gw
  inlet:
    type: LoadBalancer
    loadBalancer:
      serviceAnnotations:
        service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
```

### Bare metal с балансировщиком MetalLB {#bare-metal-metallb}

1. Включите модуль [`metallb`](/modules/metallb/).
1. Создайте объект MetalLoadBalancerClass с пулом адресов. Разместите балансировщики MetalLB на тех же узлах, что и поды Envoy Proxy модуля `alb` (обычно frontend-узлы с лейблом `node-role.deckhouse.io/frontend`):

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: MetalLoadBalancerClass
   metadata:
     name: alb
   spec:
     addressPool:
       - 192.168.2.100-192.168.2.150
     isDefault: false
     nodeSelector:
       node-role.deckhouse.io/frontend: ""
     type: L2
   ```

1. Создайте объект ClusterALBInstance и укажите [`spec.inlet.loadBalancer.loadBalancerClass`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-loadbalancer-loadbalancerclass):

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: ClusterALBInstance
   metadata:
     name: main
   spec:
     gatewayName: public-gw
     inlet:
       type: LoadBalancer
       loadBalancer:
         loadBalancerClass: alb
         serviceAnnotations:
           # Число адресов, выделяемых из пула MetalLoadBalancerClass.
           network.deckhouse.io/l2-load-balancer-external-ips-count: "1"
   ```

### Bare metal без внешнего балансировщика (инлет HostPort) {#bare-metal-hostport}

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterALBInstance
metadata:
  name: main
spec:
  gatewayName: public-gw
  inlet:
    type: HostPort
    hostPort:
      httpPort: 80
      httpsPort: 443
```

Чтобы поды Envoy Proxy размещались только на выделенных узлах, задайте [`nodeSelector`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-nodeselector) и [`tolerations`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-tolerations) для ClusterALBInstance.

### Приём трафика за внешним L7-балансировщиком (Proxy Protocol) {#proxy-protocol}

Если модуль `alb` работает за внешним L7-балансировщиком (например, Cloudflare или Qrator), включите [`useProxyProtocol`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-useproxyprotocol), чтобы получать реальные адреса клиентов. Дополнительно ограничьте с помощью [`spec.originalIPDetection`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-originalipdetection) список подсетей, из которых разрешено доверять заголовкам с адресом клиента.

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterALBInstance
metadata:
  name: main
spec:
  gatewayName: public-gw
  inlet:
    type: HostPort
    hostPort:
      httpPort: 80
      httpsPort: 443
  useProxyProtocol: true
  originalIPDetection:
    setRealIPFrom:
      - 10.0.0.0/16
```

{% alert level="warning" %}
Proxy Protocol и HTTP/3 нельзя включать одновременно.
{% endalert %}

### Разделение публичной и административной зон {#public-and-admin-zones}

Создайте отдельный объект Gateway для каждой зоны и ограничьте приём административного трафика с помощью [`spec.acceptRequestsFrom`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-acceptrequestsfrom). Решение о допуске соединения принимается по реальному адресу подключения, а не по заголовкам запроса.

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterALBInstance
metadata:
  name: public
spec:
  gatewayName: public-gw
  inlet:
    type: LoadBalancer
    loadBalancer: {}
---
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterALBInstance
metadata:
  name: admin
spec:
  gatewayName: admin-gw
  inlet:
    type: LoadBalancer
    loadBalancer: {}
  acceptRequestsFrom:
    - 1.2.3.4/32
    - 10.0.0.0/16
```

Далее для каждого шлюза создайте отдельные объекты ListenerSet и маршруты. Примеры публикации приложений: [Использование Application Load Balancer (ALB)](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb.html).

### Публикация в неймспейсе (ALBInstance) {#namespaced-load-balancer}

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
    loadBalancer: {}
```

После перехода ALBInstance в состояние `Ready` создайте объекты ListenerSet и HTTPRoute в том же неймспейсе. Дальнейшие шаги — в разделе [Публикация приложения через объект ALBInstance](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb.html#публикация-приложения-через-объект-albinstance).

## FAQ и дополнительные материалы {#faq}

Ответы на частые вопросы по модулю `alb` собраны в [FAQ модуля `alb`](/modules/alb/faq.html):

- [Поддерживает ли модуль приём TCP-трафика?](/modules/alb/faq.html#does-the-module-support-receiving-tcp-traffic) — также [открытие дополнительного TCP/UDP-порта](#tcp-port) и [примеры публикации маршрутов](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb.html#grpcroute-tlsroute-tcproute-and-udproute-objects);
- [Поддерживает ли модуль приём UDP-трафика?](/modules/alb/faq.html#does-the-module-support-receiving-udp-traffic);
- [Как сконфигурировать балансировщик нагрузки для проверки доступности ClusterALBInstance/ALBInstance?](/modules/alb/faq.html) (endpoint `/healthz` на порту `80`).

Полный набор примеров инлетов и зон — в [примерах модуля `alb`](/modules/alb/examples.html). Справочник параметров: [конфигурация модуля `alb`](/modules/alb/configuration.html) и [Custom Resources модуля `alb`](/modules/alb/cr.html).

## Просмотр конфигурации Envoy Proxy {#envoy-config}

Для диагностики полезно посмотреть, какую конфигурацию контроллер и конфигуратор прокси передали в Envoy Proxy, обслуживающий объект Gateway.

Для этого выполните следующие действия:

1. Выберите под Envoy Proxy для нужного объекта Gateway:

   ```bash
   d8 k -n d8-alb get pods -l alb.deckhouse.io/gateway=shared-gateway
   ```

1. Получите конфигурацию пода с помощью команды (вместо `<envoy-proxy-pod-name>` используйте имя пода Envoy Proxy, полученное на предыдущем шаге):

   ```bash
   d8 k -n d8-alb exec -it <envoy-proxy-pod-name> pilot-agent request GET /config_dump
   ```

   Если нужен только отдельный раздел конфигурации, явно укажите его:

   ```bash
   d8 k -n d8-alb exec -it <envoy-proxy-pod-name> pilot-agent request GET /config_dump?resource=dynamic_listeners
   d8 k -n d8-alb exec -it <envoy-proxy-pod-name> pilot-agent request GET /config_dump?resource=dynamic_route_configs
   d8 k -n d8-alb exec -it <envoy-proxy-pod-name> pilot-agent request GET /config_dump?resource=dynamic_active_clusters
   ```

Так можно проверить, появились ли ожидаемые обработчики трафика, виртуальные хосты и upstream-кластеры после изменения объекта ListenerSet или объекта Route.
