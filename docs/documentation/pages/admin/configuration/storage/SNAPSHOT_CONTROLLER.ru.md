---
title: "Настройка создания снимков томов"
permalink: ru/admin/configuration/storage/snapshot-controller.html
lang: ru
---

Deckhouse Kubernetes Platform поддерживает создание снимков томов для CSI-драйверов в кластере Kubernetes.

Снимки фиксируют состояние тома на определенный момент времени и могут быть использованы для восстановления данных или клонирования томов. Способность создавать снимки зависит от возможностей используемого CSI-драйвера.

## Поддерживаемые CSI-драйверы

Создание снимков поддерживается следующими CSI-драйверами:

- [Облачные ресурсы провайдера OpenStack](/modules/cloud-provider-openstack/);
- [Облачные ресурсы провайдера VMWare vSphere](/modules/cloud-provider-vsphere/);
- [Распределённое хранилище Ceph](../storage/external/ceph.html);
- [Облачные ресурсы провайдера Amazon Web Services](/modules/cloud-provider-aws/);
- [Облачные ресурсы провайдера Microsoft Azure](/modules/cloud-provider-azure/);
- [Облачные ресурсы провайдера Google Cloud Platform](/modules/cloud-provider-gcp/);
- [Реплицируемое хранилище на основе DRBD](../storage/sds/lvm-replicated.html);
- [Хранилище данных NFS](../storage/external/nfs.html).

## Создание снимков

Перед созданием снимков убедитесь, что в кластере настроены объекты VolumeSnapshotClass. Список доступных классов можно получить командой:

```shell
d8 k get volumesnapshotclasses.snapshot.storage.k8s.io
```

Чтобы создать снимок для тома, укажите нужный VolumeSnapshotClass в манифесте:

```yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: example-snapshot
spec:
  volumeSnapshotClassName: <имя-класса>
  source:
    persistentVolumeClaimName: <имя-PVC>
```

## Восстановление из снимка

Чтобы восстановить данные из снимка, создайте PVC, ссылающийся на ранее созданный объект VolumeSnapshot:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: restored-pvc
spec:
  dataSource:
    name: example-snapshot
    kind: VolumeSnapshot
    apiGroup: snapshot.storage.k8s.io
  storageClassName: <имя-StorageClass>
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
```

{% alert level="warning" %}
Не все CSI-драйверы поддерживают восстановление тома из снимка. Убедитесь, что используемый драйвер поддерживает соответствующие возможности.
{% endalert %}
