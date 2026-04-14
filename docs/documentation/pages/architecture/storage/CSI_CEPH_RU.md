---
title: Модуль csi-ceph
permalink: ru/architecture/storage/csi-ceph.html
lang: ru
search: csi-ceph, ceph, cephfs, rbd
description: Архитектура модуля csi-ceph в Deckhouse Kubernetes Platform.
---

Модуль `csi-ceph` предназначен для  интеграции DKP с Ceph-кластерами и обеспечивает управление хранилищем на основе [RBD (RADOS Block Device)](https://docs.ceph.com/en/reef/rbd/) или [CephFS](https://docs.ceph.com/en/reef/cephfs/). Он позволяет создавать StorageClass в Kubernetes с помощью ресурса CephStorageClass.

Подробнее с описанием модуля можно ознакомиться [в разделе документации модуля](/modules/csi-ceph/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`csi-ceph`](/modules/csi-ceph/) на уровне 2 модели C4 и его взаимодействия с другими компонентами Deckhouse Kubernetes Platform (DKP) изображены на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура модуля csi-ceph](../../../images/architecture/storage/c4-l2-csi-ceph.ru.png)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Controller** — контроллер, обслуживающий следующие [кастомные ресурсы](/modules/csi-ceph/stable/cr.html):

    * CephClusterAuthentication — параметры аутентификации кластера Ceph;
    * CephClusterConnection — параметры подключения к кластеру Ceph;
    * CephMetadataBackup — резервная копия метаданных Persistent Volume;
    * CephStorageClass —  определяет конфигурацию для Kubernetes StorageClass.

    В CephStorageClass задается тип storage-класса (`CephFS`, `RBD`), reclaim policy, параметры аутентификации кластера Ceph, параметры подключения к кластеру Ceph, а так же специфичные для каждого storage-класса дополнительные параметры. В зависимости от типа storage-класса эти параметры используются provisioner’ом CSI-драйвера `rbd.csi.ceph.com` или `cephfs.csi.ceph.com` при управлении томами.

   Состоит из следующих контейнеров:

   * **controller** — основной контейнер;
   * **webhook** — сайдкар-контейнер, реализующий вебхук-сервер для проверки стандартных ресурсов StorageClass.

1. **CSI-драйвер (`rbd/cephfs`)** — реализация CSI-драйвера для `rbd.csi.ceph.com` или `cephfs.csi.ceph.com` provisioner. Выбор CSI-драйвера выполняется путём задания storage-класса в кастомном ресурсе CephStorageClass.

CSI-драйвер (`cephfs`) реализован по типовой архитектуре CSI-драйвера, используемого в DKP, можно ознакомиться [в разделе документации архитектуры CSI-драйвера](../../cluster-and-infrastructure/infrastructure/csi-driver.html).

CSI-драйвер (`rbd`) реализован по отличной от типовой архитектуры CSI-драйвера, которая приведена [в разделе документации CSI-драйвера](../../storage/csi-drivers/csi-driver-ceph-rbd.html).

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

* **Kube-apiserver**:

  * мониторинг ресурсов PersistentVolume, PersistentVolumeClaim, VolumeAttachment, StorageClass;
  * работа с кастомными ресурсами CephClusterAuthentication, CephClusterConnection, CephMetadataBackup, CephStorageClass;
  * создание ресурса StorageClass.

С модулем взаимодействуют следующие внешние компоненты:

* **Kube-apiserver** — валидация стандартных ресурсов StorageClass.
