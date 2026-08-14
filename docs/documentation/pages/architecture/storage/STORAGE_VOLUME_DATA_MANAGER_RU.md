---
title: Модуль storage-volume-data-manager
permalink: ru/architecture/storage/storage-volume-data-manager.html
lang: ru
search: storage-volume-data-manager, import, export
description: Архитектура модуля storage-volume-data-manager в Deckhouse Kubernetes Platform.
---

Модуль [`storage-volume-data-manager`](/modules/storage-volume-data-manager/) обеспечивает безопасные экспорт и импорт содержимого постоянных томов по протоколу HTTPS в Deckhouse Kubernetes Platform (DKP).

Модуль работает со следующими кастомными ресурсами:

- [DataExport](/modules/storage-volume-data-manager/cr.html#dataexport) — объект, описывающий операцию экспорта данных;
- [DataImport](/modules/storage-volume-data-manager/cr.html#dataimport) — объект, описывающий операцию импорта данных;
- VolumePopulator — объект, регистрирующий поддерживаемый исходный тип ресурса для наполнения PersistentVolume (PV). Подробнее можно ознакомиться [в описании механизма Volume Populators](https://kubernetes.io/docs/concepts/storage/volume-populators-and-data-sources/).

Модуль `storage-volume-data-manager` поддерживает работу операций экспорта для ресурсов PersistentVolumeClaim (PVC), [VolumeSnapshot](/modules/snapshot-controller/cr.html#volumesnapshot), [VirtualDisk](/modules/virtualization/cr.html#virtualdisk) и [VirtualDiskSnapshot](/modules/virtualization/stable/cr.html#virtualdisksnapshot).

Для операции импорта данных модуль поддерживает только ресурс PersistentVolumeClaim.

Подробнее с примерами использования модуля можно ознакомиться в [соответствующем разделе документации](/modules/storage-volume-data-manager/usage.html).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`storage-volume-data-manager`](/modules/storage-volume-data-manager/) на уровне 2 модели C4 и его взаимодействия с другими компонентами DKP изображены на следующей диаграмме:

![Архитектура модуля storage-volume-data-manager](../../images/architecture/storage/c4-l2-storage-volume-data-manager.ru.png)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Controller** (Deployment) — контроллер, обслуживающий кастомные ресурсы DataExport и DataImport. 

   Контроллер выполняет следующие основные операции:

   - создаёт и удаляет Deployment `deploy-for-<KIND_SHORT>-<HASH>`, соответствующие кастомным ресурсам DataExport;
   - обновляет в статусе DataExport или DataImport поле `status.url=https://<POD_IP>:8085/` при переходе соответствующего пода в состояние `Running`. Поле `status.url` используется для взаимодействия с сервисом импорта или экспорта с использованием утилиты [Deckhouse CLI](../../cli/d8/);
   - создаёт ресурсы Service и Ingress при [`spec.publish=true`](/modules/storage-volume-data-manager/cr.html#dataexport-v1alpha1-spec-publish) в DataExport или DataImport и удаляет их при `spec.publish=false`;
   - создаёт целевой PVC и следит за его состоянием для каждого ресурса DataImport;
   - создаёт и управляет ресурсом Job `dummy-for-<KIND_SHORT>-<HASH>`, если выполняются следующие условия:

      - в спецификации DataImport указан параметр [`spec.waitForFirstConsumer=false`](/modules/storage-volume-data-manager/cr.html#dataimport-v1alpha1-spec-waitforfirstconsumer);
      - в спецификации соответствующего StorageClass `volumeBindingMode=WaitForFirstConsumer`;
      - PVC находится в состоянии `Pending`;

   - реализует вебхуки для валидации кастомных ресурсов DataExport через механику [Validating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/).

   При обработке кастомного ресурса DataExport контроллер выполняет следующие действия, зависящие от источника данных:

   #### PersistentVolumeClaim
   
   1. Проверяет, что PV не смонтирован и нет связанных VolumeAttachment.
   1. Создаёт временный PVC и перепривязывает к нему целевой PV.
   1. Создаёт Deployment `deploy-for-pvc-<HASH>` и монтирует к нему PV через временный PVC.
   1. Восстанавливает привязку PV к исходному PVC при истечении `TTL` или удалении DataExport.
   1. Удаляет временный PVC.

   #### VolumeSnapshot
   
   1. Создаёт временные ресурсы VolumeSnapshotContent `data-pre-provisioned-<HASH>` и VolumeSnapshot `snapshot-<HASH>`, ссылающиеся на оригинальные данные снимка.
   1. Создаёт временный PVC, параметр `dataSource` которого ссылается на временный VolumeSnapshot. Драйвер Container Storage Interface (CSI) восстанавливает из снимка новый том и привязывает его к PVC.
   1. Создаёт Deployment `deploy-for-vs-<HASH>` и монтирует к нему PV через временный PVC.
   1. Удаляет временные PVC, VolumeSnapshot и VolumeSnapshotContent при истечении `TTL` или удалении DataExport.

   #### VirtualDisk
   
   1. Получает информацию о целевом PVC из параметра `status.target.persistentVolumeClaimName` ресурса VirtualDisk.
   1. Устанавливает аннотацию `storage.deckhouse.io/data-export-request=true`. Эту аннотацию обрабатывает virtualization-controller модуля [`virtualization`](/modules/virtualization/) и выполняет действия для отключения диска.
   1. Ожидает готовности VirtualDisk к экспорту: состояние `Ready` должно иметь значение `True` либо значение `False` с причиной `Exporting`, а состояние `InUse` — значение `True` с причиной `UsedForDataExport`.
   1. Выполняет такие же действия, как для PVC.
   1. Удаляет аннотацию `storage.deckhouse.io/data-export-request=true` с целевого PVC.

   #### VirtualDiskSnapshot
   
   1. Ожидает состояния `VirtualDiskSnapshotReady=True` у ресурса VirtualDiskSnapshot.
   1. Получает информацию о целевом VolumeSnapshot из параметра `status.volumeSnapshotName` ресурса VirtualDiskSnapshot.
   1. Выполняет действия, аналогичные действиям для VolumeSnapshot.

   Состоит из следующих контейнеров:

   * **controller** — основной контейнер;
   * **webhooks** — сайдкар-контейнер, реализующий вебхуки для валидации кастомных ресурсов DataExport.

1. **Populator** (Deployment) — компонент, состоящий из одного контейнера **populator**, который обеспечивает подготовку к импорту данных в PVC. Компонент отслеживает только те PVC, у которых в спецификации указано `dataSourceRef=DataImport`.

   Populator реализует следующий порядок выполнения:

   1. Создаёт временный служебный PVC и привязывает к нему существующий PV.
   1. Создаёт компонент deploy-for-&lt;KIND_SHORT&gt;-&lt;HASH&gt; (для импорта) и монтирует к нему PV, используя служебный PVC.
   1. Привязывает PV к пользовательскому PVC, когда заливка данных завершена.
   1. Удаляет компонент deploy-for-&lt;KIND_SHORT&gt;-&lt;HASH&gt; и служебный PVC.

1. **Data-source-validator** (StatefulSet) — компонент состоит из одного контейнера **data-source-validator**. Компонент основан на Open Source-проекте [volume-data-source-validator](https://github.com/kubernetes-csi/volume-data-source-validator).

   Data-source-validator проверяет, что тип ресурса, указанный в поле `dataSourceRef` PVC, зарегистрирован объектом VolumePopulator. Если тип не зарегистрирован, компонент создаёт предупреждающий Event.

1. **Deploy-for-&lt;KIND_SHORT&gt;-&lt;HASH&gt;** (Deployment) — компонент, реализующий HTTP-сервер передачи файлов и данных блочного тома при экспорте или импорте.

   В зависимости от режима запуска компонент состоит из следующих контейнеров:
   
   - **data-exporter** — опциональный контейнер, использующийся при запуске компонента для экспорта данных;
   - **server** — опциональный контейнер, использующийся при запуске компонента для импорта данных.

1. **Dummy-for-&lt;KIND_SHORT&gt;-&lt;HASH&gt;** (Job) — компонент состоит из одного контейнера **dummy-consumer**, который не выполняет никаких действий. Компонент необходим для создания PV, соответствующего запрошенному PVC. Это требуется для подготовки PV к обработке компонентом populator. 

   Компонент запускается в пользовательском неймспейсе, в котором определён целевой PVC.

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

   - авторизация запросов;
   - работа с кастомными ресурсами DataExport, DataImport, VolumePopulator, VolumeSnapshot, VolumeSnapshotContent, VirtualDisk и VirtualDiskSnapshot;
   - работа со стандартными ресурсами PersistentVolumeClaim, PersistentVolume, Ingress, Service, Secret, Event, Deployment `deploy-for-<KIND_SHORT>-<HASH>` и Job `dummy-for-<KIND_SHORT>-<HASH>`;
   - читает ресурсы Pod, ConfigMap, VolumeAttachment и StorageClass.

С модулем взаимодействуют следующие внешние компоненты:

1. **Kube-apiserver** — использование вебхука валидации для проверки создаваемых ресурсов DataExport.

1. **Controller nginx** — пересылка внешних запросов пользователя к сервисам импорта и экспорта.
