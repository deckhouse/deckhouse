---
title: Модуль alb
permalink: ru/architecture/network/alb.html
lang: ru
search: alb, application load balancer, gateway api
description: Архитектура модуля alb в Deckhouse Kubernetes Platform.
---

Модуль [`alb`](/modules/alb/) реализует прикладной балансировщик нагрузки (ALB, Application Load Balancer) и позволяет публиковать приложения с помощью [Kubernetes Gateway API](https://gateway-api.sigs.k8s.io/). Он разворачивает и настраивает инфраструктуру для приёма и маршрутизации внешних запросов, а также проверяет пользовательскую конфигурацию Gateway API.

Модуль работает со следующими кастомными ресурсами API-группы `network.deckhouse.io`:

- [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance) — определяет экземпляр ALB (data-plane) в масштабе всего кластера, используется для общих или платформенных Gateway;
- [ALBInstance](/modules/alb/cr.html#albinstance) — определяет экземпляр ALB в рамках пользовательского неймспейса, используется для Gateway уровня приложения или проекта.

Подробнее с описанием настройки и примерами использования модуля можно ознакомиться [в разделе документации модуля](/modules/alb/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`alb`](/modules/alb/) на уровне 2 модели C4 и его взаимодействие с другими компонентами Deckhouse Kubernetes Platform (DKP) изображены на следующей диаграмме:

![Архитектура модуля alb](../../images/architecture/network/c4-l2-alb.ru.svg)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Proxy-configurator** (Deployment) — управляющий компонент Gateway API, собранный на основе Istio Pilot (istiod) только для работы с Gateway API (инжект сайдкаров и штатный механизм создания инфраструктуры Gateway API у istiod выключены).

   Компонент отдаёт конфигурацию для Envoy-прокси по протоколу xDS и выпускает для них сертификаты через встроенный CA-сервер. Также валидирует и обновляет статусы ресурсов Gateway, HTTPRoute, GRPCRoute, TCPRoute, UDPRoute, TLSRoute и ListenerSet.

   Состоит из одного контейнера **proxy-configurator**.

1. **Gateway-controller** (Deployment) — центральный контроллер модуля, который выполняет следующие действия:

   - управляет кастомными ресурсами ClusterALBInstance и ALBInstance;
   - устанавливает CRD ресурсов Gateway API `*.gateway.networking.k8s.io`;
   - реализует контроллер Gateway API для GatewayClass `d8-alb` и создаёт Gateway для каждого инстанса;
   - обслуживает admission-вебхуки для их валидации;
   - создаёт временные объекты Ingress для HTTP-01 challenge cert-manager поверх HTTPRoute при миграции с [`ingress-nginx`](/modules/ingress-nginx/);
   - создаёт и удаляет компоненты proxy и geoproxy на каждый ClusterALBInstance или ALBInstance.

   Состоит из следующих контейнеров:

   - **gateway-controller** — основной контейнер;
   - **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищённого доступа к метрикам и graph API gateway-controller.

1. **Proxy** (Deployment или DaemonSet) — экземпляр Envoy data-plane, принимает и маршрутизирует внешний трафик по конфигурации, полученной от proxy-configurator по протоколу xDS.

   Gateway-controller создаёт этот компонент динамически (не Helm-шаблоном) на каждый кастомный ресурс ClusterALBInstance или ALBInstance. Тип рабочей нагрузки зависит от вида ресурса: ClusterALBInstance разворачивается как DaemonSet, ALBInstance — как Deployment за Service.

   Состоит из следующих контейнеров:

   - **geo-downloader-init** — опциональный init-контейнер, скачивает базу GeoIP перед стартом Envoy;
   - **geo-downloader** — опциональный сайдкар-контейнер, периодически обновляет локальную копию базы GeoIP;
   - **proxy** — основной контейнер;
   - **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищённого доступа к метрикам основного контейнера.

   Gateway-controller добавляет два первых контейнера, только если у ресурса ClusterALBInstance или ALBInstance задан параметр [`spec.geoIP`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-geoip).

1. **Geoproxy** (StatefulSet) — кеширующий прокси-сервер для компонента proxy, который предоставляет быстрый доступ к базе данных GeoIP, загружаемой у провайдера MaxMind. Компонент также обеспечивает доступ к уже загруженным базам в кластерах без выхода в интернет, что повышает стабильность работы контроллера Gateway API с GeoIP-базами.

   Компонент предоставляет следующие возможности:

   * экономия лицензий MaxMind (загрузка баз происходит из одной точки раз в сутки);
   * постоянное хранение данных (перезагрузка компонентов не приводит к повторным обращениям к серверам MaxMind);
   * возможность указать своё зеркало для загрузки баз.

   Gateway-controller создаёт этот компонент программно на каждый ClusterALBInstance и ALBInstance, только если у ресурса задан параметр [`spec.geoIP.licenseKeySecretRef`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-geoip-licensekeysecretref), содержащий ссылку на Secret с лицензионным ключом MaxMind.

   Состоит из одного контейнера **geoproxy**.

1. **Module-cleanup-waiter** (Job) — Helm `pre-delete` хук модуля, запускаемый контроллером Deckhouse перед удалением модуля [`alb`](/modules/alb/), который ожидает, пока gateway-controller завершит очистку ресурсов модуля, и только после этого позволяет закончить удаление релиза. Не участвует в штатной работе модуля.

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

   - устанавливает ресурсы API-группы `*.gateway.networking.k8s.io`;
   - управляет DaemonSet, Deployment, StatefulSet, Service, Secret и ConfigMap;
   - получает и обновляет Node, Namespace;
   - управляет кастомными ресурсами ClusterALBInstance и ALBInstance, Ingress и ресурсами Gateway API (API-группы `*.gateway.networking.k8s.io`);
   - авторизует запросы на получение метрик и graph API gateway-controller, а также метрик proxy.

1. **Источник данных GeoIP** (провайдер MaxMind или зеркало) — скачивает базу данных GeoIP.

С модулем взаимодействуют следующие внешние компоненты:

1. **Kube-apiserver** — вызывает валидацию кастомных ресурсов ClusterALBInstance, ALBInstance и ресурсов Gateway API (API-группа `*.gateway.networking.k8s.io`).

1. **Prometheus-main**:

   - собирает метрики gateway-controller;
   - собирает метрики proxy.

1. **Балансировщик нагрузки** — балансировка HTTP/HTTPS-трафика между экземплярами компонента proxy.

1. **[Веб-интерфейс Deckhouse](/modules/console/)** — запрашивает граф связей ресурсов Gateway API для визуализации.
