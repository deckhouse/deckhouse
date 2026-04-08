---
title: Модуль csi-vsphere
permalink: ru/architecture/storage/csi-vsphere.html
lang: ru
search: csi-vsphere, vmware vsphere
description: Архитектура модуля csi-vsphere в Deckhouse Kubernetes Platform.
---

Модуль [`csi-vsphere`](/modules/csi-vsphere/) предоставляет поддержку [Container Storage Interface (CSI)](https://github.com/container-storage-interface/spec/blob/master/spec.md) для сред VMware vSphere, обеспечивая динамическое предоставление и управление постоянными томами хранения в кластерах Kubernetes, работающих на инфраструктуре vSphere.

Подробнее с описанием модуля можно ознакомиться [в соответствующем разделе документации](/modules/csi-vsphere/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`csi-vsphere`](/modules/csi-vsphere/) на уровне 2 модели C4 и его взаимодействия с другими компонентами Deckhouse Kubernetes Platform (DKP) изображены на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура модуля csi-vsphere](../../../../images/architecture/cluster-and-infrastructure/c4-l2-csi-vsphere.ru.png)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Cloud-data-discoverer** — отвечает за сбор данных из API VMware vSphere и предоставление их в виде секрета `kube-system/d8-cloud-provider-discovery-data`. Этот секрет содержит параметры конкретного облака, которые используется CSI-драйвером для управления томами.

   Состоит из следующих контейнеров:

   * **cloud-data-discoverer** — основной контейнер;
   * **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищенного доступа к метрикам контейнера cloud-data-discoverer.

1. **CSI-драйвер (vsphere)** — реализация CSI-драйвера для VMware vSphere. С архитектурой CSI-драйвера, используемого в модуле `csi-vsphere` DKP, можно ознакомиться в [соответствующем разделе документации](../storage/csi-drivers/csi-vsphere.html).

   CSI-драйвер (vsphere) не поддерживает работу со снимками. По этой причине в поде `csi-controller` отсутствует сайдкар-контейнер snapshotter ([external-snapshotter](https://github.com/kubernetes-csi/external-snapshotter)).

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

    * мониторинг ресурсов PersistentVolumeClaim, VolumeAttachment;
    * создание секрета `kube-system/d8-cloud-provider-discovery-data`;
    * создание StorageClass на основе результатов обнаружения;
    * авторизация запросов на получение метрик.

1. **VMware vSphere**:

    * получение параметров облака;
    * управление дисками.

С модулем взаимодействуют следующие внешние компоненты:

* **Prometheus-main** — сбор метрик cloud-data-discoverer.
