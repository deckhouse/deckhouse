---
title: "ALB средствами Kubernetes Gateway API"
permalink: ru/admin/configuration/network/ingress/alb/alb-gateway-api.html
description: "Публикация приложений с помощью Kubernetes Gateway API."
lang: ru
---

Для реализации ALB средствами [Kubernetes Gateway API](https://kubernetes.io/docs/concepts/services-networking/gateway/) используется модуль [`alb`](/modules/alb/).

Модуль `alb` реализует прикладной балансировщик нагрузки (Application Load Balancer, ALB) и позволяет публиковать приложения с помощью Kubernetes Gateway API. Он разворачивает и настраивает инфраструктуру для приёма и маршрутизации внешних запросов, а также проверяет пользовательскую конфигурацию Gateway API.

{% alert level="info" %}
ALB средствами Kubernetes Gateway API может использоваться в кластере совместно с ALB средствами Ingress NGINX Controller.
Подробнее — в разделе [«Совместное использование с другими модулями и сторонним решениями»](#совместное-использование-с-другими-модулями-и-сторонним-решениями).
{% endalert %}

## Валидация конфигурации Gateway API

Помимо настройки инфраструктуры Gateway API, модуль `alb` валидирует пользовательские настройки, чтобы не допустить применения конфликтующих конфигураций. Например, он выявляет конфликты между одинаковыми обработчиками трафика в разных объектах ListenerSet, если они ссылаются на один и тот же объект Gateway.

## Действия перед включением и настройкой ALB в кластере

Перед включением и настройкой ALB в кластере DKP:

- Убедитесь, что указан глобальный параметр [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate). **Проверка актуальна, если необходимо [публиковать служебные домены](#публикация-служебных-доменов): [веб-интерфейсы служебных компонентов DKP](/products/kubernetes-platform/documentation/v1/user/web/ui.html) и других модулей**. Если параметр `publicDomainTemplate` не указан, системные объекты HTTPRoute/Gateway/ListenerSet будут создаваться некорректно и веб-интерфейсы служебных компонентов DKP и других модулей не будут опубликованы.
- [Проверьте совместимость](#совместное-использование-с-alb-на-основе-сторонних-решений-gateway-api) используемых версий API для объектов сторонних решений Gateway API с версиями, требуемыми для контроллера модуля `alb`. **Проверка актуальна, если в кластере используются сторонние решения Gateway API**.

## Совместное использование с другими модулями и сторонним решениями

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
- TCPRoute: v1alpha2;
- TLSRoute: v1.

Контроллер модуля `alb` в процессе запуска проверяет текущие хранимые версии объектов Gateway API. В случае обнаружения расхождения между установленными и требуемыми версиями контроллер прекращает работу. Если же в кластере полностью отсутствует тот или иной тип объекта Gateway API, нужная версия будет создана контроллером автоматически и он продолжит работу.

Для ручной проверки совместимости версий установленных в кластере объектов Gateway API с требуемыми версиями используйте команду:

```bash
declare -A want=([gatewayclasses.gateway.networking.k8s.io]=v1 [gateways.gateway.networking.k8s.io]=v1 [grpcroutes.gateway.networking.k8s.io]=v1 [httproutes.gateway.networking.k8s.io]=v1 [listenersets.gateway.networking.k8s.io]=v1 [referencegrants.gateway.networking.k8s.io]=v1beta1 [tcproutes.gateway.networking.k8s.io]=v1alpha2 [tlsroutes.gateway.networking.k8s.io]=v1 [backendtlspolicies.gateway.networking.k8s.io]=v1); for crd in "${!want[@]}"; do got="$(d8 k get crd "$crd" -o jsonpath='{.spec.versions[?(@.storage==true)].name}' 2>/dev/null || true)"; if [[ "$got" == "${want[$crd]}" ]]; then echo "$crd OK storage=$got"; else echo "$crd FAILED cluster=${got:-MISSING} expected=${want[$crd]}"; fi; done | sort
```

В остальном модуль конфигурирует и управляет только объектами Gateway определённого GatewayClass, что минимизирует риск возникновения конфликтов при использовании сторонних решений Gateway API.

## Публикация приложений

Процесс публикации приложения включает следующие шаги:

1. [Создание управляемого объекта Gateway (шлюз)](#создание-управляемого-объекта-gateway) с помощью cluster-scoped (используется ресурс [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance)) или namespaced- (используется ресурс [ALBInstance](/modules/alb/cr.html#albinstance)) кастомного ресурса.
1. [Создание объекта ListenerSet](#создание-объектов-listenerset-для-управления-приёмом-входящих-запросов), который привязывается к созданному на предыдущем шаге объекту Gateway. ListenerSet управляет приемом входящих запросов.
1. [Создание объектов (маршрутов)](#создание-маршрутов-и-настройка-маршрутизации) для маршрутизации входящих запросов к приложению и их привязка к ListenerSet. Для маршрутизации используются объекты HTTPRoute, GRPCRoute, TCPRoute и TLSRoute (нужный выбирается в зависимости от типа трафика к публикуемому приложению).

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
- Каждый объект Gateway по умолчанию создает два обработчика: `d8-http` (порт `80`) и `d8-https` (порт `443`). Они предназначены для служебных целей. Например, для проверки доступности шлюза или работы cert-manager (HTTP-01). Для публикации приложений эти обработчики использовать не рекомендуется, используйте для этого ListenerSet.
- На один объект Gateway могут ссылаться несколько объектов ClusterALBInstance или ALBInstance (через поле `gatewayName`). В этом случае они описывают общий шлюз, но инфраструктура приёма запросов может отличаться в зависимости от настроек. Можно рассматривать `gatewayName` как аналог `ingressClass` для объектов [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller).

{% alert level="warning" %}
Ручная модификация объектов Gateway, управляемых модулем, не допускается.
{% endalert %}

Пример манифеста ресурса ClusterALBInstance для создания общекластерного шлюза:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterALBInstance
metadata:
  name: public-gw
  namespace: prod
spec:
  gatewayName: public-gw
  inlet:
    type: LoadBalancer
```

Пример манифеста ресурса ALBInstance для создания отдельного шлюза для приложения или команды в выделенном неймспейсе представлен в разделе [«Использование»](../../../../../user/network/ingress/alb.html#публикация-приложения-через-объект-albinstance).

### Создание объектов ListenerSet для управления приёмом входящих запросов

Объект ListenerSet описывает системные и пользовательские обработчики трафика, которые задают имя хоста, режим TLS, порт и протокол. Каждый объект ListenerSet связывается с конкретным родительским объектом Gateway через поле `spec.parentRef`, а затем к нему подключаются маршруты.

Расположение объектов ListenerSet зависит от используемого типа объекта Gateway:

- для ClusterALBInstance объекты ListenerSet могут располагаться в любом неймспейсе;
- для ALBInstance объекты ListenerSet рекомендуется располагать в том же неймспейсе, что и родительский ALBInstance.

В обоих случаях объект ListenerSet рекомендуется располагать в том же неймспейсе, что и подключаемые к нему объекты HTTPRoute, GRPCRoute, TCPRoute и TLSRoute. Это упрощает читаемость конфигурации и позволяет избежать дополнительных настроек, например объектов ReferenceGrant.

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
- TCPRoute — для маршрутизации TCP-трафика.

Маршруты привязываются к ListenerSet.

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

   Пример манифеста cluster-scoped объект ClusterALBInstance c параметром `spec.defaultDeckhouseGateway: true`:

   ```yaml
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

## Смена инлета с сохранением текущего Gateway

Чтобы сменить инлет, используемый для уже созданного объекта Gateway, выполните следующие действия:

1. Создайте новый объект ClusterALBInstance или объект ALBInstance с другим именем, но с тем же значением `spec.gatewayName`, используя нужный тип инлета.
1. Проверьте, что новый путь приёма трафика работает корректно.
1. Удалите неактуальный объект ClusterALBInstance или объект ALBInstance.

Так как `gatewayName` не меняется, объект Gateway остаётся прежним. В большинстве случаев объект ListenerSet и маршруты при этом можно не пересоздавать.

## Открытие дополнительного TCP-порта на общекластерном Gateway

Если кроме стандартных HTTP/HTTPS-слушателей на шлюзе нужен отдельный TCP-порт, добавьте в соответствующий ClusterALBInstance поле [`spec.inlet.additionalPorts`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-additionalports) с описанием TCP-порта.

Пример:

```yaml
... 

spec:
  gatewayName: public-gw
  inlet:
    type: LoadBalancer
    loadBalancer: {}
    additionalPorts:
      - port: 9000
        protocol: TCP

...
```

Контроллер добавит на шлюз, управляемый объектом ClusterALBInstance, соответствующий обработчик TCP-трафика с именем секции (`sectionName`) вида `tcp-port-9000`. Затем можно создать объект (маршрут) TCPRoute, который будет ссылаться на этот объект Gateway и этот `sectionName` напрямую:

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
В случае создания объекта TCPRoute в неймспейсе, отличном от неймспейса целевого Gateway, дополнительно необходимо создать соответствующий ReferenceGrant объект.
{% endalert %}

Если один и тот же шлюз (Gateway) управляется несколькими объектами ClusterALBInstance, набор [`additionalPorts`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-additionalports), который попадает в объект Gateway, берётся из самого старого объекта ClusterALBInstance. Для остальных инстансов (ClusterALBInstance) в статусе может появиться признак конфликта портов.

## Просмотр конфигурации Envoy Proxy

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
