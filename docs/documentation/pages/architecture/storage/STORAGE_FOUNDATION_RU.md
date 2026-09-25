---
title: Модуль storage-foundation
permalink: ru/architecture/storage/storage-foundation.html
lang: ru
search: snapshot, снимок, import, export
description: Архитектура модуля storage-foundation в Deckhouse Kubernetes Platform.
---

Модуль [`storage-foundation`](/modules/storage-foundation/) включает поддержку снимков томов для совместимых драйверов Container Storage Interface (CSI) в Deckhouse Kubernetes Platform (DKP). Также модуль обеспечивает безопасные экспорт и импорт содержимого постоянных томов по протоколу HTTPS.

Следующие CSI-драйверы в DKP поддерживают создание снимков томов:

- [cloud-provider-openstack](/modules/cloud-provider-openstack/)
- [cloud-provider-vsphere](/modules/cloud-provider-vsphere/)
- [cloud-provider-aws](/modules/cloud-provider-aws/)
- [cloud-provider-azure](/modules/cloud-provider-azure/)
- [cloud-provider-gcp](/modules/cloud-provider-gcp/)
- [sds-local-volume](/modules/sds-local-volume/stable/)
- [sds-replicated-volume](/modules/sds-replicated-volume/stable/)
- [csi-ceph](/modules/csi-ceph/stable/)
- [csi-nfs](/modules/csi-nfs/stable/)
- [csi-hpe](/modules/csi-hpe/stable/)
- [csi-huawei](/modules/csi-huawei/stable/)
- [csi-yadro-tatlin-unified](/modules/csi-yadro-tatlin-unified/stable/)

{% alert level="info" %}
При установке модуля `storage-foundation` происходит замена образов внешних контроллеров CSI-драйвера на пропатченные версии, что приводит к перезапуску соответствующих контейнеров в DKP.
В пропатченных версиях компонентов устранены некоторые уязвимости CVE (Common Vulnerabilities and Exposures). Кроме того:

- в контроллер csi-external-snapshotter добавлена возможность создавать снимок тома без использования VolumeSnapshot, используется только VolumeSnapshotContent;
- в контроллер csi-external-provisioner добавлена обработка кастомного ресурса VolumeRestoreRequest, на основе которого контроллер создаёт PersistentVolume (PV) и PersistentVolumeClaim (PVC) и отдаёт команду CSI-драйверу выполнить копирование данных из VolumeSnapshotContent или из существующего PV.
{% endalert %}

Модуль работает со следующими кастомными ресурсами:

- [DataExport](/modules/storage-foundation/cr.html#dataexport) — объект, описывающий операцию экспорта данных;
- [DataImport](/modules/storage-foundation/cr.html#dataimport) — объект, описывающий операцию импорта данных;
- VolumeCaptureRequest — объект, описывающий операцию создания снимка содержимого тома PVC или другого поддерживаемого ресурса;
- VolumeRestoreRequest — объект, описывающий операцию восстановления содержимого тома PVC или другого поддерживаемого ресурса из снимка;
- VolumePopulator — объект, регистрирующий поддерживаемый исходный тип ресурса для наполнения PV. Подробнее можно ознакомиться [в описании механики Volume Populators](https://kubernetes.io/docs/concepts/storage/volume-populators-and-data-sources/);
- [VolumeSnapshot](/modules/storage-foundation/cr.html#volumesnapshot) — объект, описывающий запрос на создание точечного снимка постоянного тома или на привязку к существующему снимку;
- [VolumeSnapshotContent](/modules/storage-foundation/cr.html#volumesnapshotcontent) — объект, представляющий фактические данные снимка постоянного тома;
- [VolumeSnapshotClass](/modules/storage-foundation/cr.html#volumesnapshotclass) — объект, содержащий параметры создания снимков томов, используемые нижележащей системой хранения.

Модуль `storage-foundation` поддерживает экспорт для ресурсов PVC, [VirtualDisk](/modules/virtualization/cr.html#virtualdisk) и любой ресурс снимка, поддерживающий контракт [state-snapshotter](/modules/state-snapshotter/) (например: [VolumeSnapshot](/modules/storage-foundation/cr.html#volumesnapshot), [VirtualDiskSnapshot](/modules/virtualization/stable/cr.html#virtualdisksnapshot).

Подробнее с примерами использования модуля можно ознакомиться в [соответствующем разделе документации](/modules/storage-foundation/usage.html).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`storage-foundation`](/modules/storage-foundation/) на уровне 2 модели C4 и его взаимодействия с другими компонентами DKP изображены на следующей диаграмме:

![Архитектура модуля storage-foundation](../../images/architecture/storage/c4-l2-storage-foundation.ru.png)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Controller** (Deployment) — контроллер, состоящий из одного контейнера **controller** и обслуживающий кастомные ресурсы VolumeCaptureRequest, VolumeRestoreRequest и VolumeSnapshot.

   Контроллер выполняет следующие действия:

   - создаёт VolumeSnapshotContent и ожидает завершения операции создания снимка компонентом csi-external-snapshotter CSI-драйвера, при обработке кастомного ресурса VolumeCaptureRequest с параметром `spec.mode=Snapshot`;
   - при обработке кастомного ресурса VolumeCaptureRequest с параметром `spec.mode=Detach`:
      - отвязывает PV от соответствующего PVC;
      - устанавливает аннотацию `storage-foundation.deckhouse.io/detached=true` в ресурсе PV;
      - удаляет указанный PVC;
   - ожидает готовности PVC при обработке кастомного ресурса VolumeRestoreRequest. Непосредственно обработку ресурса и создание PVC выполняет csi-external-provisioner соответствующего CSI-драйвера;
   - обрабатывает жизненный цикл кастомного ресурса VolumeSnapshot, интегрируя его для работы с модулем [`state-snapshotter`](/modules/state-snapshotter/);
   - удаляет ресурсы VolumeCaptureRequest и VolumeRestoreRequest после истечения периода хранения, который отсчитывается от `status.completionTimestamp`.

1. **Data-manager-controller** (Deployment) — контроллер, состоящий из одного контейнера **data-manager-controller** и обслуживающий кастомные ресурсы DataExport и DataImport.

   Контроллер data-manager-controller выполняет следующие основные операции:

   - создаёт и удаляет Deployment `deploy-for-<KIND_SHORT>-<HASH>`, соответствующие кастомным ресурсам DataExport и DataImport;
   - обновляет в статусе DataExport или DataImport поле `status.url=https://<POD_IP>:8085/` при переходе соответствующего пода в состояние `Running`. Поле `status.url` используется для взаимодействия с сервисом импорта или экспорта с использованием утилиты [Deckhouse CLI](../../cli/d8/);
   - создаёт ресурсы Service и Ingress при [`spec.publish=true`](/modules/storage-foundation/cr.html#dataexport-v1alpha1-spec-publish) в DataExport или DataImport и удаляет их при `spec.publish=false`.

   При обработке кастомного ресурса DataExport контроллер выполняет следующие действия, зависящие от источника данных:

   - PersistentVolumeClaim:

      1. Проверяет, что PV не смонтирован и нет связанных VolumeAttachment.
      1. Создаёт временный PVC и перепривязывает к нему целевой PV.
      1. Создаёт Deployment `deploy-for-<KIND_SHORT>-<HASH>` и монтирует к нему PV через временный PVC.
      1. Восстанавливает привязку PV к исходному PVC при истечении `TTL` или удалении DataExport.
      1. Удаляет временный PVC.

   - VirtualDisk:

      1. Получает информацию о целевом PVC из параметра `status.target.persistentVolumeClaim` ресурса VirtualDisk.
      1. Устанавливает аннотации `storage-foundation.deckhouse.io/data-export-request=true` и `storage.deckhouse.io/data-export-request=true`. Эту аннотацию обрабатывает virtualization-controller модуля [`virtualization`](/modules/virtualization/) и выполняет действия для отключения диска.
      1. Ожидает готовности VirtualDisk к экспорту: состояние `Ready` должно иметь значение `True` либо значение `False` с причиной `Exporting`, а состояние `InUse` — значение `True` с причиной `UsedForDataExport`.
      1. Выполняет такие же действия, как для PVC.
      1. Удаляет аннотации `storage-foundation.deckhouse.io/data-export-request=true` и `storage.deckhouse.io/data-export-request=true` с целевого PVC.

   - ресурсы снимков, реализующие контракт [state-snapshotter](/modules/state-snapshotter/):

      1. Создаёт кастомный ресурс VolumeRestoreRequest.
      1. Ожидает выполнения запроса на восстановление, получает PV с данными.
      1. Создаёт Deployment `deploy-for-<KIND_SHORT>-<HASH>` и монтирует к нему PV через временный PVC.
      1. Удаляет временные PVC, VolumeRestoreRequest при истечении `TTL` или удалении DataExport.

   При обработке кастомного ресурса DataImport контроллер выполняет следующие действия, зависящие от режима импорта данных:

   - режим `spec.mode=CreatePVC`:

      1. Создаёт целевой PVC и следит за его состоянием для каждого ресурса DataImport.
      1. Создаёт и управляет ресурсом Job `dummy-for-<KIND_SHORT>-<HASH>`, если выполняются следующие условия:

         - в спецификации DataImport указан параметр [`spec.waitForFirstConsumer=false`](/modules/storage-foundation/cr.html#dataimport-v1alpha1-spec-waitforfirstconsumer);
         - в спецификации соответствующего StorageClass `volumeBindingMode=WaitForFirstConsumer`;
         - PVC находится в состоянии `Pending`.
      1. Ожидает завершения наполнения данными компонентом populator.

   - режим `spec.mode=PopulateData`:

      1. Создаёт временный PVC.
      1. Создаёт и управляет ресурсом Job `dummy-for-<KIND_SHORT>-<HASH>`, если в спецификации соответствующего StorageClass `volumeBindingMode=WaitForFirstConsumer` и PVC находится в состоянии `Pending`.
      1. Ожидает завершения наполнения данными компонентом populator.
      1. Создаёт кастомный ресурс VolumeCaptureRequest `data-import-vcr-<UID>` и ожидает готовности VolumeSnapshotContent.
      1. Удаляет временный PVC.

1. **Populator** (Deployment) — компонент, состоящий из одного контейнера **populator**, который обеспечивает подготовку к импорту данных в PVC. Компонент отслеживает только те PVC, у которых в спецификации указано `dataSourceRef=DataImport`, и работает совместно с механикой [Volume Populators](https://kubernetes.io/docs/concepts/storage/volume-populators-and-data-sources/).

   В процессе импорта механизм Volume Populators создаёт и обслуживает служебный PVC, через который выполняется наполнение тома данными. Компонент populator выполняет следующие действия:

   1. Создаёт Deployment `deploy-for-<KIND_SHORT>-<HASH>` для импорта и монтирует к нему служебный PVC.
   1. Ожидает завершения заливки данных.
   1. Удаляет временный Deployment после завершения импорта.

1. **Data-source-validator** (StatefulSet) — компонент состоит из одного контейнера **data-source-validator**. Компонент основан на Open Source-проекте [volume-data-source-validator](https://github.com/kubernetes-csi/volume-data-source-validator).

   Data-source-validator проверяет, что тип ресурса, указанный в поле `dataSourceRef` PVC, зарегистрирован объектом VolumePopulator. Если тип не зарегистрирован, компонент создаёт предупреждающий Event.

1. **Deploy-for-&lt;KIND_SHORT&gt;-&lt;HASH&gt;** (Deployment) — компонент, реализующий HTTP-сервер передачи файлов и данных блочного тома при экспорте или импорте.

   В зависимости от режима запуска компонент состоит из следующих контейнеров:

   - **data-exporter** — опциональный контейнер, использующийся при запуске компонента для экспорта данных;
   - **server** — опциональный контейнер, использующийся при запуске компонента для импорта данных.

1. **Dummy-for-&lt;KIND_SHORT&gt;-&lt;HASH&gt;** (Job) — компонент состоит из одного контейнера **dummy-consumer**. Компонент запускается для того, чтобы инициировать привязку временного PVC в StorageClass с `volumeBindingMode=WaitForFirstConsumer`.

   Компонент запускается в пользовательском неймспейсе, в котором определён целевой PVC.

1. **Webhooks** (Deployment) — компонент, состоящий из одного контейнера **webhooks**, который реализует validation-вебхуки для кастомных ресурсов DataExport и mutating-вебхуки для ресурсов VolumeSnapshot через механизм [Validation/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/).

1. **Snapshot-controller** — контроллер снимков, который работает совместно с сайдкар-контейнером snapshotter ([external-snapshotter](https://github.com/kubernetes-csi/external-snapshotter)) пода csi-controller CSI-драйвера (при условии, что CSI-драйвер провайдера поддерживает создание снимков).

   Для всех установленных CSI-драйверов используется один snapshot-controller, который следит за ресурсами VolumeSnapshot и VolumeSnapshotContent. При появлении нового ресурса VolumeSnapshot контроллер создает ресурс VolumeSnapshotContent и связывает их между собой. В результате VolumeSnapshot ссылается на соответствующий VolumeSnapshotContent, а тот — на исходный VolumeSnapshot.

   Создание снимка — это многоступенчатый процесс:

   1. Snapshot-controller создает ресурс VolumeSnapshotContent.
   1. Сайдкар snapshotter запускает создание снимка с помощью csi-controller на соответствующем узле и обновляет статус VolumeSnapshotContent (поля `snapshotHandle`, `creationTime`, `restoreSize`, `readyToUse` и `error`).
   1. Snapshot-controller отслеживает статус VolumeSnapshotContent и обновляет статус ресурса VolumeSnapshot до завершения двунаправленной привязки и установки поля `readyToUse` в значение `true`. При возникновении ошибки аналогичным образом обновляется поле `error`.

   Состоит из следующих контейнеров:

   * **snapshot-controller** — является [Open Source-проектом](https://github.com/kubernetes-csi/external-snapshotter/tree/master/pkg/common-controller);
   * **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищенного доступа к метрикам контроллера. Является [Open Source-проектом](https://github.com/brancz/kube-rbac-proxy).

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

   - авторизация запросов;
   - работа с кастомными ресурсами DataExport, DataImport, VolumeCaptureRequest, VolumeRestoreRequest, VolumePopulator, VolumeSnapshot, VolumeSnapshotContent, VirtualDisk и VirtualDiskSnapshot;
   - работа со стандартными ресурсами PersistentVolumeClaim, PersistentVolume, Ingress, Service, Secret, Event, Deployment `deploy-for-<KIND_SHORT>-<HASH>` и Job `dummy-for-<KIND_SHORT>-<HASH>`;
   - читает ресурсы Pod, ConfigMap, VolumeAttachment и StorageClass.

С модулем взаимодействуют следующие внешние компоненты:

1. **Kube-apiserver**:

   - использование вебхука валидации для проверки создаваемых ресурсов DataExport;
   - использование mutating-вебхука для ресурса VolumeSnapshot.

1. **Controller nginx** — пересылка внешних запросов пользователя к сервисам импорта и экспорта.

1. **Prometheus-main** — сбор метрик компонента snapshot-controller.
