---
title: Модуль sds-node-configurator
permalink: ru/architecture/storage/sds-node-configurator.html
lang: ru
search: sds-node-configurator, lvm, block device, блочное устройство, volume group, группа томов, thin pool, thin том, thin volume, logical volume, логический том
description: Архитектура модуля sds-node-configurator в Deckhouse Kubernetes Platform.
---

Модуль `sds-node-configurator` управляет LVM на узлах кластера через кастомные ресурсы Kubernetes, выполняя следующие операции:

* Автоматическое обнаружение блочных устройств и создание/обновление/удаление соответствующих им ресурсов [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice).
* Автоматическое обнаружение на узлах групп томов LVM с LVM-тегом `storage.deckhouse.io/enabled=true` и thin pool на них, а также управление соответствующими ресурсами [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup). Модуль автоматически создает ресурс [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup), если его еще не существует для обнаруженной группы томов LVM.
* Сканирование на узлах физических томов LVM, входящих в управляемые группы томов LVM. При расширении базовых блочных устройств соответствующие физические тома LVM автоматически увеличиваются (выполняется pvresize).
* Создание/расширение/удаление групп томов LVM на узле в соответствии с настройками ресурсов [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup).

Подробнее с описанием модуля можно ознакомиться в [в разделе документации модуля](/modules/sds-node-configurator/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`sds-node-configurator`](/modules/sds-node-configurator/) на уровне 2 модели C4 и его взаимодействия с другими компонентами DKP изображены на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура модуля sds-node-configurator](../../../images/architecture/storage/c4-l2-sds-node-configurator.ru.png)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Sds-node-configurator** (DaemonSet) — контроллер, запущенный на узлах кластера и выполняющий перечисленные выше операции с кастомными ресурсами BlockDevice, LVMVolumeGroup, LVMLogicalVolume, LVMLogicalVolumeSnapshot и т.д. С описанием всех кастомных ресурсов, управляемых модулем, можно ознакомиться на [соответствующей странице](/modules/sds-node-configurator/cr.html).

   Состоит из следующих контейнеров:

   * **sds-utils-installer** — init-контейнер, устанавливающий набор утилит, необходимых для управления блочными устройствами и LVM-томами;
   * **thin-volumes-enabler** — init-контейнер, включающий поддержку thin томов;
   * **sds-node-configurator-agent** — основной контейнер;
   * **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищенного доступа к метрикам контроллера. Является [Open Source-проектом](https://github.com/brancz/kube-rbac-proxy).

2. **Controller** (Deployment) — контроллер, следящий за кастомными ресурсами, относящимися к блочным устройствам и LVM. Controller работает с метаданными ресурсов и обновляет их статусы.

   Состоит из следующих контейнеров:

   * **controller** — основной контейнер;
   * **webhook** — сайдкар-контейнер, реализующий вебхук-сервер для проверки кастомных ресурсов [LVMLogicalVolumeSnapshot](/modules/sds-node-configurator/cr.html#lvmlogicalvolumesnapshot). Если используемая редакция DKP не поддерживает функционал снимков логических томов LVM, кастомный ресурс [LVMLogicalVolumeSnapshot](/modules/sds-node-configurator/cr.html#lvmlogicalvolumesnapshot) не проходит валидацию.

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

   * работа с кастомными ресурсами, относящимися к блочным устройствам и LVM;
   * авторизация запросов на метрики.

С модулем взаимодействуют следующие внешние компоненты:

1. **Kube-apiserver** - валидация кастомных ресурсов [LVMLogicalVolumeSnapshot](/modules/sds-node-configurator/cr.html#lvmlogicalvolumesnapshot);
2. **Kube-scheduler** - сбор метрик компонента sds-node-configurator.
