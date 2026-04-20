---
title: Модуль csi-netapp
permalink: ru/architecture/storage/external/csi-netapp.html
lang: ru
search: csi-netapp, netapp
description: Архитектура модуля csi-netapp в Deckhouse Kubernetes Platform.
---

Модуль [`csi-netapp`](/modules/csi-netapp/) предназначен для управления томами c использованием систем хранения данных NetApp. Он позволяет создавать StorageClass в Kubernetes с помощью ресурса NetappStorageClass.

Подробнее с описанием модуля можно ознакомиться [в разделе документации модуля](/modules/csi-netapp/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`csi-netapp`](/modules/csi-netapp/) на уровне 2 модели C4 и его взаимодействия с другими компонентами Deckhouse Kubernetes Platform (DKP) изображены на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура модуля csi-netapp](../../../images/architecture/storage/c4-l2-csi-netapp.ru.png)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Controller** — контроллер, обслуживающий следующие [кастомные ресурсы](/modules/csi-netapp/cr.html):

    * NetappStorageConnection — параметры подключения к СХД NetApp;
    * NetappStorageClass — определяет конфигурацию для создания Kubernetes StorageClass, который использует provisioner `csi.trident.netapp.io`.

    Также controller синхронизирует метку `storage.deckhouse.io/csi-netapp-node` для узлов кластера в соответствии со значением селектора узлов [`spec.settings.nodeSelector`](/modules/csi-netapp/configuration.html) кастомного ресурса ModuleConfig.

    Состоит из одного основного контейнера **controller**.

1. **CSI-драйвер (netapp)** — реализация CSI-драйвера для `csi.trident.netapp.io` provisioner. С типовой архитектурой CSI-драйвера, используемого в DKP, можно ознакомиться [в разделе документации архитектуры CSI-драйвера](../../cluster-and-infrastructure/infrastructure/csi-driver.html).

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

    * мониторинг ресурсов PersistentVolume, PersistentVolumeClaim, VolumeAttachment и StorageClass;
    * работа с кастомными ресурсами TridentBackendConfig, NetappStorageConnection и NetappStorageClass;
    * создание ресурса VolumeSnapshotClass, Secret и StorageClass.

1. **СХД NetApp** — создание, удаление и управление томами, а также подключение и отключение томов от узлов.
