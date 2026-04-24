---
title: Управление CloudEphemeral-узлами
permalink: ru/architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html
lang: ru
search: cloudephemeral узлы
description: Архитектура модуля node-manager для CloudEphemeral-узлов.
---

На данной странице описана архитектура модуля [`node-manager`](/modules/node-manager/) для CloudEphemeral-узлов.

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`node-manager`](/modules/node-manager/) на уровне 2 модели C4 и его взаимодействия с другими компонентами Deckhouse Kubernetes Platform (DKP) изображены на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура модуля node-manager для CloudEphemeral-узлов](../../../../images/architecture/cluster-and-infrastructure/c4-l2-cloud-ephemeral-nodes.ru.png)

## Компоненты модуля

{% alert level="info" %}
Bashible — это ключевой компонент подсистемы Cluster & Infrastructure, обеспечивающий работу модуля `node-manager`. При этом он не является компонентом модуля, поскольку работает на уровне ОС в качестве системной службы. Bashible подробно описан в [соответствующем разделе документации](bashible.html)
{% endalert %}

Модуль, управляющий CloudEphemeral-узлами, состоит из следующих компонентов:

1. **Bashible-api-server** — [Kubernetes Extension API Server](https://kubernetes.io/docs/tasks/extend-kubernetes/setup-extension-api-server/), развернутый на master-узлах. Генерирует bashible-скрипты из шаблонов, хранящихся в кастомных ресурсах. При обращении к kube-apiserver за ресурсами, содержащими бандлы bashible, kube-apiserver перенаправляет запрос в bashible-api-server и возвращает сформированный результат. Подробнее с описанием работы bashible и bashible-api-server можно ознакомиться в [соответствующем разделе документации](bashible.html).

2. **Capi-controller-manager** (Deployment) — основные контроллеры из проекта [Kubernetes Cluster API](https://github.com/kubernetes-sigs/cluster-api). Cluster API является расширением Kubernetes, которое дает возможность управлять кластерами как кастомными ресурсами внутри другого Kubernetes-кластера. Под capi-controller-manager состоит из следующих контейнеров:

   * **control-plane-manager** — основной контейнер;
   * **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищенного доступа к метрикам контроллера.

3. **Cluster-autoscaler** (Deployment) — [дополнительный компонент Kubernetes](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler), который автоматически изменяет количество узлов в кластере в зависимости от нагрузки. Подробнее с автоматическим масштабированием узлов можно ознакомиться в [разделе документации по управлению узлами](overview.html#масштабирование-узлов-в-облаке).

   Компонент включает в себя следующие контейнеры:

   * **cluster-autoscaler** — основной контейнер;
   * **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищенного доступа к метрикам **cluster-autoscaler**.

4. **Early-oom** (DaemonSet) — на каждом узле разворачивается под, который считывает из каталога `/proc` метрики по загрузке ресурсов на хосте и в случае повышенной нагрузки завершает поды раньше, чем это сделает [kubelet](../../kubernetes-and-scheduling/kubelet.html). **Early-oom** по умолчанию включен, но его можно отключить в [настройках модуля](/modules/node-manager/configuration.html#parameters-earlyoomenabled) в случае, если он создаёт проблемы для нормальной работы узлов.

   Включает в себя следующие контейнеры:

   * **psi-monitor** — основной контейнер, который отслеживает метрику *PSI (Pressure Stall Information)*, отражающую время, в течение которого процессы ожидают освобождения определённых ресурсов, таких как CPU, память или I/O;
   * **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищенного доступа к метрикам **early-oom**.

5. **Fencing-agent** (DaemonSet) и **Fencing-controller** — компоненты, реализующие механизм fencing. Подробное описание приведено на странице [Управление Static-узлами](static-nodes.html#компоненты-модуля). Подробнее о том, как fencing обрабатывает разные типы узлов, см. раздел [«Как fencing обрабатывает разные типы узлов»](/modules/node-manager/faq.html#как-fencing-обрабатывает-разные-типы-узлов) в FAQ модуля `node-manager`.

6. **Standby-holder** (Deployment) — под для резервирования узлов. При включенном параметре [`spec.cloudinstances.standby`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-standby) кастомного ресурса NodeGroup в соответствующей группе узлов во всех [зонах](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) создаются резервные узлы.

   Резервный узел — это узел кластера, на котором резервируются ресурсы, доступные в любой момент для масштабирования. Наличие такого узла позволяет cluster-autoscaler не ждать инициализации узла (которая может занимать несколько минут), а сразу размещать на нем нагрузку.

   Standby-holder не выполняет никакой полезной работы, а резервирует ресурсы, не позволяя cluster-autoscaler удалить временно неиспользуемый узел.

   У пода standby-holder минимальный PriorityClass, и он вытесняется с узла при появлении реальной нагрузки. Подробнее о приоритизации и вытеснении подов можно почитать в [документации Kubernetes](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/).

   Под содержит один контейнер **reserve-resources**.

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

   * получение секрета `kube-system/d8-node-manager-cloud-provider` для подключения к облаку;
   * работа с кастомными ресурсами Cluster API;
   * работа с ресурсами Node;
   * отслеживание нагрузки на узлах;
   * автомасштабирование узлов;
   * авторизация запросов на метрики.

2. Файлы на узлах:

   * `/proc` — читает метрики PSI для OOM Kill;
   * `/dev/watchdog` — отправляет сигнал в Watchdog для сброса сторожевого таймера.

{% alert level="info" %}
Модуль взаимодействует с модулем `cloud-provider` через kube-apiserver, используя секрет `kube-system/d8-node-manager-cloud-provider`, для получения всех необходимых настроек подключения к облаку и создания CloudEphemeral-узлов. Также `cloud-provider` предоставляет модулю `node-manager` шаблоны для создания кастомных ресурсов Cluster API, специфичных для определенных провайдеров.
{% endalert %}

С модулем взаимодействуют следующие внешние для него компоненты:

1. **Kube-apiserver**:

   * выполняет mutating- и validating-вебхуки capi-controller-manager;
   * пересылает в bashible-api-server запросы на ресурсы bashible.

2. **Prometheus-main** — сбор метрик компонентов модуля `node-manager`.

## Особенности архитектуры, специфичные для CloudEphemeral-узлов

1. Узлы эфемерны, автоматически создаются и удаляются модулем.
2. Для взаимодействия с инфраструктурой облака необходим установленный и настроенный облачный провайдер (`cloud-provider-*` на схеме). Включает также csi-driver и cloud-controller-manager.
3. **Capi-controller-manager** — компонент, обеспечивающий жизненный цикл самого кластера и его узлов. Не заказывает узлы в облаке самостоятельно, работает с кастомными ресурсами более высокого уровня, не привязанного к инфраструктуре. Генерирует инфраструктурные кастомные ресурсы, оставляя всю работу для инфраструктурного провайдера, который развертывается модулем конкретного облачного провайдера `cloud-provider`.
4. **Cluster-autoscaler** — обеспечивает автомасштабирование узлов кластера.
5. Поддерживается резервирование узлов.
