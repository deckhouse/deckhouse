---
title: Модуль cloud-provider-aws
permalink: ru/architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-aws.html
lang: ru
search: cloud-provider-aws, cloud provider aws, amazon web services
description: Архитектура модуля cloud-provider-aws в Deckhouse Kubernetes Platform.
---

Модуль [`cloud-provider-aws`](/modules/cloud-provider-aws/) обеспечивает интеграцию с облачными ресурсами [Amazon Web Services](https://aws.amazon.com/). Он используется модулем [`node-manager`](/modules/node-manager/) для заказа узлов в соответствии [с настройками группы узлов](/modules/node-manager/cr.html#nodegroup).

Подробнее с описанием модуля можно ознакомиться [в соответствующем разделе документации](/modules/cloud-provider-aws/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`cloud-provider-aws`](/modules/cloud-provider-aws/) на уровне 2 модели C4 и его взаимодействия с другими компонентами Deckhouse Kubernetes Platform (DKP) изображены на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура модуля cloud-provider-aws](../../../../images/architecture/cluster-and-infrastructure/c4-l2-cloud-provider-aws.ru.png)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Cloud-controller-manager** — реализация [cloud controller manager](https://kubernetes.io/ru/docs/concepts/architecture/cloud-controller/) для AWS. Компонент обеспечивает интеграцию с облаком AWS и выполняет следующие функции:

   * реализует связь 1:1 между объектом узла в Kubernetes (Node) и виртуальной машиной в облачном провайдере. Для этого:

     * заполняет поля `spec.providerID` и `nodeInfo` ресурса Node;
     * проверяет наличие виртуальной машины в облаке и при ее отсутствии удаляет ресурс Node в кластере.

   * при создании ресурса Service типа LoadBalancer в Kubernetes создаёт балансировщик в облаке, который направляет трафик извне к узлам кластера;
   * создает сетевые маршруты для сети `PodNetwork` на стороне AWS.

   Подробнее о cloud-controller-manager можно почитать [в документации Kubernetes](https://kubernetes.io/ru/docs/concepts/architecture/cloud-controller/).

   Состоит из одного контейнера:

   * **aws-cloud-controller-manager**.

1. **Cloud-data-discoverer** — отвечает за сбор данных из API облачного провайдера и предоставление их в виде секрета `kube-system/d8-cloud-provider-discovery-data`. Этот секрет содержит параметры конкретного облака, которые используются другими компонентами модуля `cloud-provider-aws`.

   Состоит из следующих контейнеров:

   * **cloud-data-discoverer** — основной контейнер;
   * **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищенного доступа к метрикам контейнера cloud-data-discoverer.

1. **CSI-драйвер (aws)** — реализация CSI-драйвера для AWS. С типовой архитектурой CSI-драйвера, используемого в модулях `cloud-provider-*` DKP, можно ознакомиться в [соответствующем разделе документации](../infrastructure/csi-driver.html).

1. **Node-termination-handler** — [AWS Node Termination Handler](https://github.com/aws/aws-node-termination-handler), отвечает за обработку DKP событий от сервисов AWS о недоступности экземпляров EC2.

   Node-termination-handler обрабатывает следующие события AWS:
   * [EC2 maintenance events](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/monitoring-instances-status-check_sched.html);
   * [EC2 Spot interruptions](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-interruptions.html);
   * [ASG Scale-In](https://docs.aws.amazon.com/autoscaling/ec2/userguide/AutoScalingGroupLifecycle.html#as-lifecycle-scale-in);
   * [ASG AZ Rebalance](https://docs.aws.amazon.com/autoscaling/ec2/userguide/ec2-auto-scaling-capacity-rebalancing.html);
   * [EC2 Instance Termination](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/terminating-instances.html).

   При получении указанных событий node-termination-handler выполняет drain/cordon для соответствующего узла.

   Состоит из одного контейнера:

   * **node-termination-handler**.

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

    * мониторинг ресурсов PersistentVolumeClaim, VolumeAttachment;
    * создание секрета `kube-system/d8-cloud-provider-discovery-data`;
    * синхронизация узлов Kubernetes с виртуальными машинами в облаке;
    * мониторинг сервисов типа LoadBalancer;
    * авторизация запросов на получение метрик.

1. **Amazon Web Services**:

    * получение параметров облака;
    * получение `ProviderID` и прочей информации о виртуальных машинах, которые являются узлами кластера;
    * управление балансировщиками;
    * создание сетевых маршрутов для сети `PodNetwork`;
    * получение событий о недоступности экземпляров EC2;
    * управление дисками.

С модулем взаимодействуют следующие внешние компоненты:

* **Prometheus-main** — сбор метрик cloud-data-discoverer.

Непрямые взаимодействия:

1. Модуль `cloud-provider-aws` предоставляет модулю [`node-manager`](/modules/node-manager/) следующие артефакты:

   * шаблоны для создания кастомных ресурсов для конкретного провайдера, которые `cloud-provider-aws` использует для создания виртуальных машин в облаке;
   * секрет `kube-system/d8-node-manager-cloud-provider`, в котором содержатся все необходимые настройки для подключения к облаку и создания CloudEphemeral-узлов. Эти настройки прописываются в кастомных ресурсах, созданных на основе упомянутых выше шаблонов и учитывающих особенности провайдера.

1. Модуль `cloud-provider-aws` предоставляет компоненты Terraform/OpenTofu для AWS, которые используются при сборке исполняемого файла утилиты [`dhctl`](https://github.com/deckhouse/deckhouse/tree/main/dhctl) в модуле [`terraform-manager`](/modules/terraform-manager/), такие как:

   * Terraform/OpenTofu-провайдер;
   * Terraform-модули;
   * layouts — набор схем размещения в облаке, определяющих, как создается базовая инфраструктура, как и с какими дополнительными характеристиками для данного размещения должны создаваться узлы. Например, в одной схеме узлы могут иметь публичные IP-адреса, а в другой — нет. Каждая схема включает три модуля:

     * `base-infrastructure` — базовая инфраструктура (например, создание сетей), может быть пустым;
     * `master-node`;
     * `static-node`.
