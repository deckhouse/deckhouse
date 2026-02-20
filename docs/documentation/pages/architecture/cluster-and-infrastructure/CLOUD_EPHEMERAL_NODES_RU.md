---
title: Управление CloudEphemeral-узлами
permalink: ru/architecture/cluster-and-infrastructure/cloud-ephemeral-nodes/
lang: ru
search: cloudephemeral nodes, cloudephemeral узлы
---

На данной странице описана архитектура модуля [node-manager](/modules/node-manager/) для CloudEphemeral-узлов.

## Архитектура модуля

{% alert level="info" %}
Для лучшего восприятия схемы на ней допущены следующие упрощения:

* На схеме выглядит так, что контейнеры подов взаимодействуют с контейнерами других подов напрямую. На самом деле они взаимодействуют через соответствующие им сервисы Kubernetes (внутренние балансировщики). Если взаимодействие происходит через специфичный сервис, в подписи над стрелкой указано название сервиса.
* Поды могут быть запущены несколькими репликами. На схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [node-manager](/modules/node-manager/) на уровне 2 модели C4 и его взаимодействия с другими компонентами платформы изображены на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4 --->
![c4 l2 cloud-ephemeral-nodes](../../../images/architecture/cluster-and-infrastructure/c4-l2-cloud-ephemeral-nodes.png)

## Компоненты модуля

{% alert level="info" %}
**Bashible** - ключевой компонент подсистемы **Cluster & Infrastructure**, на котором завязана работа модуля. Однако он не является компонентом модуля, так как работает на уровне ОС как системная служба. **Bashible** подробно описан в соответствующем [разделе документации](../bashible/)
{% endalert %}

Модуль, управляющий CloudEphemeral-узлами, состоит из следующих компонентов:

1. **bashible-api-server** - [Kubernetes Extension APIServer](https://kubernetes.io/docs/tasks/extend-kubernetes/setup-extension-api-server/), который деплоится на Master-узлы. **bashible-api-server** генерирует bashible-скрипты из шаблонов, которые хранятся в Custom Resources. При обращении к **kube-apiserver** за ресурсами, содержащими бандлы **bashible**, **kube-apiserver** обращается к **bashible-api-server** и возвращает результат от него. Подробнее с работой **bashible** и **bashible-api-server** можно ознакомиться в соответствующем [разделе документации](../bashible/).

2. **capi-controller-manager** (Deployment) — core-контроллеры из проекта Kubernetes [Cluster API](https://github.com/kubernetes-sigs/cluster-api). **Cluster API** является расширением для Kubernetes, которое дает возможность управлять Kubernetes-кластерами как Custom Resources внутри другого Kubernetes-кластера. Под **capi-controller-manager** в свою очередь состоит из следующих контейнеров:

   * **control-plane-manager** - основной контейнер.
   * **kube-rbac-proxy** - sidecar-контейнер с авторизирующим прокси на основе Kubernetes RBAC для организации защищенного доступа к метрикам контроллера.

3. **cluster-autoscaler** (Deployment) -  дополнительный [компонент](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler) Kubernetes, который автоматически изменяет количество узлов в кластере в зависимости от нагрузки. Подробнее с автоматическим масштабированием узлов можно ознакомиться в [документации модуля](/products/kubernetes-platform/documentation/v1/architecture/node.html#%D0%BC%D0%B0%D1%81%D1%88%D1%82%D0%B0%D0%B1%D0%B8%D1%80%D0%BE%D0%B2%D0%B0%D0%BD%D0%B8%D0%B5-%D1%83%D0%B7%D0%BB%D0%BE%D0%B2-%D0%B2-%D0%BE%D0%B1%D0%BB%D0%B0%D0%BA%D0%B5). Включает в себя следующие контейнеры:

   * **cluster-autoscaler** - основной контейнер.
   * **kube-rbac-proxy** - sidecar-контейнер с авторизирующим прокси на основе Kubernetes RBAC для организации защищенного доступа к метрикам **cluster-autoscaler**.

4. **early-oom** (DaemonSet) - на каждом узле разворачивается под, который читает из каталога `/proc` метрики по загрузке ресурсов на хосте и в случае повышенной нагрузки уничтожает поды раньше, чем это сделает [kubelet](../../kubernetes-and-scheduling/kubelet/). **early-oom** по умолчанию включен, но его можно отключить в [настройках модуля](/modules/node-manager/configuration.html#parameters-earlyoomenabled), если его работа создаёт проблемы в нормальной работе узлов. Включает в себя следующие контейнеры:

   * **psi-monitor** - основной контейнер, он следит за метрикой *PSI (Pressure Stall Information)*, которая показывает время, в течение которого процессы ожидают освобождения определённых ресурсов, таких как CPU, память или I/O.
   * **kube-rbac-proxy** - sidecar-контейнер с авторизирующим прокси на основе Kubernetes RBAC для организации защищенного доступа к метрикам **early-oom**.

5. **fencing-agent** (DaemonSet) - разворачивается на определенной группе узлов (NodeGroup) при включённой [настройке в спецификации Custom Resource NodeGroup](/modules/node-manager/cr.html#nodegroup-v1-spec-fencing). После запуска агент активирует Watchdog и устанавливает специальную метку `node-manager.deckhouse.io/fencing-enabled` на узле, где он функционирует. Агент регулярно проверяет доступность Kubernetes API. Если API доступен, агент отправляет сигнал в Watchdog, что сбрасывает сторожевой таймер. Также агент отслеживает специальные метки обслуживания на узле и, в зависимости от их наличия, включает или отключает Watchdog. В качестве Watchdog используется модуль ядра *softdog* с параметрами `soft_margin=60` и `soft_panic=1`. Это означает, что время таймаута сторожевого таймера составляет 60 секунд. По истечении этого времени происходит *kernel-panic*, и узел остается в этом состоянии до тех пор, пока пользователь не выполнит его перезагрузку. Состоит из одного контейнера:

   * **fencing-agent** - выполняет описанные выше проверки, сигнал в Watchdog отправляется посредством записи в файл `/dev/watchdog` на хосте.

6. **fencing-controller** - контроллер, который отслеживает все узлы с установленной меткой `node-manager.deckhouse.io/fencing-enabled`. Если какой-либо из узлов становится недоступным на протяжении более 60 секунд, контроллер удаляет все поды с этого узла и затем удаляет сам узел.

7. **standby-holder** (Deployment) - под, используемый для резервирования узлов. При включенной [настройке в спецификации Custom Resource NodeGroup](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-standby) в соответствующей группе узлов во всех [зонах](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) создаются резервные узлы. Резервный узел — это узел кластера, на котором резервируются ресурсы, доступные в любой момент для масштабирования. Наличие такого узла позволяет **cluster autoscaler**’у не ждать инициализации узла (которая может занимать несколько минут), а сразу размещать на нем нагрузку. **standby-holder** не выполняет никакой полезной работы, а резервируя ресурсы, не дает **cluster autoscaler**’у удалить пока никем не используемый узел.
Под имеет минимальный *PriorityClass* и вытесняется с узла при распределении нагрузки на узел. Подробнее с *Pod Priority and Preemption* можно ознакомиться в [документации Kubernetes](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/). Под состоит из одного контейнера:

   * **reserve-resources** - такой же **pause** контейнер, какой используется во всех подах для резервирования сетевого namespace (в подах он скрыт от пользователя).

## Взаимодействия модуля

Модуль взаимодействует с:

1. **kube-apiserver**:

   * получение секрета `kube-system/d8-node-manager-cloud-provider` для подключения к облаку,
   * работа с Custom Resources Cluster API,
   * работа с ресурсами Node,
   * отслеживание нагрузки на узлах,
   * автомасштабирование узлов,
   * авторизация запросов на метрики.

2. Файлами на узлах:

   * `/proc` - читает метрики PSI для OOM Kill.
   * `/dev/watchdog` - отправляет сигнал в Watchdog для сброса сторожевого таймера.

{% alert level="info" %}
Модуль также (не напрямую, а через **kube-apiserver**) взаимодействует с модулем **cloud-provider** через секрет `kube-system/d8-node-manager-cloud-provider`, получая все необходимые настройки для подключения к облаку и создания CloudEphemeral-инстансов. Также **cloud-provider** предоставляет нод-менеджеру шаблоны для создания провайдер-специфичных Custom Resources Cluster API.
{% endalert %}

С модулем взаимодействуют следующие внешние для него компоненты:

1. **kube-apiserver**:

   * Выполняет mutating/validating вебхуки **capi-controller-manager**.
   * Пересылает **bashible-api-server** запросы на ресурсы **bashible**.

2. **prometheus-main** - сбор метрик компонентов модуля **node-manager**.

## Особенности архитектуры, специфичные для CloudEphemeral-узлов

1. Узлы эфемерны, автоматически создаются и удаляются модулем.
2. **cloud-provider** - для взаимодействия с IaaS облака необходим установленный и настроенный облачный провайдер для этого облака (cloud-provider-* на схеме). Включает также **csi-driver** и **cloud-controller-manager**.
3. **capi-controller-manager** - компонент обеспечивающий жизненный цикл самого кластера и его узлов. Самостоятельно не заказывает узлы в облаке, работает с Custom Resources более высокого уровня, не привязанного к инфраструктуре. Генерирует инфраструктурные Custom Resources, оставляя всю работу для инфраструктурного провайдера, который деплоится модулем конкретного облачного провайдера **cloud-provider**.
4. **cluster-autoscaler** - обеспечивает автомасштабирования узлов кластера.
5. Возможно резервирование узлов.
