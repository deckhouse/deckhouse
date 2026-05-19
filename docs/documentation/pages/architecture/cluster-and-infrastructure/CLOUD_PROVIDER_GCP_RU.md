---
title: Модуль cloud-provider-gcp
permalink: ru/architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-gcp.html
lang: ru
search: cloud-provider-gcp, cloud provider gcp, google cloud platform
description: Архитектура модуля cloud-provider-gcp в Deckhouse Kubernetes Platform.
---

Модуль [`cloud-provider-gcp`](/modules/cloud-provider-gcp/) обеспечивает интеграцию с облачными ресурсами [Google Cloud Platform](https://cloud.google.com/). Он используется модулем [`node-manager`](/modules/node-manager/) для заказа узлов в соответствии [с настройками группы узлов](/modules/node-manager/cr.html#nodegroup).

Подробнее с описанием модуля можно ознакомиться [в соответствующем разделе документации](/modules/cloud-provider-gcp/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`cloud-provider-gcp`](/modules/cloud-provider-gcp/) на уровне 2 модели C4 и его взаимодействия с другими компонентами Deckhouse Kubernetes Platform (DKP) изображены на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура модуля cloud-provider-gcp](../../../../images/architecture/cluster-and-infrastructure/c4-l2-cloud-provider-gcp.ru.png)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Cloud-controller-manager** — реализация [cloud controller manager](https://kubernetes.io/ru/docs/concepts/architecture/cloud-controller/) для GCP. Компонент обеспечивает интеграцию с облаком GCP и выполняет следующие функции:

   * реализует связь 1:1 между объектом узла в Kubernetes (Node) и виртуальной машиной в облачном провайдере. Для этого:

     * заполняет поля `spec.providerID` и `nodeInfo` ресурса Node;
     * проверяет наличие виртуальной машины в облаке и при ее отсутствии удаляет ресурс Node в кластере.

   * при создании ресурса Service типа LoadBalancer в Kubernetes создаёт балансировщик в облаке, который направляет трафик извне к узлам кластера;
   * создает сетевые маршруты для сети `PodNetwork` на стороне GCP.

   Подробнее о cloud-controller-manager можно почитать [в документации Kubernetes](https://kubernetes.io/ru/docs/concepts/architecture/cloud-controller/).

   Состоит из одного контейнера:

   * **gcp-cloud-controller-manager**.

1. **Cloud-data-discoverer** — отвечает за сбор данных из API облачного провайдера и предоставление их в виде секрета `kube-system/d8-cloud-provider-discovery-data`. Этот секрет содержит параметры конкретного облака, которые используются другими компонентами модуля `cloud-provider-gcp`.

   Состоит из следующих контейнеров:

   * **cloud-data-discoverer** — основной контейнер;
   * **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищенного доступа к метрикам контейнера cloud-data-discoverer.

1. **CSI-драйвер (gcp)** — реализация CSI-драйвера для GCP. С типовой архитектурой CSI-драйвера, используемого в модулях `cloud-provider-*` DKP, можно ознакомиться в [соответствующем разделе документации](../infrastructure/csi-driver.html).

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

    * мониторинг ресурсов PersistentVolumeClaim, VolumeAttachment;
    * создание секрета `kube-system/d8-cloud-provider-discovery-data`;
    * синхронизация узлов Kubernetes с виртуальными машинами в облаке;
    * мониторинг сервисов типа LoadBalancer;
    * авторизация запросов на получение метрик.

1. **Google Cloud Platform**:

    * получение параметров облака;
    * получение `ProviderID` и прочей информации о виртуальных машинах, которые являются узлами кластера;
    * управление балансировщиками;
    * создание сетевых маршрутов для сети `PodNetwork`;
    * управление дисками.

С модулем взаимодействуют следующие внешние компоненты:

* **Prometheus-main** — сбор метрик cloud-data-discoverer.

Непрямые взаимодействия:

1. Модуль `cloud-provider-gcp` предоставляет модулю [`node-manager`](/modules/node-manager/) следующие артефакты:

   * шаблоны для создания кастомных ресурсов для конкретного провайдера, которые `cloud-provider-gcp` использует для создания виртуальных машин в облаке;
   * секрет `kube-system/d8-node-manager-cloud-provider`, в котором содержатся все необходимые настройки для подключения к облаку и создания CloudEphemeral-узлов. Эти настройки прописываются в кастомных ресурсах, созданных на основе упомянутых выше шаблонов и учитывающих особенности провайдера.

1. Модуль `cloud-provider-gcp` предоставляет компоненты Terraform/OpenTofu для GCP, которые используются при сборке исполняемого файла утилиты [`dhctl`](https://github.com/deckhouse/deckhouse/tree/main/dhctl) в модуле [`terraform-manager`](/modules/terraform-manager/), такие как:

   * Terraform/OpenTofu-провайдер;
   * Terraform-модули;
   * layouts — набор схем размещения в облаке, определяющих, как создается базовая инфраструктура, как и с какими дополнительными характеристиками для данного размещения должны создаваться узлы. Например, в одной схеме узлы могут иметь публичные IP-адреса, а в другой — нет. Каждая схема включает три модуля:

     * `base-infrastructure` — базовая инфраструктура (например, создание сетей), может быть пустым;
     * `master-node`;
     * `static-node`.
