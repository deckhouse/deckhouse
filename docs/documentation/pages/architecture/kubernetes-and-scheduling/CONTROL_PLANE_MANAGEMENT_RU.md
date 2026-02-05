---
title: Управление компонентами control plane кластера
permalink: ru/architecture/kubernetes-and-scheduling/control-plane-management/
lang: ru
search: control-plane-manager, управление control plane
---

## Модуль control-plane-manager

Управление компонентами control plane кластера осуществляется с помощью модуля [control-plane-manager](/modules/control-plane-manager/), который запускается на всех master-узлах кластера (узлы с лейблом `node-role.kubernetes.io/control-plane: ""`).

Функции управления control plane:

* **Управление сертификатами**, необходимыми для работы control-plane, в том числе продление, выпуск при изменении конфигурации и т. п. Позволяет автоматически поддерживать безопасную конфигурацию control plane и быстро добавлять дополнительные SAN для организации защищенного доступа к API Kubernetes.
* **Настройка компонентов**. Автоматически создает необходимые конфигурации и манифесты компонентов control-plane.
* **Upgrade/downgrade компонентов**. Поддерживает в кластере одинаковые версии компонентов.
* **Управление конфигурацией etcd-кластера и его членов**. Масштабирует master-узлы, выполняет миграцию из single-master в multi-master и обратно.
* **Настройка kubeconfig**. Обеспечивает всегда актуальную конфигурацию для работы kubectl на узлах кластера. Генерирует, продлевает, обновляет `kubeconfig` с правами *cluster-admin* и создает symlink пользователю root, чтобы `kubeconfig` использовался по умолчанию.
* **Расширение работы планировщика**, за счет подключения внешних плагинов через вебхуки. Управляется ресурсом `KubeSchedulerWebhookConfiguration`. Позволяет использовать более сложную логику при решении задач планирования нагрузки в кластере, например:

  * размещение подов приложений организации хранилища данных ближе к самим данным,
  * приоритизация узлов в зависимости от их состояния (сетевой нагрузки, состояния подсистемы хранения и т. д.),
  * разделение узлов на зоны, и т. п.

Подробнее с настройками модуля и примерами его использования можно ознакомиться в соответствующем [разделе документации](/modules/control-plane-manager/).

### Архитектура модуля

{% alert level="info" %}
Для лучшего восприятия схемы на ней допущены следующие упрощения:

* На схеме выглядит так, что контейнеры подов взаимодействуют с контейнерами других подов напрямую. На самом деле они взаимодействуют через соответствующие им сервисы Kubernetes (внутренние балансировщики). Если взаимодействие происходит через специфичный сервис, в подписи над стрелкой указано название сервиса.
* Поды могут быть запущены несколькими репликами. На схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [control-plane-manager](/modules/control-plane-manager/) на уровне 2 модели C4 и его взаимодействия с другими компонентами платформы изображены на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4 --->
![c4 l2 control-plane-manager](../../../images/architecture/kubernetes-and-scheduling/c4-l2-control-plane-manager.png)

### Компоненты модуля

Модуль состоит из следующих компонентов:

1. **d8-control-plane-manager** (DaemonSet) — управляет компонентами control plane кластера, запускается на всех master-узлах кластера, в свою очередь состоит из следующих контейнеров:

   * **control-plane-manager** - основной контейнер. **control-plane-manager** является разработкой компании Флант.

   Набор sidecar-контейнеров, предназначенных для предварительного скачивания образов соответствующих компонентов control-plane, стоят на паузе, не выполняют никакой работы:

   * **image-holder-kube-apiserver**
   * **image-holder-kube-apiserver-healthcheck**
   * **image-holder-kube-controller-manager**
   * **image-holder-kube-scheduler**
   * **image-holder-etcd**

2. **kubernetes-api-proxy** (статические поды) - на каждом master-узле настраивается дополнительный прокси-сервер, отвечающий на `localhost. Прокси-сервер по умолчанию обращается к локальному экземпляру **kube-apiserver**, но в случае его недоступности последовательно опрашивает остальные экземпляры **kube-apiserver**. Включает в себя следующие контейнеры:

   * **kubernetes-api-proxy** - собственно, сам прокси-сервер [NGINX](https://github.com/nginx/nginx).
   * **kubernetes-api-proxy-reloader** - sidecar-контейнер, перезапускает контейнер с прокси-сервером при изменении его конфигурации. **kubernetes-api-proxy-reloader** является разработкой компании Флант.

3. **d8-etcd-backup** (CronJob) - периодически выполняет резервное копирование базы данных **etcd** кластера. Состоит из контейнера:

   * **backup** - контейнер с shell-скриптом, который через утилиту etcdctl создает снимок базы данных и сохраняет его в каталоге `/var/lib/etcd` на master-узле (это каталог по-умолчанию, его можно переопределить через [параметры модуля](/modules/control-plane-manager/configuration.html#parameters-etcd-backup)).

### Взаимодействия модуля

Модуль взаимодействует с:
1. **kube-apiserver**:

   * управление control-plane компонентами кластера.
   * проксирование и балансировка запросов до **kube-apiserver**, отправляемых на адрес `localhost`.

2. **etcd**:

   * управление конфигурацией etcd-кластера и его членов.
   * периодическое резервное копирование базы данных.

С модулем взаимодействуют следующие внешние для него компоненты:

1. **kubelet** - запросы до **kube-apiserver**, отправляемые на адрес `localhost`, проксируются компонентом **kubernetes-api-proxy** модуля.

## Мониторинг control plane кластера

Мониторинг control plane кластера осуществляется с помощью модуля [monitoring-kubernetes-control-plane](/modules/monitoring-kubernetes-control-plane/), который организует безопасный сбор метрик и предоставляет базовый набор правил мониторинга следующих компонентов кластера:

* **kube-apiserver**
* **kube-controller-manager**
* **kube-scheduler**
* **etcd**

Подробнее с настройками модуля **monitoring-kubernetes-control-plane** можно ознакомиться в соответствующем [разделе документации](/modules/monitoring-kubernetes-control-plane/).

### Компоненты модуля monitoring-kubernetes-control-plane

Модуль состоит из одного компонента:
1. **control-plane-proxy** (DaemonSet) — запускается на всех master-узлах кластера, в свою очередь состоит из одного контейнера:

   * **kube-rbac-proxy** - контейнер с авторизирующим прокси на основе Kubernetes RBAC для организации защищенного доступа к метрикам.

### Взаимодействия компонента **control-plane-proxy**

**control-plane-proxy** взаимодействует с:

1. **kube-apiserver**:

   * авторизация запросов на получение метрик.

2. Компонентами control plane кластера. **control-plane-proxy** пересылает авторизованные запросы на метрики до:

   * **kube-controller-manager**
   * **kube-scheduler**
   * **etcd**

С **control-plane-proxy** взаимодействует **prometheus-main** - сбор метрик компонентов control plane кластера.

Взаимодействие компонента модуля **monitoring-kubernetes-control-plane** с control plane кластера изображено на приведенной выше схеме архитектуры модуля **control-plane-manager**.

### Сбор метрик с kube-apiserver

Метрики **kube-apiserver** собираются **prometheus-main** напрямую, модуль [monitoring-kubernetes-control-plane](/modules/monitoring-kubernetes-control-plane/) только добавляет правила сбора метрик **kube-apiserver** в **prometheus-main**.
