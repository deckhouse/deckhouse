---
title: Модуль istio
permalink: ru/architecture/network/istio.html
lang: ru
search: istio, service mesh, ambient, federation, multicluster, sidecar
description: Архитектура модуля istio в Deckhouse Kubernetes Platform.
---

Модуль [`istio`](/modules/istio/) реализует Service Mesh (сервис-меш) на основе [Istio](https://istio.io/) для централизованного управления сетевым трафиком в Deckhouse Kubernetes Platform (DKP). Модуль обеспечивает mTLS (Mutual Transport Layer Security), авторизацию запросов, маршрутизацию трафика, балансировку нагрузки и наблюдаемость взаимодействий между приложениями.

Модуль [`istio`](/modules/istio/) предоставляет возможность одновременной работы нескольких версий Istio. В параметре модуля [`globalVersion`](/modules/istio/configuration.html#parameters-globalversion) указывается какая версия Istio будет использоваться по-умолчанию для тех неймспейсов, у которых установлен лейбл `istio-injection: enabled`. В случае, если требуется использовать версию Istio, отличную от версии по-умолчанию, для неймспейсов устанавливается лейбл, соответствующий ревизии Istio, например, `istio.io/rev: v1x27`.

Модуль работает со следующими кастомными ресурсами API-группы `deckhouse.io`:

- [IngressIstioController](/modules/istio/cr.html#ingressistiocontroller) — описывает инстанс Istio ingress gateway, обслуживающий выбранный класс шлюза;
- [IstioFederation](/modules/istio/cr.html#istiofederation) — назначает удалённый кластер доверенным для федерации сервис-меш (DKP EE);
- [IstioMulticluster](/modules/istio/cr.html#istiomulticluster) — назначает удалённый кластер доверенным для multicluster-конфигурации (DKP EE);
- [WaypointInstance](/modules/istio/cr.html#waypointinstance) — описывает ambient-прокси waypoint, создаваемый компонентом waypoint-controller (DKP EE).

Модуль также устанавливает и использует кастомные ресурсы [Istio](https://istio.io/) (API-группы `networking.istio.io`, `security.istio.io`, `telemetry.istio.io`, `extensions.istio.io`). Подробнее можно ознакомиться [в справочнике кастомных ресурсов Istio](/modules/istio/istio-cr.html).

{% alert level="warning" %}
В DKP поддерживается только одна версия Istio 1.25 с поддержкой оператора. Все последующие версии работают без оператора.
Версия Istio 1.25 является устаревшей и будет удалена в будущих обновлениях.
{% endalert %}

Если запрошена установка версии Istio с поддержкой оператора, то устанавливаются и используются кастомные ресурсы [Sail Operator](https://github.com/istio-ecosystem/sail-operator) API-группы `sailoperator.io`:

- Istio — описывает развёртывание сервис-меш Istio, состоящее из одного или нескольких control plane;
- IstioRevision — представляет одну ревизию control plane Istio.

Набор компонентов модуля и его архитектура зависит от редакции DKP. Редакция DKP Enterprise Edition (EE) включает возможность реализовать межкластерную федерацию сервис-меш, периодический анализ конфигурации сервис-меш и возможность использования [ambient-режима Istio](https://istio.io/latest/docs/ambient/overview/).

Подробнее с настройками модуля можно ознакомиться [в разделе документации модуля](/modules/istio/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`istio`](/modules/istio/) на уровне 2 модели C4 и его взаимодействие с другими компонентами Deckhouse Kubernetes Platform (DKP) изображены на следующих диаграммах:

- Базовая функциональность модуля (control plane, CNI, ingress gateway, Kiali, config-analyzer):

  ![Архитектура модуля istio](../../images/architecture/network/c4-l2-istio.ru.svg)

- Ambient-режим (на диаграмме отражены только отличия от базового варианта):

  ![Архитектура модуля istio в ambient-режиме](../../images/architecture/network/c4-l2-istio-ambient.ru.svg)

- Федерация и multicluster-конфигурация (на диаграмме отражены только отличия от базового варианта):

  ![Архитектура модуля istio в конфигурации federation/multicluster](../../images/architecture/network/c4-l2-istio-multicluster.ru.svg)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Operator-\<VERSION>** (Deployment) — реализация [Sail Operator](https://github.com/istio-ecosystem/sail-operator), управляющий жизненным циклом control plane Istio. Компонент отвечает за установку всех ресурсов, необходимых для работы control plane определенной версии.

   В DKP поддерживается работа оператора только для версии Istio 1.25.

   Компонент отслеживает кастомные ресурсы Istio и IstioRevision и на их основе создаёт Deployment `istiod-<VERSION>`, Service и ConfigMap. Управление вебхуками для валидации и мутации принудительно отключено в операторе, этим занимается контроллер Deckhouse модуля [`deckhouse`](/modules/deckhouse/) при применении Helm-чарта модуля.

   Состоит из одного контейнера:

   - **operator** — основной контейнер.

1. **Istiod-\<VERSION>** (Deployment) — компонент control plane Istio, выполняющий следующие действия:

   - распространяет конфигурацию маршрутизации сайдкар-прокси по протоколу [xDS](https://github.com/cncf/xds);
   - выпускает сертификаты для рабочих нагрузок под управлением Istio;
   - выполняет валидацию и мутацию подов пользовательских приложений сайдкар-контейнерами через механику [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/);
   - выполняет валидацию кастомных ресурсов API-групп `*.istio.io` через механику Validating Admission Controllers.

   Для версии Istio 1.25 (устарела и запланирована к удалению) компонент создаётся и управляется компонентом operator-\<VERSION> через кастомные ресурсы Istio и IstioRevision. Для остальных [поддерживаемых в DKP версий Istio](/modules/istio/#таблица-совместимости-поддерживаемых-версий) разворачивается напрямую Helm-чартом модуля.

   Состоит из одного контейнера:

   - **discovery** — основной контейнер.

1. **Ingress-gateway-controller-\<NAME>** (DaemonSet) — контроллер, обрабатывающий входящий меш-трафик приложений. Создаётся контроллером Deckhouse модуля [`deckhouse`](/modules/deckhouse/) для каждого кастомного ресурса [IngressIstioController](/modules/istio/cr.html#ingressistiocontroller). Плейсхолдер `<NAME>` определяется именем ресурса IngressIstioController.

   Состоит из одного контейнера:

   - **istio-proxy** — основной контейнер на основе Open Source-проекта [Envoy](https://github.com/envoyproxy/envoy), обрабатывающий входящий трафик и получающий конфигурацию по протоколу xDS от контроллера istiod.

1. **Kiali** (Deployment) — веб-интерфейс [Kiali](https://kiali.io/) для управления и наблюдения за ресурсами Istio и пользовательскими сервисами под управлением Istio, позволяющая следующее:

   - визуализировать связи между сервисами;
   - диагностировать проблемные связи между сервисами;
   - диагностировать состояние Istio control plane.

   Состоит из следующих контейнеров:

   - **kiali** — основной контейнер;
   - **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищённого доступа к веб-интерфейсу Kiali.

   Аутентификация пользователей веб-интерфейса Kiali выполняется модулем [`user-authn`](/modules/user-authn/) через отдельный dex-authenticator.

1. **Istio-config-analyzer-\<VERSION>** (Deployment) — компонент, выполняющий периодический анализ конфигурации сервис-меш (`istioctl analyze`) и экспортирующий результаты анализа в виде метрик Prometheus.

   Компонент создаётся контроллером Deckhouse модуля [`deckhouse`](/modules/deckhouse/) для каждой ревизии Istio, если параметр [`.settings.configAnalysis.enabled`](/modules/istio/configuration.html#parameters-configanalysis-enabled) в настройках модуля принимает значение `true` (по умолчанию — `true`).

   Состоит из следующих контейнеров:

   - **istio-config-analyzer** — основной контейнер;
   - **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищённого доступа к метрикам.

1. **Istio-cni-node** (DaemonSet) — компонент CNI-плагина Istio, настраивающий перехват трафика подов на каждом узле кластера без настройки iptables-правил через init-контейнер в каждом поде.

   Компонент создаётся контроллером Deckhouse, если параметр [`.settings.dataPlane.trafficRedirectionSetupMode`](/modules/istio/configuration.html#parameters-dataplane-trafficredirectionsetupmode) в настройках модуля принимает значение `CNIPlugin` (по умолчанию — `InitContainer`).

   Состоит из следующих контейнеров:

   - **install-cni** — основной контейнер, устанавливающий и настраивающий CNI-плагин на узле;
   - **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищённого доступа к метрикам install-cni.

1. **Ztunnel** (DaemonSet) — компонент ambient-режима Istio, обеспечивающий L4-прослойку данных (mTLS, авторизацию на уровне L4) без добавления сайдкара к приложению пользователя. Запускается на каждом узле кластера.

   Компонент создаётся контроллером Deckhouse, если параметр [`.settings.ambient.enabled`](/modules/istio/configuration.html#parameters-ambient-enabled) в настройках модуля принимает значение `true` (по умолчанию — `false`).

   Состоит из одного контейнера:

   - **istio-proxy** — основной контейнер, получающий конфигурацию по протоколу xDS от контроллера istiod.

1. **Waypoint-controller** (Deployment) — контроллер L7-прослойки ambient-режима. Контроллер управляет кастомным ресурсом [WaypointInstance](/modules/istio/cr.html#waypointinstance) и создаёт или обновляет соответствующий Deployment `waypoint-<NAME>`.

   Создаётся при тех же условиях, что и ztunnel.

   Состоит из одного контейнера:

   - **waypoint-controller** — основной контейнер.

1. **Waypoint-\<NAME>** (Deployment) — проксирующий сервис, обрабатывающий L7-трафик (HTTP-маршрутизацию и авторизацию) для сервисов или рабочих нагрузок указанного неймспейса. Создаётся динамически компонентом waypoint-controller для каждого ресурса WaypointInstance.

   Состоит из одного контейнера:

   - **istio-proxy** — основной контейнер, получающий конфигурацию по протоколу xDS от контроллера istiod.

1. **Metadata-exporter** (Deployment) — компонент, предоставляющий публичные метаданные кластера (корневой сертификат CA, публичные ключи, адреса эндпоинтов) удалённым кластерам для настройки межкластерного взаимодействия ([федерация](/modules/istio/#федерация) или [мультикластер](/modules/istio/#мультикластер)).

   Компонент создаётся контроллером Deckhouse, если включён параметр [`.settings.federation.enabled`](/modules/istio/configuration.html#parameters-federation-enabled) или параметр [`.settings.multicluster.enabled`](/modules/istio/configuration.html#parameters-multicluster-enabled) в настройках модуля (по умолчанию оба — `false`).

   Состоит из следующих контейнеров:

   - **metadata-exporter** — основной контейнер;
   - **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищённого доступа к метрикам.

1. **Alliance-healthcheck** (Deployment) — компонент, проверяющий доступность меш-соединения с удалёнными кластерами и обновляющий статус кастомных ресурсов IstioFederation и IstioMulticluster.

   Создаётся при тех же условиях, что и metadata-exporter.

   Состоит из одного контейнера:

   - **healthcheck** — основной контейнер.

1. **Ingressgateway** (DaemonSet) — компонент, принимающий меш-трафик от удалённых кластеров через mTLS с SNI passthrough. Отдельный от ingress-gateway-controller-\<NAME> компонент, обслуживающий исключительно federation- и multicluster-трафик.

   Компонент создаётся контроллером Deckhouse, если:

   - включён параметр [`.settings.federation.enabled`](/modules/istio/configuration.html#parameters-federation-enabled) в настройках модуля;
   - включён параметр [`.settings.multicluster.enabled`](/modules/istio/configuration.html#parameters-multicluster-enabled) в настройках модуля и включён параметр [`.spec.enableIngressGateway`](/modules/istio/cr.html#istiomulticluster-v1alpha1-spec-enableingressgateway) в кастомном ресурсе IstioMulticluster.

   Состоит из одного контейнера:

   - **istio-proxy** — основной контейнер, получающий конфигурацию по протоколу xDS от контроллера istiod.

1. **Metrics-exporter** (Deployment) — компонент, собирающий данные для метрик multicluster-конфигурации.

   Компонент создаётся контроллером Deckhouse, если включён параметр [`.settings.multicluster.enabled`](/modules/istio/configuration.html#parameters-multicluster-enabled) в настройках модуля.

   Состоит из следующих контейнеров:

   - **metrics-exporter** — основной контейнер;
   - **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищённого доступа к метрикам.

1. **Api-proxy** (Deployment) — компонент, предоставляющий удалённым кластерам доступ на чтение к конфигурации сервис-меш и ресурсам приложений этого кластера. Используется istiod удалённого кластера для multicluster service discovery.

   Компонент создаётся контроллером Deckhouse, если включён параметр [`.settings.multicluster.enabled`](/modules/istio/configuration.html#parameters-multicluster-enabled) в настройках модуля.

   Состоит из одного контейнера:

   - **api-proxy** — основной контейнер.

Компонент, не входящий в состав модуля [`istio`](/modules/istio/):

- **Пользовательское приложение** — рабочая нагрузка, созданная пользователем и модифицированная компонентом istiod при мутации пода.

   Состоит из следующих контейнеров:

  - **istio-init** — опциональный init-контейнер, выполняющий подготовку iptables правил для перехвата трафика приложения. Добавляется, если параметр [`.settings.dataPlane.trafficRedirectionSetupMode`](/modules/istio/configuration.html#parameters-dataplane-trafficredirectionsetupmode) в настройках модуля принимает значение `InitContainer`. В случае значения `CNIPlugin` эту функцию выполняет компонент istio-cni-node;
  - **istio-proxy** — сайдкар-контейнер, обеспечивающий работу пользовательского приложения в меш-сети Istio. Добавляется, если параметр [`.settings.ambient.enabled`](/modules/istio/configuration.html#parameters-ambient-enabled) в настройках модуля принимает значение `false` (по умолчанию — `false`);
  - **user-app** — набор init-контейнеров и сайдкар-контейнеров пользовательского приложения.

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

   - авторизует запросы к метрикам компонентов модуля;
   - управляет кастомными ресурсами Istio, IstioRevision, IngressIstioController, IstioFederation, IstioMulticluster и WaypointInstance;
   - управляет кастомными ресурсами API-групп `networking.istio.io`, `security.istio.io`, `telemetry.istio.io` и `extensions.istio.io`;
   - создаёт и управляет ресурсами Deployment `istiod-<VERSION>` и `waypoint-<NAME>`, DaemonSet `ingress-gateway-controller-<NAME>`;
   - получает ресурсы Pod, Namespace, Node, Service, Secret, ConfigMap, Job, CronJob, Deployment, DaemonSet, ReplicaSet и StatefulSet.

1. **Модуль [`user-authn`](/modules/user-authn/)** — выполняет аутентификацию пользователей веб-интерфейса Kiali.
1. **Trickster** — запрашивает метрики трафика сервис-меш для веб-интерфейса Kiali.
1. **Внешний кластер DKP**:

   - проверяет доступность меш-соединения между кластерами;
   - получает параметры сервис-меш и приложений пользователя в удалённом кластере.

С модулем взаимодействуют следующие внешние компоненты:

1. **Kube-apiserver**:

   - валидирует кастомные ресурсы API-групп `networking.istio.io`, `security.istio.io`, `telemetry.istio.io` и `extensions.istio.io`;
   - мутирует поды для добавления init- и сайдкар-контейнеров.

1. **Prometheus-main** — собирает метрик всех компонентов модуля.
1. **Балансировщик нагрузки** — балансирует входящий трафик к ingress-gateway-controller.
1. **Controller nginx** — пересылает авторизованный запрос пользователя к веб-интерфейсу Kiali.
1. **Удалённый кластер DKP**:

   - запрашивает публичные метаданные кластера;
   - отправляет меш-трафик через mTLS SNI passthrough;
   - читает параметры сервис-меш и приложений пользователя кластера.

1. **Пользовательское приложение** — получает конфигурацию по протоколу xDS от контроллера istiod.
