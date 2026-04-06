---
title: Модуль csi-s3
permalink: ru/architecture/storage/csi-s3.html
lang: ru
search: csi-s3, s3
description: Архитектура модуля csi-s3 в Deckhouse Kubernetes Platform.
---

Модуль [`csi-s3`](/modules/csi-s3/) предназначен для управления томами на основа `S3-хранилищ`. Он использует [geeseFS](https://github.com/yandex-cloud/geesefs) — файловую систему FUSE на основе S3. Модуль позволяет создавать StorageClass в Kubernetes с помощью ресурса S3StorageClass.

Подробнее с описанием модуля можно ознакомиться [в разделе документации модуля](/modules/csi-s3/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`csi-s3`](/modules/csi-s3/) на уровне 2 модели C4 и его взаимодействия с другими компонентами Deckhouse Kubernetes Platform (DKP) изображены на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура модуля csi-s3](../../../images/architecture/storage/c4-l2-csi-s3.ru.png)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Controller** — контроллер, обслуживающий кастомный ресурс [S3StorageClass](/modules/csi-s3/stable/cr.html). В S3StorageClass задаются параметры подключения к S3-хранилищу, такие как: URL подключения, имя используемаого bucket, регион и токены доступа.

   Состоит из следующих контейнеров:

   * **controller** — основной контейнер;
   * **webhook** — сайдкар-контейнер, реализующий вебхук-сервер для проверки кастомного ресурса S3StorageClass.

1. **CSI-драйвер (s3)** — реализация CSI-драйвера для `s3.csi.k8s.io` provisioner. С архитектурой CSI-драйвера `csi-s3` можно ознакомиться [в соответствющем разделе документации](../storage/csi-drivers/csi-driver-s3.html).

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

* **Kube-apiserver**:

  * мониторинг ресурсов PersistentVolume, PersistentVolumeClaim, VolumeAttachment, StorageClass;
  * работа с кастомными ресурсами S3StorageClass;
  * создание ресурса StorageClass.

С модулем взаимодействуют следующие внешние компоненты:

* **Kube-apiserver** — валидация кастомных ресурсов S3StorageClass.
