---
title: Модуль cloud-provider-vsphere
permalink: ru/architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-vsphere.html
lang: ru
search: cloud-provider-vsphere, cloud provider vsphere, vmware vsphere
description: Архитектура модуля cloud-provider-vsphere в Deckhouse Kubernetes Platform.
---

Модуль [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/) обеспечивает интеграцию с облачными ресурсами [VMware vSphere](https://www.vmware.com/products/cloud-infrastructure/vsphere). Он используется модулем [`node-manager`](/modules/node-manager/) для заказа узлов в соответствии [с настройками группы узлов](/modules/node-manager/cr.html#nodegroup).

Подробнее с описанием модуля можно ознакомиться [в соответствующем разделе документации](/modules/cloud-provider-vsphere/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/) на уровне 2 модели C4 и его взаимодействия с другими компонентами Deckhouse Kubernetes Platform (DKP) изображены на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура модуля cloud-provider-vsphere](../../../../images/architecture/cluster-and-infrastructure/c4-l2-cloud-provider-vsphere.ru.png)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Сloud-controller-manager** — [Kubernetes vSphere Cloud Provider](https://github.com/kubernetes/cloud-provider-vsphere), реализация [cloud сontroller manager](https://kubernetes.io/ru/docs/concepts/architecture/cloud-controller/) для VMware vSphere. Компонент обеспечивает интеграцию с облачными ресурсами на базе VMware vSphere и выполняет следующие функции:

   * реализует связь 1:1 между объектом узла в Kubernetes (Node) и виртуальной машиной в облачном провайдере. Для этого:

     * заполняет поля `spec.providerID` и `nodeInfo` ресурса Node;
     * проверяет наличие виртуальной машины в облаке и при ее отсутствии удаляет ресурс Node в кластере.

   * при создании ресурса Service типа LoadBalancer в Kubernetes создаёт балансировщик в облаке, который направляет трафик извне к узлам кластера.

   Подробнее о cloud-controller-manager можно почитать [в документации Kubernetes](https://kubernetes.io/ru/docs/concepts/architecture/cloud-controller/).

   Состоит из одного контейнера:

   * **vsphere-cloud-controller-manager**.

1. **Cloud-data-discoverer** — отвечает за сбор данных из API облачного провайдера и предоставление их в виде секрета `kube-system/d8-cloud-provider-discovery-data`. Этот секрет содержит параметры конкретного облака, которые используется другими компонентами модуля `cloud-provider-vsphere`.

   Состоит из следующих контейнеров:

   * **cloud-data-discoverer** — основной контейнер;
   * **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищенного доступа к метрикам контейнера cloud-data-discoverer.

1. **CSI-драйвер (vsphere)** — реализация CSI-драйвера для VMware vSphere. С архитектурой CSI-драйвера, используемого в модуле `cloud-provider-vsphere` DKP, можно ознакомиться в [соответствующем разделе документации](../infrastructure/csi-vsphere.html).

   CSI-драйвер (vsphere) не поддерживает работу со снимками. По этой причине в поде `csi-controller` отсутствует сайдкар-контейнер snapshotter ([external-snapshotter](https://github.com/kubernetes-csi/external-snapshotter)).

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

    * мониторинг ресурсов PersistentVolumeClaim, VolumeAttachment;
    * создание секрета `kube-system/d8-cloud-provider-discovery-data`;
    * синхронизация узлов Kubernetes с виртуальными машинами в облаке;
    * мониторинг сервисов типа LoadBalancer;
    * авторизация запросов на получение метрик.

1. **VMware vSphere**:

    * получение параметров облака;
    * получение `ProviderID` и прочей информации о виртуальных машинах, которые являются узлами кластера;
    * управление балансировщиками;
    * управление дисками.

С модулем взаимодействуют следующие внешние компоненты:

* **Prometheus-main** — сбор метрик cloud-data-discoverer.

Непрямые взаимодействия:

1. Модуль `cloud-provider-vsphere` предоставляет модулю [`node-manager`](/modules/node-manager/) следующие артефакты:

   * шаблоны для создания кастомных ресурсов для конкретного провайдера, которые `cloud-provider-vsphere` использует для создания виртуальных машин в облаке;
   * секрет `kube-system/d8-node-manager-cloud-provider`, в котором содержатся все необходимые настройки для подключения к облаку и создания CloudEphemeral-узлов. Эти настройки прописываются в кастомных ресурсах, созданных на основе упомянутых выше шаблонов и учитывающих особенности провайдера.

2. Модуль `cloud-provider-vsphere` предоставляет компоненты Terraform/OpenTofu для VMware vSphere, которые используются при сборке исполняемого файла утилиты [`dhctl`](https://github.com/deckhouse/deckhouse/tree/main/dhctl) в модуле [`terraform-manager`](/modules/terraform-manager/), такие как:

   * Terraform/OpenTofu-провайдер;
   * Terraform-модули;
   * layouts — набор схем размещения в облаке, определяющих, как создается базовая инфраструктура, как и с какими дополнительными характеристиками для данного размещения должны создаваться узлы. Например, в одной схеме узлы могут иметь публичные IP-адреса, а в другой — нет. Каждая схема включает три модуля:

     * `base-infrastructure` — базовая инфраструктура (например, создание сетей), может быть пустым;
     * `master-node`;
     * `static-node`.
