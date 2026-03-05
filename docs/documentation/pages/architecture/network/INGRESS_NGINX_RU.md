---
title: Модуль ingress-nginx
permalink: ru/architecture/network/ingress-nginx.html
lang: ru
search: ingress-nginx, ingress, ingress controller, ingress контроллер, nginx, istio
description: Архитектура модуля ingress-nginx в Deckhouse Kubernetes Platform.
---

Модуль `ingress-nginx` устанавливает и управляет [Ingress NGINX Controller](https://kubernetes.github.io/ingress-nginx/) с помощью кастомного ресурса IngressNginxController.

Модуль может работать в режиме высокой доступности (HA) и предоставляет гибкие настройки размещения Ingress-контроллеров на узлах кластера, а также параметры работы контроллера с учетом особенностей реализации инфраструктуры.

Модуль поддерживает запуск и раздельную конфигурацию нескольких экземпляров Ingress NGINX Controller. Это позволяет, например, разделять внешние и внутренние (intranet) Ingress-ресурсы приложений.

Подробнее с настройками модуля и примерами его использования можно ознакомиться в [соответствующем разделе документации](/modules/ingress-nginx/configuration.html).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`ingress-nginx`](/modules/ingress-nginx/) на уровне 2 модели C4 и его взаимодействия с другими компонентами Deckhouse Kubernetes Platform (DKP) изображены на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4 --->
![Архитектура модуля ingress-nginx](../../../images/architecture/network/c4-l2-ingress-nginx.png)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Controller-nginx** ([Advanced DaemonSet](https://openkruise.io/docs/user-manuals/advanceddaemonset)) — нестандартный DaemonSet с продвинутыми возможностями, управляемый kruise-controller-manager.

   Состоит из следующих контейнеров:

   * **controller** — основной контейнер IngressNGINX controller, реализующий основную логику модуля. Является [Open Source-проектом](https://kubernetes.github.io/ingress-nginx/);
   * **protobuf-exporter** — сайдкар-контейнер в поде ingress-controller, принимающий статистику NGINX в виде сообщений в формате protobuf. Разбирает и агрегирует сообщения по установленным правилам, а также экспортирует метрики в формате Prometheus. Является разработкой компании «Флант»;
   * **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищенного доступа к метрикам и состоянию контроллера и `protobuf-exporter`. Является [Open Source-проектом](https://github.com/brancz/kube-rbac-proxy);
   * **istio-proxy** — сайдкар-контейнер Istio, добавляемый в под при включенном параметре [`spec.enableIstioSidecar`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-enableistiosidecar) кастомного ресурса IngressNginxController. В этом случае часть пользовательских запросов проходит через него.

2. **Validator-nginx** (Deployment) — состоит из одного контейнера. Validator — это Ingress NGINX Controller, запущенный в режиме валидации и обладающий ограниченным набором привилегий. Реализует вебхук-сервер, используемый для проверки Ingress-ресурсов через механику [Validating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/).

3. **Kruise-controller-manager** (Deployment) — контроллер, управляющий кастомным ресурсом [Advanced DaemonSet](https://openkruise.io/docs/user-manuals/advanceddaemonset). Данное расширение DaemonSet позволяет использовать продвинутые возможности при обновлении Ingress NGINX controller, отсутствующие в стандартной реализации DaemonSet-контроллера Kubernetes.

   Состоит из следующих контейнеров:

   * **kruise** — основной контейнер kruise-controller-manager;
   * **kruise-state-metrics** — сайдкар-контейнер, отслеживающий состояние объектов API OpenKruise и предоставляющий соответствующие метрики (но не метрики работы самого kruise-controller-manager);
   * **kube-rbac-proxy** — сайдкар-контейнер, обеспечивающий авторизованный доступа к метрикам и состоянию контроллера. Подробно описан выше.

4. **Failover-cleaner** (DaemonSet) — развертывается на узлах кластера, на которых установлен лейбл `ingress-nginx-controller.deckhouse.io/need-hostwithfailover-cleanup=true`. Представляет собой bash-скрипт, который актуализирует правила iptables в зависимости от используемого инлета контроллера. При штатной работе ingress-controller компонент failover-cleaner не запущен ни на одном узле.

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

   * синхронизация конфигурации NGINX при изменении Ingress-ресурсов;
   * авторизация запросов на получение метрик, статистики и проверки состояния контроллера;
   * перенаправление внешних HTTP-запросов на эндпоинт API Kubernetes.

2. **Dex-authenticator служебных сервисов и пользовательских приложений** — используется для аутентификации запросов в dex через dex-authenticator, которые выполняют функции OAuth2 Proxy.

3. **Служебные сервисы DKP** (Console, Dashboard, Grafana и прочие) — модуль перенаправляет HTTP-запросы, прошедшие аутентификацию через Dex.

4. **Пользовательские сервисы, развернутые в DKP** — модуль перенаправляет на них внешние HTTP-запросы. Для этого пользователь должен создать соответствующие Ingress-ресурсы, а также кастомный ресурс [DexAuthenticator](/modules/user-authn/cr.html#dexauthenticator), если требуется аутентификация через Dex.

{% alert level="info" %}
Для упрощения схемы на ней изображены взаимодействия ingress-controller только c одним служебным сервисом DKP — компонентом frontend модуля `console` и соответствующим console-dex-authenticator.
{% endalert %}

С модулем взаимодействуют следующие внешние компоненты:

1. **Kube-apiserver** — использует validation-вебхук для проверки создаваемых или обновляемых [Ingress-ресурсов](https://kubernetes.io/docs/concepts/services-networking/ingress/).
2. **Prometheus-main** — собирает метрики контроллеров ingress и kruise, а также статистику NGINX.
3. **Балансировщик нагрузки** — балансировка HTTP/HTTPS-трафика между работоспособными экземплярами ingress-controller.

## Способы приема трафика из внешней сети

Способы приема трафика из внешней сети подробно описаны в параметре [`spec.inlet`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-inlet) кастомного ресурса IngressNginxController.

Для инлетов вида LoadBalancer, LoadBalancerWithProxyProtocol и LoadBalancerWithSSLPassthrough указанный на схеме балансировщик нагрузки автоматически предоставляется облачным провайдером (при развертывании DKP в облаке), либо может быть реализован при помощи MetalLB-контроллера (при установке на bare-metal-хостах). С настройками модуля `metallb` можно ознакомиться в [соответствующем разделе документации](/modules/metallb/configuration.html).

Для инлетов вида HostPort, HostPortWithProxyProtocol, HostPortWithSSLPassthrough и HostWithFailover балансировщик нагрузки разворачивается пользователем, либо может отсутствовать. В этом случае пользователь самостоятельно настраивает бэкенды балансировщика или обеспечивает сетевую связность до ingress-controller. Точкой входа в ingress-controller в этом случае являются порты на узлах кластера, на которых запущен контроллер.
