---
title: Модуль operator-argo
permalink: ru/architecture/delivery/operator-argo.html
lang: ru
search: operator-argo, GitOps, Argo CD, развёртывание приложений
description: Архитектура модуля operator-argo в Deckhouse Kubernetes Platform.
---

Модуль [`operator-argo`](/modules/operator-argo/) разворачивает [Argo CD Operator](https://argocd-operator.readthedocs.io/) в кластере Deckhouse Kubernetes Platform (DKP). Модуль позволяет установить Argo CD в кластере DKP с помощью ресурса ArgoCD.

Модуль работает со следующими кастомными ресурсами:

- [Application](/modules/operator-argo/cr.html#application) — описание развёртывания приложений и управление ими;
- [ApplicationSet](/modules/operator-argo/cr.html#applicationset) — шаблонизация и массовое создание приложений по определённым правилам;
- [AppProject](/modules/operator-argo/cr.html#appproject) — определение набора приложений и политик доступа к приложениям;
- [ArgoCD](/modules/operator-argo/cr.html#argocd) — основной ресурс для развёртывания и настройки экземпляра Argo CD;
- [ArgoCDExport](/modules/operator-argo/cr.html#argocdexport) — экспорт настроек и состояния Argo CD для резервного копирования или миграции;
- [ImageUpdater](/modules/operator-argo/cr.html#imageupdater) — автоматическое обновление образов контейнеров приложений;
- [NamespaceManagement](/modules/operator-argo/cr.html#namespacemanagement) — определение правил управления неймспейсами для экземпляра Argo CD;
- [NotificationsConfiguration](/modules/operator-argo/cr.html#notificationsconfiguration) — определение параметров уведомлений о событиях в Argo CD и приложениях.

Подробнее с настройками модуля и примерами его использования можно ознакомиться в [соответствующем разделе документации](/modules/operator-argo/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`operator-argo`](/modules/operator-argo/) на уровне 2 модели C4 и его взаимодействие с другими компонентами DKP изображены на следующих диаграммах.

Основной вариант развёртывания с базой данных Redis в неотказоустойчивой конфигурации:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура модуля operator-argo с Redis non-HA](../../../images/architecture/delivery/c4-l2-operator-argo.ru.svg)

Вариант с отказоустойчивой конфигурацией Redis (на диаграмме отражены только отличия от основного варианта развёртывания):

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура модуля operator-argo с Redis HA](../../../images/architecture/delivery/c4-l2-operator-argo-ha.ru.svg)

Вариант развёртывания Argo CD в [управляющем кластере](https://argocd-agent.readthedocs.io/stable/concepts/components-terminology/) в мультикластерной конфигурации (на диаграмме отражены только отличия от основного варианта развёртывания):

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура модуля operator-argo с Principal](../../../images/architecture/delivery/c4-l2-operator-argo-principal.ru.svg)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **argocd-operator-controller-manager** (Deployment) — реализация [Argo CD Operator](https://argocd-operator.readthedocs.io/), позволяющая разворачивать экземпляры Argo CD в кластере DKP. Компонент работает со следующими кастомными ресурсами:
   - [ArgoCD](/modules/operator-argo/cr.html#argocd) — основной ресурс для развёртывания и настройки экземпляра Argo CD;
   - [ArgoCDExport](/modules/operator-argo/cr.html#argocdexport) — экспорт настроек и состояния Argo CD для резервного копирования или миграции. Оператор читает кастомный ресурс ArgoCDExport и создаёт Job/CronJob с именем, совпадающим с именем ресурса ArgoCDExport. Созданный Job/CronJob выполняет резервное копирование настроек экземпляра Argo CD;
   - [NamespaceManagement](/modules/operator-argo/cr.html#namespacemanagement) — определение правил управления неймспейсами для экземпляра Argo CD. Оператор следит за кастомным ресурсом NamespaceManagement и соответствующим образом обновляет ConfigMap `argocd-cmd-params-cm`;
   - [NotificationsConfiguration](/modules/operator-argo/cr.html#notificationsconfiguration) — определение параметров уведомлений о событиях в Argo CD и приложениях. Оператор читает кастомные ресурсы NotificationsConfiguration и на основе них обновляет конфигурацию в ConfigMap `argocd-notifications-cm`.

   `argocd-operator-controller-manager` создаёт ресурсы Deployment, Secret, ConfigMap, StatefulSet и другие для каждого кастомного ресурса ArgoCD, добавляя имя этого ресурса в качестве префикса для создаваемых ресурсов.

   Состоит из следующих контейнеров:

   - **manager** — основной контейнер;
   - **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищённого доступа к метрикам `manager`.

{% alert level="info" %}
Следующие компоненты описывают ресурсы, которые создаёт `argocd-operator-controller-manager` на основе конфигурации, заданной в кастомном ресурсе ArgoCD. Для описания используется префикс `<ArgoCD name>`, который будет заменяться контроллером на имя ресурса ArgoCD.
{% endalert %}

1. **&lt;ArgoCD name&gt;-server** (Deployment) — основной компонент взаимодействия с экземпляром Argo CD. &lt;ArgoCD name&gt;-server предоставляет REST/gRPC API и пользовательский веб-интерфейс для управления Argo CD. Компонент позволяет управлять кастомными ресурсами Application, ApplicationSet и AppProject через предоставляемые интерфейсы (веб, API, CLI).

   Оператор создаёт компонент, если в параметре [`.spec.server.enabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-server-enabled) кастомного ресурса ArgoCD задано значение `true` (значение по умолчанию — `true`).

   Состоит из следующих контейнеров:

   - **argocd-server-init** — опциональный набор инит-контейнеров, задаваемых пользователем в параметре [`.spec.server.initContainers`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-server-initcontainers) кастомного ресурса ArgoCD;
   - **rollout-extension** — опциональный инит-контейнер, загружающий расширение UI для работы с [кастомным ресурсом Rollout](https://argoproj.github.io/argo-rollouts/features/specification/). Модуль не предоставляет контроллер, обрабатывающий этот кастомный ресурс. Контроллер должен быть установлен и настроен дополнительно. `argocd-operator-controller-manager` добавляет rollout-extension, если значение параметра [`.spec.server.enableRolloutsUI`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-server-enablerolloutsui) принимает значение `true`;
   - **argocd-server** — основной контейнер;
   - **argocd-server-sidecar** — опциональный набор сайдкар-контейнеров, задаваемых пользователем в параметре [`.spec.server.sidecarContainers`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-server-sidecarcontainers) кастомного ресурса ArgoCD.

1. **&lt;ArgoCD name&gt;-repo-server** (Deployment) — компонент, отвечающий за обработку шаблонов, генерацию манифестов приложений и работу с внешними репозиториями, используемыми в Argo CD. &lt;ArgoCD name&gt;-repo-server отвечает за синхронизацию манифестов приложений из указанных репозиториев и передачу их в соответствующие компоненты для последующего развёртывания.

   Оператор создаёт компонент, если в параметре [`.spec.repo.enabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-repo-enabled) кастомного ресурса ArgoCD задано значение `true` (значение по умолчанию — `true`).

   Состоит из следующих контейнеров:

   - **copyutil** — инит-контейнер, копирующий исполняемые файлы для использования из основного контейнера;
   - **argocd-repo-server-init** — опциональный набор инит-контейнеров, настраиваемых через параметр [`.spec.repo.initContainers`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-repo-initcontainers) кастомного ресурса ArgoCD для подготовки окружения;
   - **argocd-repo-server** — основной контейнер, выполняющий операции по генерации и обработке манифестов, а также работу с удалёнными Git-репозиториями приложений;
   - **argocd-repo-server-sidecar** — опциональный набор сайдкар-контейнеров, задаваемых пользователем в параметре [`.spec.repo.sidecarContainers`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-repo-sidecarcontainers) кастомного ресурса ArgoCD и позволяющих расширять функциональность repo-server.

1. **&lt;ArgoCD name&gt;-application-controller** (StatefulSet) — компонент, отвечающий за синхронизацию и управление состоянием приложений, определённых в Argo CD. &lt;ArgoCD name&gt;-application-controller обеспечивает идемпотентное применение манифестов Kubernetes, управление процессом развёртывания, отката, автоматического восстановления, а также отслеживание состояния ресурсов в кластере.

   Оператор создаёт компонент, если в параметре [`.spec.controller.enabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-controller-enabled) кастомного ресурса ArgoCD задано значение `true` (значение по умолчанию — `true`).

   Состоит из следующих контейнеров:

   - **application-controller-init** — опциональный набор инит-контейнеров, настраиваемых через параметр [`.spec.controller.initContainers`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-controller-initcontainers) кастомного ресурса ArgoCD для подготовки окружения;
   - **argocd-application-controller** — основной контейнер, реализующий логику синхронизации кастомных ресурсов Application и создаваемых на их основе ресурсов;
   - **application-controller-sidecar** — опциональный набор сайдкар-контейнеров, задаваемых пользователем в параметре [`.spec.controller.sidecarContainers`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-controller-sidecarcontainers) кастомного ресурса ArgoCD, которые позволяют расширить стандартные возможности контроллера.

1. **&lt;ArgoCD name&gt;-applicationset-controller** (Deployment) — опциональный компонент, состоящий из одного контейнера **applicationset-controller** и отвечающий за управление кастомным ресурсом [ApplicationSet](/modules/operator-argo/cr.html#applicationset) в Argo CD. Он позволяет автоматически создавать, обновлять или удалять ресурсы Application на основе заданных шаблонов и генераторов (например, генераторов Git, List, Matrix и Cluster). Это облегчает массовое управление похожими приложениями, которые должны быть развёрнуты в различных окружениях или кластерах.

   Оператор создаёт компонент, если в параметре [`.spec.applicationSet.enabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-applicationset-enabled) кастомного ресурса ArgoCD задано значение `true` (значение по умолчанию — `true`).

   Более подробную информацию о компоненте можно найти в [документации applicationset-controller](https://argo-cd.readthedocs.io/en/stable/operator-manual/applicationset/).

1. **&lt;ArgoCD name&gt;-argocd-image-updater-controller** (Deployment) — опциональный компонент, состоящий из одного контейнера **argocd-image-updater** и предназначенный для автоматического обновления образов контейнеров в приложениях Argo CD при появлении новых версий в реестрах образов. Компонент отслеживает изменения тегов образов и при обнаружении новой версии обновляет соответствующие ресурсы Application в Argo CD (например, значения тегов образов в манифестах или `Helm values`) через запрос на слияние (pull request) в Git-репозиторий либо напрямую, в зависимости от выбранного способа работы.

   `<ArgoCD name>-argocd-image-updater-controller` выполняет следующие функции:

   - управляет кастомным ресурсом [ImageUpdater](/modules/operator-argo/cr.html#imageupdater), описывающим параметры для автоматического обновления образов контейнеров приложений;
   - периодически проверяет указанные в приложениях образы контейнеров в поддерживаемых реестрах (Docker Hub, Quay.io, Harbor и др.);
   - поддерживает фильтрацию тегов образов по шаблонам и стратегиям обновления (`semver`, `latest` и др.);
   - при обнаружении новой версии образа автоматически выполняет `write-back` в Argo CD Application или Git в зависимости от настроенного метода.

   Для корректной работы компоненту требуются права доступа к Git-репозиториям и, при необходимости, к приватным реестрам образов. Учётные данные для доступа к реестрам образов можно хранить в секретах Kubernetes.

   Для включения компонента необходимо задать в параметре [`.spec.imageUpdater.enabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-imageupdater-enabled) кастомного ресурса ArgoCD значение `true`.

   Более подробную информацию о компоненте можно найти в [документации argocd-image-updater](https://argocd-image-updater.readthedocs.io/).

1. **&lt;ArgoCD name&gt;-notifications-controller** (Deployment) — опциональный контроллер, состоящий из одного контейнера **argocd-notifications-controller** и реализующий отправку уведомлений о событиях в Argo CD (например, об успешной синхронизации приложения, ошибках развёртывания, изменениях статуса и др.) во внешние системы уведомлений, включая электронную почту, Slack, Microsoft Teams, Telegram, OpsGenie, Webhook и другие.

   Оператор `argocd-operator-controller-manager` формирует настройки для уведомлений на основе кастомных ресурсов [NotificationsConfiguration](/modules/operator-argo/cr.html#notificationsconfiguration) и сохраняет их в ConfigMap `argocd-notifications-cm` и Secret `argocd-notifications-secret`, которые используются контроллером для формирования и отправки уведомлений.

   Для включения компонента необходимо задать в параметре [`.spec.notifications.enabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-notifications-enabled) кастомного ресурса ArgoCD значение `true`.

   Более подробную информацию о механизмах работы можно найти в [документации по Argo CD Notifications](https://argo-cd.readthedocs.io/en/stable/operator-manual/notifications/).

1. **&lt;ArgoCD name&gt;-dex-server** (Deployment) — опциональный компонент для аутентификации пользователей в Argo CD, выступающий в роли OIDC-провайдера (OpenID Connect) на базе Dex. Компонент реализует возможность входа пользователей через различные внешние провайдеры аутентификации (LDAP, GitHub, GitLab, SAML, Azure AD и др.), а также поддерживает работу со статическими пользователями, определёнными в конфигурации Dex.

   Состоит из следующих контейнеров:

   - **copyutil** — инит-контейнер, копирующий исполняемые файлы для использования из основного контейнера;
   - **dex** — основной контейнер.

   Для включения компонента необходимо задать параметры в разделе [`.spec.sso.dex`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-sso-dex) кастомного ресурса ArgoCD.

   {% alert level="warning" %}
   Для аутентификации пользователей Argo CD в DKP модуль `operator-argo` поддерживает интеграцию с модулем [`user-authn`](/modules/user-authn/) (встроенная аутентификация DKP). Другие внешние провайдеры через Dex в данной конфигурации не используются.

   Подробнее см. в [примерах использования модуля `operator-argo`](/modules/operator-argo/examples.html#аутентификация).
   {% endalert %}

1. **&lt;ArgoCD name&gt;-redis** (Deployment) — обязательный компонент, состоящий из одного контейнера **redis** и отвечающий за хранение данных об очередях задач и состоянии сессий в Argo CD. &lt;ArgoCD name&gt;-redis реализует отдельный экземпляр базы данных [Redis](https://redis.io/).

   `argocd-operator-controller-manager` разворачивает этот компонент, если параметр [`.spec.ha.enabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-ha-enabled) кастомного ресурса ArgoCD принимает значение `false`.

1. **&lt;ArgoCD name&gt;-redis-ha-server** (StatefulSet) — обязательный компонент для развёртывания Redis в режиме высокой доступности (HA) в составе Argo CD. Реализует отказоустойчивый кластер Redis с репликацией и автоматическим переключением (`failover`) с помощью механизма [Redis Sentinel](https://redis.io/docs/latest/operate/oss_and_stack/management/sentinel/).

   Состоит из следующих контейнеров:

   - **config-init** — инит-контейнер, подготавливающий конфигурацию для Redis и Sentinel перед запуском основных контейнеров;
   - **redis** — основной контейнер, реализующий экземпляр Redis-сервера;
   - **sentinel** — вспомогательный контейнер, запускающий [Redis Sentinel](https://redis.io/docs/latest/operate/oss_and_stack/management/sentinel/) для мониторинга состояния экземпляров Redis и автоматического переключения на реплику при отказе основного экземпляра.

   `argocd-operator-controller-manager` разворачивает этот компонент, если параметр [`.spec.ha.enabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-ha-enabled) кастомного ресурса ArgoCD принимает значение `true`.

1. **&lt;ArgoCD name&gt;-redis-ha-haproxy** (Deployment) — дополнительный компонент для балансировки нагрузки и распределения трафика к экземплярам кластера Redis (`redis-ha-server`).

   Состоит из следующих контейнеров:

   - **config-init** — инит-контейнер, подготавливающий конфигурацию для HAProxy перед запуском основного контейнера;
   - **haproxy** — контейнер, работающий в роли прокси-сервера и обеспечивающий прозрачную маршрутизацию запросов клиентов к доступным `master`/`replica`-экземплярам Redis, а также автоматизацию переключения между ними при `failover`.

   `argocd-operator-controller-manager` разворачивает этот компонент, если параметр [`.spec.ha.enabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-ha-enabled) кастомного ресурса ArgoCD принимает значение `true`.

1. **&lt;ArgoCD name&gt;-agent-agent** (Deployment) — опциональный компонент, состоящий из одного контейнера **&lt;ArgoCD name&gt;-agent-agent** и отвечающий за выполнение операций над управляемыми ресурсами Kubernetes-кластера по заданию из Argo CD. Компонент устанавливает подключение к Argo CD Principal, синхронизирует приложения и управляет их состоянием на основе команд, поступающих от Argo CD Principal.

   Подробнее с архитектурой мультикластерной конфигурации Argo CD можно ознакомиться в [документации Argo CD](https://argocd-agent.readthedocs.io/stable/concepts/architecture/#architectural-diagram).

   `argocd-operator-controller-manager` разворачивает этот компонент, если параметр [`.spec.argoCDAgent.agent.enabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-argocdagent-agent-enabled) кастомного ресурса ArgoCD принимает значение `true`. В одном ресурсе ArgoCD не допускается одновременное использование Argo CD Agent и Argo CD Principal.

1. **&lt;ArgoCD name&gt;-agent-principal** (Deployment) — опциональный компонент, состоящий из одного контейнера **&lt;ArgoCD name&gt;-agent-principal** и обеспечивающий работу Argo CD в [мультикластерной конфигурации](https://argocd-agent.readthedocs.io/stable/concepts/architecture/#architectural-diagram).

   При включении этого компонента `argocd-operator-controller-manager` перенастраивает все компоненты, использующие подключение к базе Redis, на использование Redis-прокси. Компонент `&lt;ArgoCD name&gt;-agent-principal` реализует Redis-прокси и маршрутизирует запросы к базе данных на основе анализа ключей Redis: в зависимости от значения ключей запрос направляется или в локальный экземпляр Redis, или в один из удалённых Argo CD Agent.

   `argocd-operator-controller-manager` разворачивает этот компонент, если параметр [`.spec.argoCDAgent.principal.enabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-argocdagent-principal-enabled) кастомного ресурса ArgoCD принимает значение `true`. В одном ресурсе ArgoCD не допускается одновременное использование Argo CD Agent и Argo CD Principal.

1. **&lt;Export name&gt;** (Job/CronJob) — опциональный компонент, реализованный в виде Job или CronJob и создающий под из одного контейнера **argocd-export**. Компонент создаёт резервную копию настроек и состояния экземпляра Argo CD.

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. Внешние репозитории образов — получение списка образов.
1. Внешние репозитории кода/манифестов:
    - получение манифестов развёртывания приложения из репозиториев;
    - обновление `image` в исходном коде Helm-чарта.
1. Внешний Argo CD Principal:
    - подключение к управляющему кластеру Argo CD;
    - получение запросов на обработку;
    - передача результатов выполнения запросов.
1. **kube-apiserver**:
    - управление кастомными ресурсами Application, ApplicationSet, AppProject, ArgoCD, ArgoCDExport, ImageUpdater, NamespaceManagement, NotificationsConfiguration, а также Secret, ConfigMap;
    - управление ресурсами, которые создаются при развёртывании пользовательского приложения, описанного в кастомном ресурсе Application;
    - авторизация запросов на получение метрик.
1. **Модуль [user-authn](/modules/user-authn/)** — перенаправление пользователя для аутентификации.

С модулем взаимодействуют следующие внешние компоненты:

1. **prometheus-main** — сбор метрик, предоставляемых оператором и экземплярами Argo CD.
1. Внешний Argo CD Agent:
    - подключение к управляющему кластеру Argo CD;
    - получение запросов на обработку;
    - передача результатов выполнения запросов.
