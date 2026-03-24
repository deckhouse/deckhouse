---
title: Модуль cloud-provider-huaweicloud
permalink: ru/architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-huaweicloud.html
lang: ru
search: cloud-provider-huaweicloud, cloud provider huaweicloud
description: Архитектура модуля cloud-provider-huaweicloud в Deckhouse Kubernetes Platform.
---

Модуль `cloud-provider-huaweicloud` управляет взаимодействием с облачными ресурсами [Huawei Cloud](https://www.huaweicloud.com/intl/en-us/). Он позволяет модулю [`node-manager`](/modules/node-manager/) использовать ресурсы Huawei Cloud при заказе узлов для описанной [группы узлов](/modules/node-manager/cr.html#nodegroup).

Подробнее с описанием модуля можно ознакомиться в [соответствующем разделе документации](/modules/cloud-provider-huaweicloud/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`cloud-provider-huaweicloud`](/modules/cloud-provider-huaweicloud/) на уровне 2 модели C4 и его взаимодействия с другими компонентами Deckhouse Kubernetes Platform (DKP) изображены на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура модуля cloud-provider-huaweicloud](../../../../images/architecture/cluster-and-infrastructure/c4-l2-cloud-provider-huaweicloud.ru.png)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Caphc-controller-manager** — Kubernetes Cluster API Provider для Huawei Cloud. [Cluster API](https://github.com/kubernetes-sigs/cluster-api) является расширением для Kubernetes, которое дает возможность управлять Kubernetes-кластерами как кастомными ресурсами внутри другого Kubernetes-кластера. Cluster API Provider позволяет для кластеров под управлением Cluster API заказывать виртуальные машины в инфраструктуре облачного провайдера, в данном случае Huawei Cloud. Caphc-controller-manager работает со следующими кастомными ресурсами:

   * HuaweiCloudCluster — описание кластера на базе Huawei Cloud;
   * HuaweiCloudMachineTemplate — шаблон с описанием характеристик создаваемых машин в облаке;
   * HuaweiCloudMachine — описание характеристик созданной на основе HuaweiCloudMachineTemplate машины.

   Состоит из одного контейнера:

   * **caphc-controller-manager**.

2. **Сloud-controller-manager** — реализация [Сloud сontroller manager](https://kubernetes.io/ru/docs/concepts/architecture/cloud-controller/) для Huawei Cloud. Обеспечивает взаимодействие с облаком Huawei Cloud и выполняет следующие функции:

   * реализует связь 1:1 между объектом узла в Kubernetes (Node) и виртуальной машиной в облачном провайдере. Для этого:

     * заполняет поля `spec.providerID` и `nodeInfo` ресурса Node;
     * проверяет наличие виртуальной машины в облаке и при ее отсутствии удаляет ресурс Node в кластере.

   * при создании ресурса Service типа LoadBalancer в Kubernetes создаёт балансировщик в облаке, который направляет трафик извне к узлам кластера.

   Подробнее о cloud-controller-manager можно почитать в [документации Kubernetes](https://kubernetes.io/ru/docs/concepts/architecture/cloud-controller/).

   Состоит из одного контейнера:

   * **huaweicloud-cloud-controller-manager**.

3. **Cloud-data-discoverer** — отвечает за сбор данных из API облачного провайдера и предоставление их в виде секрета `kube-system/d8-cloud-provider-discovery-data`. Этот секрет содержит параметры конкретного облака, которые используется другими компонентами модуля `cloud-provider-huaweicloud`.

   Состоит из следующих контейнеров:

   * **cloud-data-discoverer** — основной контейнер;
   * **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищенного доступа к метрикам контейнера cloud-data-discoverer.

4. **CSI-драйвер (huaweicloud)** — реализация CSI-драйвера для Huawei Cloud. С типовой архитектурой CSI-драйвера, используемого в модулях `cloud-provider-*` DKP, можно ознакомиться на [соответствующей странице документации](../infrastructure/csi-driver.html).

   CSI-драйвер (huaweicloud) не поддерживает работу со снимками. По этой причине в поде `csi-controller` отсутствует сайдкар-контейнер snapshotter ([external-snapshotter](https://github.com/kubernetes-csi/external-snapshotter)).

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

    * мониторинг ресурсов PersistentVolumeClaim, VolumeAttachment;
    * работа с кастомными ресурсами HuaweiCloudCluster, HuaweiCloudMachineTemplate, HuaweiCloudMachine;
    * создание секрета `kube-system/d8-cloud-provider-discovery-data`;
    * синхронизация узлов Kubernetes с виртуальными машинами в облаке;
    * мониторинг сервисов типа LoadBalancer;
    * авторизация запросов на получение метрик.

2. **Huawei Cloud**:

    * получение параметров облака;
    * управление виртуальными машинами;
    * получение `ProviderID` и прочей информации о виртуальных машинах, которые являются узлами кластера;
    * управление балансировщиками;
    * управление дисками.

С модулем взаимодействуют следующие внешние компоненты:

1. **Prometheus-main** — сбор метрик cloud-data-discoverer.

Непрямые взаимодействия:

1. Модуль `cloud-provider-huaweicloud` предоставляет модулю [`node-manager`](/modules/node-manager/) следующие артефакты:

   * шаблоны для создания кастомных ресурсов Cluster API для конкретного провайдера, которые `cloud-provider-huaweicloud` использует для создания виртуальных машин в облаке;
   * секрет `kube-system/d8-node-manager-cloud-provider`, в котором содержатся все необходимые настройки для подключения к облаку и создания CloudEphemeral-узлов. Эти настройки прописываются в кастомных ресурсах Cluster API, созданных на основе упомянутых выше шаблонов и учитывающих особенности провайдера.

2. Модуль `cloud-provider-huaweicloud` предоставляет компоненты Terraform/OpenTofu для Huawei Cloud, которые используются при сборке исполняемого файла утилиты [dhctl](https://github.com/deckhouse/deckhouse/tree/main/dhctl) в модуле [`terraform-manager`](/modules/terraform-manager/), такие как:

   * Terraform/OpenTofu-провайдер;
   * Terraform-модули;
   * layouts — набор схем размещения в облаке, определяющих, как создается базовая инфраструктура, как и с какими дополнительными характеристиками для данного размещения должны создаваться узлы. Например, в одной схеме узлы могут иметь публичные IP-адреса, а в другой — нет. Каждый layout включает три модуля:

     * `base-infrastructure` — базовая инфраструктура (например, создание сетей), может быть пустым;
     * `master-node`;
     * `static-node`.
