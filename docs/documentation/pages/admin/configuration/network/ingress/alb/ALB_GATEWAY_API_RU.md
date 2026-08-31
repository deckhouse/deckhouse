---
title: "ALB средствами Kubernetes Gateway API"
permalink: ru/admin/configuration/network/ingress/alb/alb-gateway-api.html
description: "Публикация приложений с помощью Kubernetes Gateway API."
lang: ru
extractedLinksMax: 4
relatedLinks:
  - title: "Миграция с ingress-nginx на alb"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/migration.html
  - title: "Публикация приложений средствами Kubernetes Gateway API"
    url: /products/kubernetes-platform/documentation/v1/user/network/ingress/alb/gateway-api.html
  - title: "Балансировка входящего трафика"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/
  - title: "Документация модуля alb"
    url: /modules/alb/
  - title: "Параметры модуля alb"
    url: /modules/alb/configuration.html
  - title: "Custom Resources модуля alb"
    url: /modules/alb/cr.html
  - title: "FAQ модуля alb"
    url: /modules/alb/faq.html
  - title: "Примеры модуля alb"
    url: /modules/alb/examples.html
  - title: "Документация модуля cert-manager"
    url: /modules/cert-manager/
---

Для реализации ALB средствами [Kubernetes Gateway API](https://kubernetes.io/docs/concepts/services-networking/gateway/) используется модуль [`alb`](/modules/alb/).

Модуль `alb` реализует прикладной балансировщик нагрузки (Application Load Balancer, ALB) и позволяет публиковать приложения с помощью Kubernetes Gateway API. Он разворачивает и настраивает инфраструктуру для приёма и маршрутизации внешних запросов, а также проверяет пользовательскую конфигурацию Gateway API.

{% alert level="info" %}
ALB средствами Kubernetes Gateway API может использоваться в кластере совместно с ALB средствами Ingress NGINX Controller.
О совместном использовании с другими модулями и сторонними решениями читайте в разделе [«Совместное использование с другими модулями и сторонними решениями»](#совместное-использование-с-другими-модулями-и-сторонними-решениями).
{% endalert %}

## Обзор и схема

Модуль построен на Kubernetes Gateway API — API маршрутизации входящего трафика, расширяющем модель Ingress API. Объект ClusterALBInstance или ALBInstance создаёт управляемый Gateway. К Gateway привязывается ListenerSet, который описывает обработчики входящих запросов. Маршруты направляют трафик к сервисам приложений.

![Схема ресурсов и прохождения трафика Gateway API](../../../../../images/network/ingress/alb/gateway-api-scheme.svg)

Модуль поддерживает:

- единый декларативный API для HTTP/HTTPS, gRPC, TCP, UDP и TLS passthrough;
- разделение ответственности между администратором кластера (ClusterALBInstance), администратором неймспейса (ALBInstance и ListenerSet — hostname, TLS, порты) и разработчиками приложения (маршруты);
- обработку запросов: WAF на уровне маршрута, внешнюю аутентификацию, списки разрешённых IP-адресов, ограничение частоты запросов, закрепление сессии (session affinity), GeoIP, BackendTLSPolicy, Proxy Protocol и HTTP/3.

Kubernetes Gateway API и API Gateway — разные понятия. Kubernetes Gateway API — это набор ресурсов Kubernetes для описания маршрутизации трафика к приложениям. API Gateway — архитектурный компонент, предоставляющий единую точку входа к API приложений. Модуль `alb` реализует Kubernetes Gateway API.

Сравнение возможностей модулей `alb` и `ingress-nginx` приведено в разделе [«Сравнение возможностей модулей ingress-nginx и alb»](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/#сравнение-возможностей-модулей-ingress-nginx-и-alb).

### Роли объектов

Gateway API разделяет ответственность между администраторами кластера и неймспейса и разработчиками приложений:

- администратор кластера — управляет инфраструктурой приёма трафика через ClusterALBInstance (общекластерный Gateway);
- администратор неймспейса — управляет ALBInstance и ListenerSet (hostname, TLS, порты) в неймспейсе;
- разработчики приложения — задают маршруты (HTTPRoute, GRPCRoute, TLSRoute, TCPRoute, UDPRoute).

### Зачем нужен ListenerSet

ListenerSet — расширение Gateway API. Объект ListenerSet описывает системные и пользовательские обработчики трафика: имя хоста, режим TLS, порт и протокол. Каждый ListenerSet связывается с родительским Gateway через `spec.parentRef`. Затем к нему подключаются маршруты.

Каждый объект Gateway по умолчанию создаёт два обработчика: `d8-http` (порт `80`) и `d8-https` (порт `443`). Они предназначены для служебных целей — например, для проверки доступности шлюза или работы `cert-manager` (HTTP-01). Для публикации приложений эти обработчики использовать не рекомендуется. Используйте ListenerSet.

### ClusterALBInstance и ALBInstance {#clusteralbinstance-and-albinstance}

При создании управляемого объекта Gateway для публикации пользовательских приложений используются кастомные ресурсы [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance) (общекластерный объект) и [ALBInstance](/modules/alb/cr.html#albinstance) (ресурс в неймспейсе).

Особенности этих ресурсов и разница между ними описаны в таблице:

| | ClusterALBInstance | ALBInstance |
| :--- | :--- | :--- |
| Назначение | Развёртывание общекластерного Gateway | Развёртывание Gateway в неймспейсе |
| Сценарии использования | - Общая точка входа (общекластерный шлюз).<br> - Системный шлюз для публикации веб-интерфейсов служебных компонентов DKP и других модулей (может требоваться [«Действия перед включением и настройкой ALB в кластере»](#действия-перед-включением-и-настройкой-alb-в-кластере)).<br> - Платформенный шлюз | Отдельный шлюз для приложения или команды в выделенном неймспейсе |
| Поддерживаемые типы инлета | [`LoadBalancer`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-loadbalancer), [`HostPort`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-hostport) | [`LoadBalancer`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-loadbalancer) |
| Реализация прокси | Envoy Proxy | Envoy Proxy |
| Тип развёртывания | DaemonSet | Deployment |
| Локализация объектов ListenerSet и маршрутов | В любом пользовательском неймспейсе | В том же неймспейсе, что и объект ALBInstance (обязательно) |
| Права доступа | Администратор кластера | Администратор неймспейса |

После создания ClusterALBInstance или ALBInstance в кластере появляется управляемый объект Gateway (шлюз). При этом:

- Каждый объект Gateway обслуживается как минимум одним экземпляром Envoy Proxy.
- Трафик в него приходит через сервис с типом `LoadBalancer` или напрямую с использованием параметров [`HostPort`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-hostport).
- На один объект Gateway могут ссылаться несколько объектов ClusterALBInstance или ALBInstance через поле `gatewayName`. В этом случае они используют общий Gateway. Конфигурация инфраструктуры для приёма и обработки трафика при этом может различаться. Поле `gatewayName` можно рассматривать как аналог `ingressClass` для объектов [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller).

### Валидация конфигурации

Помимо настройки инфраструктуры Gateway API, модуль `alb` валидирует пользовательские настройки, чтобы не допустить применения конфликтующих конфигураций. Например, он выявляет конфликты между одинаковыми обработчиками трафика в разных объектах ListenerSet, если они ссылаются на один и тот же объект Gateway.

## Инлеты

Инлет определяет, как внешний трафик попадает на Envoy Proxy, обслуживающий объект Gateway:

- [`LoadBalancer`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-loadbalancer) — приём трафика через сервис с типом `LoadBalancer` (облачные провайдеры или bare metal с MetalLB). Доступен и для ClusterALBInstance, и для ALBInstance.
- [`HostPort`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-hostport) — приём трафика на портах узлов без внешнего балансировщика. Доступен только для ClusterALBInstance.

Таблицу соответствия типов инлета при миграции с `ingress-nginx` см. в разделе [«Настройка инлета»](migration.html#inlet-configuration). Подробные примеры — в разделе [«Примеры для разных окружений»](#infrastructure-examples).

## Как настроить ALB

Публикация приложения включает включение модуля, создание управляемого Gateway, ListenerSet и маршрутов.

### Действия перед включением и настройкой ALB в кластере {#действия-перед-включением-и-настройкой-alb-в-кластере}

Модуль `alb` находится на стадии Preview и доступен начиная с Deckhouse Kubernetes Platform (DKP) 1.76. Параметры модуля — в [`configuration.html`](/modules/alb/configuration.html).

Перед включением и настройкой ALB в кластере DKP выполните следующее:

- Укажите глобальный параметр [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate), если нужно публиковать служебные домены — веб-интерфейсы [служебных компонентов DKP](/products/kubernetes-platform/documentation/v1/user/web/ui.html) и других модулей. Без этого параметра системные объекты HTTPRoute, Gateway и ListenerSet создаются некорректно, и веб-интерфейсы не публикуются. Подробности — в разделе [«Публикация служебных доменов»](#публикация-служебных-доменов).
- Проверьте совместимость версий API в разделе [«Совместно со сторонними реализациями Gateway API»](#alongside-third-party-gateway-api), если такие решения уже используются в кластере.
- На bare metal для инлета [`LoadBalancer`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-loadbalancer) подготовьте внешний балансировщик или модуль [`metallb`](/modules/metallb/). Инлет [`HostPort`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-hostport) доступен только для ClusterALBInstance и не требует MetalLB.

### Включение модуля и создание Gateway {#создание-управляемого-объекта-gateway}

Включите модуль `alb`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: alb
spec:
  enabled: true
```

Для создания управляемого объекта Gateway используйте ресурс ClusterALBInstance или ALBInstance.

{% alert level="warning" %}
Ручная модификация объектов Gateway, управляемых модулем, не допускается.
{% endalert %}

{% tabs Примеры ресурсов Gateway %}
{% tab "ClusterALBInstance" %}

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

{% endtab %}
{% tab "ALBInstance" %}

Пример минимальной рабочей конфигурации ALBInstance для публикации шлюза в неймспейсе:

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

После перехода ALBInstance в состояние `Ready` создайте в том же неймспейсе объекты ListenerSet и HTTPRoute. Порядок их настройки и пример публикации приложения приведены в разделе [«Публикация приложения с ListenerSet и HTTPRoute»](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/gateway-api.html#publishing-with-listenerset-and-httproute).

{% endtab %}
{% endtabs %}

### Создание ListenerSet

Объект ListenerSet описывает системные и пользовательские обработчики трафика: имя хоста, режим TLS, порт и протокол. Каждый ListenerSet связывается с родительским Gateway через `spec.parentRef`. Затем к нему подключаются маршруты.

Расположение объектов ListenerSet зависит от используемого типа объекта Gateway:

- для ClusterALBInstance объекты ListenerSet могут располагаться в любом неймспейсе;
- для ALBInstance объекты ListenerSet должны располагаться в том же неймспейсе, что и родительский ALBInstance.

В обоих случаях рекомендуется размещать объект ListenerSet в том же неймспейсе, что и связанные с ним объекты HTTPRoute, GRPCRoute и TLSRoute. Тогда не потребуются дополнительные настройки, например создание объектов ReferenceGrant.

В ListenerSet для HTTP/HTTPS указывайте порты `80` и `443`. Это порты слушателей Gateway API. Они не совпадают с параметрами [`hostPort.httpPort`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-hostport-httpport) / [`hostPort.httpsPort`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-hostport-httpsport) инлета HostPort, которые задают порты на узле.

Объекты TCPRoute и UDPRoute, использующие TCP- и UDP-порты из [`additionalPorts`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-additionalports), привязываются непосредственно к соответствующему слушателю объекта Gateway.

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
          - name: app-tls   # Наименование секрета, содержащего необходимый TLS-сертификат.
            namespace: prod

```

### Создание маршрутов

Для маршрутизации входящих запросов используются следующие типы маршрутов:

- HTTPRoute — для маршрутизации HTTP/HTTPS/TLS запросов. Объекты HTTPRoute поддерживают расширенные настройки с помощью аннотаций, которые дополняют текущую спецификацию Gateway API.
- GRPCRoute — для маршрутизации gRPC-трафика.
- TLSRoute — для сквозной маршрутизации TLS-трафика.
- TCPRoute — для маршрутизации TCP-трафика. Для TCP-портов из [`additionalPorts`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-additionalports) объект TCPRoute привязывается напрямую к слушателю Gateway, а не к ListenerSet.
- UDPRoute — для маршрутизации UDP-трафика. Для UDP-портов из [`additionalPorts`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-additionalports) объект UDPRoute привязывается напрямую к слушателю Gateway, а не к ListenerSet.

Объекты HTTPRoute, GRPCRoute и TLSRoute привязываются к ListenerSet. TCPRoute и UDPRoute для TCP/UDP-портов из [`additionalPorts`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-additionalports) привязываются напрямую к слушателю объекта Gateway.

{% tabs Примеры HTTPRoute %}
{% tab "HTTP" %}

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

{% endtab %}
{% tab "HTTPS" %}

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

{% endtab %}
{% endtabs %}

Дополнительные сценарии публикации — в разделе [«Публикация приложения с ListenerSet и HTTPRoute»](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/gateway-api.html#publishing-with-listenerset-and-httproute).

## Публикация служебных доменов {#публикация-служебных-доменов}

{% alert level="warning" %}
Если нужно публиковать служебные домены, убедитесь, что указан глобальный параметр [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate). Если параметр `publicDomainTemplate` не указан, системные объекты HTTPRoute/Gateway/ListenerSet будут создаваться некорректно и веб-интерфейсы служебных компонентов DKP и других модулей не будут опубликованы.
{% endalert %}

Для предоставления доступа к служебным доменам кластера DKP укажите шлюз по умолчанию. Создайте ClusterALBInstance с нужным типом инлета и [настройками](/modules/alb/cr.html#clusteralbinstance) и установите для него параметр [`spec.defaultDeckhouseGateway: true`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-defaultdeckhousegateway).

Пример манифеста ClusterALBInstance с параметром `spec.defaultDeckhouseGateway: true`:

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

После применения изменений проверьте статус ClusterALBInstance:

```bash
d8 k get clusteralbinstances
```

ClusterALBInstance должен перейти в состояние `Ready` и создать управляемый Gateway. После этого в соответствующих системных неймспейсах появятся ListenerSet и HTTPRoute.

### Алгоритм выбора шлюза DKP по умолчанию при использовании нескольких ClusterALBInstance

В кластере может быть одновременно несколько общекластерных Gateway, помеченных как шлюз по умолчанию (флаг [`spec.defaultDeckhouseGateway: true`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-defaultdeckhousegateway) в параметрах соответствующих ClusterALBInstance). В этом случае шлюзом по умолчанию становится Gateway, созданный объектом ClusterALBInstance с наиболее ранним `creationTimestamp` (то есть созданный раньше остальных).

Если ни один объект ClusterALBInstance не отмечен как шлюз по умолчанию, DKP допускает использование объекта Gateway, созданного модулем `alb` для инстанса ClusterALBInstance с именем `main`.

### Смена шлюза DKP по умолчанию

Если системные домены DKP необходимо перевести на обслуживание другим объектом Gateway, выполните следующие шаги:

1. Создайте новый объект ClusterALBInstance, описывающий необходимые настройки, и задайте в нём параметр [`spec.defaultDeckhouseGateway: true`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-defaultdeckhousegateway).
1. В текущем объекте ClusterALBInstance, который предоставляет шлюз DKP по умолчанию, задайте `spec.defaultDeckhouseGateway: false`.
1. Проверьте, что все системные объекты ListenerSet теперь ссылаются на новый объект Gateway в `spec.parentRef`.

## Примеры для разных окружений {#infrastructure-examples}

В этом разделе описаны приём трафика в разных окружениях и операции с уже развёрнутым ALB. Полное описание полей кастомных ресурсов приведено в документации [`ClusterALBInstance`](/modules/alb/cr.html#clusteralbinstance) и [`ALBInstance`](/modules/alb/cr.html#albinstance). Дополнительные примеры — в [`примерах модуля alb`](/modules/alb/examples.html).

{% tabs Приём трафика %}
{% tab "Облачный провайдер" %}

### Облачный провайдер (инлет LoadBalancer) {#cloud-load-balancer}

Пример ClusterALBInstance с инлетом [`LoadBalancer`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-loadbalancer):

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

Чтобы настроить облачный балансировщик, укажите необходимые аннотации для создаваемого объекта Service типа LoadBalancer в параметре [`spec.inlet.loadBalancer.serviceAnnotations`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-loadbalancer-serviceannotations).
Пример использования Network Load Balancer (NLB) в AWS:

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

{% endtab %}
{% tab "Bare metal с MetalLB" %}

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

{% endtab %}
{% tab "Bare metal с HostPort" %}

### Bare metal без внешнего балансировщика (инлет HostPort) {#bare-metal-hostport}

Пример ClusterALBInstance с инлетом [`HostPort`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-hostport):

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

{% endtab %}
{% tab "Внешний L7-балансировщик" %}

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

{% endtab %}
{% endtabs %}

### Смена инлета с сохранением текущего Gateway {#change-inlet}

Чтобы сменить инлет, используемый для уже созданного объекта Gateway, выполните следующие действия:

1. Создайте новый объект ClusterALBInstance или объект ALBInstance с другим именем, но с тем же значением `spec.gatewayName`, используя нужный тип инлета.
1. Проверьте, что новый путь приёма трафика работает корректно.
1. Удалите неактуальный объект ClusterALBInstance или объект ALBInstance.

Так как `gatewayName` не меняется, объект Gateway остаётся прежним. В большинстве случаев объект ListenerSet и маршруты при этом можно не пересоздавать.

### Открытие дополнительного TCP/UDP-порта на общекластерном Gateway {#tcp-port}

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

Контроллер добавит в управляемый объект Gateway слушатель для TCP/UDP-трафика с именем секции (`sectionName`), например `tcp-port-9000`. После этого можно создать объект TCPRoute и напрямую привязать его к этому слушателю, указав имя Gateway и соответствующий `sectionName`:

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

Примеры UDPRoute и шаги публикации приложений — в разделе [«Работа с объектами GRPCRoute, TLSRoute, TCPRoute и UDPRoute»](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/gateway-api.html#grpcroute-tlsroute-tcproute-and-udproute-objects).

### Конфликты портов при использовании нескольких ClusterALBInstance/ALBInstance для одного Gateway {#conflicts}

Если один и тот же объект Gateway обслуживается несколькими объектами ClusterALBInstance или ALBInstance, итоговый набор слушателей, который попадает в объект Gateway, берётся из инстанса ClusterALBInstance или ALBInstance с наиболее ранним `creationTimestamp` (то есть созданного раньше остальных). Для остальных инстансов в статусе появляется признак конфликта портов с указанием имени «управляющего» инстанса.

### Разделение публичной и административной зон {#public-and-admin-zones}

Можно развернуть отдельные объекты Gateway для публичного и административного трафика, чтобы у каждой зоны была своя точка входа и политика доступа. Это похоже на выделение отдельного Ingress NGINX Controller (и IngressClass) под административную зону.

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

Далее для каждого шлюза создайте отдельные объекты ListenerSet и маршруты. Примеры публикации приложений приведены в разделе [«Публикация приложения с ListenerSet и HTTPRoute»](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/gateway-api.html#publishing-with-listenerset-and-httproute).

### Публикация в неймспейсе (ALBInstance) {#namespaced-load-balancer}

Пример минимальной рабочей конфигурации ALBInstance для публикации шлюза в неймспейсе:

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

После перехода ALBInstance в состояние `Ready` создайте в том же неймспейсе объекты ListenerSet и HTTPRoute. Порядок их настройки и пример публикации приложения приведены в разделе [«Публикация приложения с ListenerSet и HTTPRoute»](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/gateway-api.html#publishing-with-listenerset-and-httproute).

### Выпуск TLS-сертификатов с cert-manager {#tls-cert-manager}

Модуль `alb` совместим с [`cert-manager`](/modules/cert-manager/). Слушатели `d8-http` / `d8-https` используются для HTTP-01 challenge. Для приложений выпускайте сертификат в Secret и ссылайтесь на него из `certificateRefs` в ListenerSet.

Минимальный пример Certificate, который создаёт Secret `app-tls` для ListenerSet:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: app-tls
  namespace: prod
spec:
  secretName: app-tls
  issuerRef:
    name: letsencrypt
    kind: ClusterIssuer
  dnsNames:
    - app.example.com
```

О выпуске сертификатов читайте в [«Документация модуля cert-manager»](/modules/cert-manager/). Для совместной работы с `ingress-nginx` используйте отдельный ClusterIssuer для каждого типа ALB.

## Совместное использование с другими модулями и сторонними решениями {#совместное-использование-с-другими-модулями-и-сторонними-решениями}

ALB средствами Kubernetes Gateway API в кластере DKP можно использовать совместно с ALB средствами Ingress NGINX Controller, а также с ALB на основе сторонних решений Gateway API. Пошаговый переход — в разделе [«Миграция с ingress-nginx на alb»](migration.html).

### Совместно с ingress-nginx {#alongside-ingress-nginx}

ALB средствами Kubernetes Gateway API может использоваться в кластере совместно с [«ALB средствами Ingress NGINX Controller»](nginx.html). В таком случае для каждого из типов ALB рекомендуется использовать отдельный объект ClusterIssuer, чтобы раздельно управлять настройками и жизненными циклами сертификатов. Один и тот же внешний hostname не должен одновременно обслуживаться обоими ALB без разделения на уровне DNS или внешнего балансировщика.

{% alert level="info" %}
Для шлюза DKP по умолчанию объект ClusterIssuer создаётся автоматически. Этот же объект ClusterIssuer используется для выпуска сертификатов системных доменов.
{% endalert %}

### Совместно со сторонними реализациями Gateway API {#alongside-third-party-gateway-api}

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
        d8 k get crd "$crd" \
          -o jsonpath='{.spec.versions[?(@.storage==true)].name}' \
          2>/dev/null || true
    )"
    if [[ -n "$got" && "$got" =~ ^(${want[$crd]})$ ]]; then
        echo "$crd OK storage=$got"
    else
        echo "$crd FAILED cluster=${got:-MISSING} expected=${want[$crd]}"
    fi
done | sort
```

В остальном модуль конфигурирует и управляет только объектами Gateway определённого GatewayClass, что минимизирует риск возникновения конфликтов при использовании сторонних решений Gateway API.

## Диагностика и проверка {#verification-and-common-questions}

В этом разделе — проверка готовности шлюза и типичные вопросы при первой настройке. Дополнительные сценарии — в [«FAQ модуля alb»](/modules/alb/faq.html).

- `Ready` и `status` — после создания ClusterALBInstance или ALBInstance дождитесь состояния `Ready` и возьмите имя и неймспейс Gateway из `status`. Описание полей — в [ClusterALBInstance status](/modules/alb/cr.html#clusteralbinstance-v1alpha1-status) и [ALBInstance status](/modules/alb/cr.html#albinstance-v1alpha1-status):

  ```bash
  d8 k get clusteralbinstance
  d8 k -n <NAMESPACE> get albinstance
  d8 k -n d8-alb get gateway
  ```

- Адрес балансировщика и DNS — при инлете LoadBalancer возьмите адрес у сервиса балансировщика (обычно в неймспейсе `d8-alb`) и укажите его в DNS для hostname из ListenerSet:

  ```bash
  d8 k -n d8-alb get svc
  ```

- Один hostname для двух ALB — модули `alb` и `ingress-nginx` могут работать в одном кластере, но один и тот же внешний hostname не должен одновременно обслуживаться двумя ALB без явного разделения на уровне DNS или внешнего балансировщика.

- Конфликт ListenerSet — если два ListenerSet с одинаковыми обработчиками ссылаются на один Gateway, контроллер отклонит конфликтующую конфигурацию. Измените hostname, порт или протокол либо удалите дублирующий ListenerSet.

### Просмотр конфигурации Envoy Proxy {#envoy-config}

Для диагностики можно просмотреть конфигурацию, переданную контроллером и конфигуратором прокси в Envoy Proxy, обслуживающий объект Gateway.

1. Выберите под Envoy Proxy для нужного объекта Gateway:

   ```bash
   d8 k -n d8-alb get pods -l alb.deckhouse.io/gateway=shared-gateway
   ```

1. Получите конфигурацию пода с помощью команды (вместо `<ENVOY_PROXY_POD_NAME>` используйте имя пода Envoy Proxy, полученное на предыдущем шаге):

   ```bash
   d8 k -n d8-alb exec -it <ENVOY_PROXY_POD_NAME> pilot-agent request GET /config_dump
   ```

   Если нужен только отдельный раздел конфигурации, явно укажите его:

   ```bash
   d8 k -n d8-alb exec -it <ENVOY_PROXY_POD_NAME> pilot-agent request GET /config_dump?resource=dynamic_listeners
   d8 k -n d8-alb exec -it <ENVOY_PROXY_POD_NAME> pilot-agent request GET /config_dump?resource=dynamic_route_configs
   d8 k -n d8-alb exec -it <ENVOY_PROXY_POD_NAME> pilot-agent request GET /config_dump?resource=dynamic_active_clusters
   ```

Так можно проверить, появились ли ожидаемые обработчики трафика, виртуальные хосты и upstream-кластеры после изменения объекта ListenerSet или объекта Route.
