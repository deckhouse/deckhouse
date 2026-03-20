---
title: Модуль cloud-provider-vcd
permalink: ru/architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-vcd.html
lang: ru
search: cloud-provider-vcd, cloud provider vcd, vmware cloud director
description: Архитектура модуля cloud-provider-vcd в Deckhouse Kubernetes Platform.
---

Модуль `cloud-provider-vcd` управляет взаимодействием с облачными ресурсами [VMware Cloud Director](https://www.vmware.com/products/cloud-infrastructure/cloud-director). Он позволяет модулю [`node-manager`](/modules/node-manager/) использовать ресурсы VMware Cloud Director при заказе узлов для описанной [группы узлов](/modules/node-manager/cr.html#nodegroup).

Подробнее с описанием модуля можно ознакомиться в [соответствующем разделе документации](/modules/cloud-provider-vcd/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`cloud-provider-vcd`](/modules/cloud-provider-vcd/) на уровне 2 модели C4 и его взаимодействия с другими компонентами Deckhouse Kubernetes Platform (DKP) изображены на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура модуля cloud-provider-vcd](../../../../images/architecture/cluster-and-infrastructure/c4-l2-cloud-provider-vcd.ru.png)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Capcd-controller-manager** — [Kubernetes Cluster API Provider Cloud Director](https://github.com/vmware-archive/cluster-api-provider-cloud-director). [Cluster API](https://github.com/kubernetes-sigs/cluster-api) является расширением для Kubernetes, которое дает возможность управлять Kubernetes-кластерами как кастомными ресурсами внутри другого Kubernetes-кластера. Cluster API Provider позволяет для кластеров под управлением Cluster API заказывать виртуальные машины в инфраструктуре облачного провайдера, в данном случае VMware Cloud Director. Capcd-controller-manager работает со следующими кастомными ресурсами:

   * VCDClusterTemplate — шаблон с описанием инфраструктурных настроек создаваемого кластера (Control Plane endpoint, Loadbalancer, VCD-специфичные настройки);
   * VCDCluster — описание созданного на основе VCDClusterTemplate кластера;
   * VCDMachineTemplate — шаблон с описанием характеристик создаваемых машин в облаке;
   * VCDMachine — описание характеристик созданной на основе VCDMachineTemplate машины.

   Состоит из одного контейнера:

   * **capcd-controller-manager**.

2. **Сloud-controller-manager** — [Kubernetes External Cloud Provider for VMware Cloud Director](https://github.com/vmware-archive/cloud-provider-for-cloud-director), реализация [Сloud сontroller manager](https://kubernetes.io/ru/docs/concepts/architecture/cloud-controller/) для VMware Cloud Director. Обеспечивает взаимодействие с облаком VMware Cloud Director и выполняет следующие функции:

   * реализует связь 1:1 между объектом узла в Kubernetes (Node) и виртуальной машиной в облачном провайдере. Для этого:

     * заполняет поля `spec.providerID` и `nodeInfo` ресурса Node;
     * проверяет наличие виртуальной машины в облаке и при ее отсутствии удаляет ресурс Node в кластере.

   * при создании ресурса Service типа LoadBalancer в Kubernetes создаёт балансировщик в облаке, который направляет трафик извне к узлам кластера.

   Подробнее о cloud-controller-manager можно почитать в [документации Kubernetes](https://kubernetes.io/ru/docs/concepts/architecture/cloud-controller/)

   Состоит из одного контейнера:

   * **vcd-cloud-controller-manager**.

3. **Cloud-data-discoverer** — отвечает за сбор данных из API облачного провайдера и предоставление их в виде секрета `kube-system/d8-cloud-provider-discovery-data`. Этот секрет содержит параметры конкретного облака, которые используется другими компонентами модуля `cloud-provider-vcd`. Например, для VMware Cloud Director — это такие параметры, как StorageProfiles, InternalNetworks, версия VCD и т.д.

   Состоит из следующих контейнеров:

   * **cloud-data-discoverer** — основной контейнер;
   * **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищенного доступа к метрикам контейнера cloud-data-discoverer.

4. **Infra-controller-manager** — отвечает за контроль расположения узлов друг относительно друга на уровне гипервизоров, что может повысить отказоустойчивость и контролировать распределение рабочих нагрузок. Infra-controller-manager работает с кастомным ресурсом [VCDAffinityRule](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass-v1-spec-affinityrule), который описывает правила размещения ресурсов в VMware Cloud Director.

   Состоит из одного контейнера:

   * **infra-controller-manager**.

5. **CSI-драйвер (VCD)** — реализация CSI-драйвера для VMware Cloud Director. С типовой архитектурой CSI-драйвера, используемого в модулях `cloud-provider-*` DKP, можно ознакомиться на [соответствующей странице документации](../infrastructure/csi-driver.html). В модуле cloud-provider-vcd используется [CSI driver for VMware Cloud Director Named Independent Disks](https://github.com/vmware-archive/cloud-director-named-disk-csi-driver).

   CSI-драйвер (VCD) не поддерживает работу со снимками. По этой причине в поде `csi-controller` отсутствует сайдкар-контейнер snapshotter ([external-snapshotter](https://github.com/kubernetes-csi/external-snapshotter)).

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

    * мониторинг ресурсов PersistentVolumeClaim, VolumeAttachment;
    * работа с кастомными ресурсами VCDClusterTemplate, VCDCluster, VCDMachineTemplate, VCDMachine, VCDAffinityRule;
    * создание секрета `kube-system/d8-cloud-provider-discovery-data`;
    * синхронизация узлов Kubernetes с виртуальными машинами в облаке;
    * мониторинг сервисов типа LoadBalancer;
    * авторизация запросов на получение метрик.

2. **VMware Cloud Director**:

    * получение параметров облака;
    * управление виртуальными машинами;
    * применение правил Affinity Rules к виртуальным машинам;
    * получение `ProviderID` и прочей информации о виртуальных машинах, которые являются узлами кластера;
    * управление балансировщиками;
    * управление дисками.

С модулем взаимодействуют следующие внешние компоненты:

1. **Prometheus-main** — сбор метрик cloud-data-discoverer.

Непрямые взаимодействия:

1. Модуль `cloud-provider-vcd` предоставляет модулю [`node-manager`](/modules/node-manager/) следующие артефакты:

   * шаблоны для создания кастомных ресурсов Cluster API для конкретного провайдера, которые `cloud-provider-vcd` использует для создания виртуальных машин в облаке;
   * секрет `kube-system/d8-node-manager-cloud-provider`, в котором содержатся все необходимые настройки для подключения к облаку и создания CloudEphemeral-узлов. Эти настройки прописываются в кастомных ресурсах Cluster API, созданных на основе упомянутых выше шаблонов и учитывающих особенности провайдера.

2. Модуль `cloud-provider-vcd` предоставляет компоненты Terraform/OpenTofu для VMware Cloud Director, которые используются при сборке исполняемого файла утилиты [dhctl](https://github.com/deckhouse/deckhouse/tree/main/dhctl) в модуле [`terraform-manager`](/modules/terraform-manager/), такие как:

   * Terraform/OpenTofu-провайдер;
   * Terraform-модули;
   * layouts — набор схем размещения в облаке, определяющих, как создается базовая инфраструктура, как и с какими дополнительными характеристиками для данного размещения должны создаваться узлы. Например, в одной схеме узлы могут иметь публичные IP-адреса, а в другой — нет. Каждый layout включает три модуля:

     * `base-infrastructure` — базовая инфраструктура (например, создание сетей), может быть пустым;
     * `master-node`;
     * `static-node`.
