{% alert level="warning" %}
На этом этапе приведен пример настройки программно-определяемого хранилища на основе DRBD.
Если вы хотите использовать другой типа хранилища, ознакомьтесь с разделом [«Настройка хранилища»](../../documentation/admin/install/steps/storage.html).
{% endalert %}

На этом шаге кластер в минимальном исполнении развернут. Настройте хранилище, которое будет использоваться для создания хранения метрик компонент кластер и дисков виртуальных машин.

Включите модуль программно-определяемого хранилища sds-replicated-volume. Выполните на **master-узле** следующие команды:

```shell
sudo -i d8 k create -f - <<EOF
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: snapshot-controller
spec:
  enabled: true
  version: 1
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: sds-node-configurator
spec:
  version: 1
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: sds-replicated-volume
spec:
  version: 1
  enabled: true
EOF
```

Дождитесь, пока модуль включится, для этого можете использовать следующую команду:

```shell
sudo -i d8 k wait module sds-replicated-volume --for='jsonpath={.status.phase}=Ready' --timeout=1200s
```

Объедините доступные на узлах блочные устройства в группы томов LVM. Чтобы получить доступные блочные устройства, выполните команду:

```shell
sudo -i d8 k get blockdevices.storage.deckhouse.io
```

Чтобы объединить блочные устройства на одном узле, необходимо создать группу томов LVM с помощью ресурса [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup).
Для создания ресурса LVMVolumeGroup на узле выполните следующую команду, предварительно заменив имена узла и блочных устройств на свои:

```shell
sudo -i d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: LVMVolumeGroup
metadata:
  name: "vg-on-dvp-worker"
spec:
  type: Local
  local:
    # Замените на имя своего узла, для которого создаете группу томов.
    nodeName: "dvp-worker"
  blockDeviceSelector:
    matchExpressions:
      - key: kubernetes.io/metadata.name
        operator: In
        values:
          # Замените на имена своих блочных устройств узла, для которого создаете группу томов.
          <span class="mustChange">- dev-ef4fb06b63d2c05fb6ee83008b55e486aa1161aa</span>
  # Имя группы томов LVM, которая будет создана из указанных выше блочных устройств на выбранном узле.
  actualVGNameOnTheNode: "vg"
EOF
```

Дождитесь, когда созданный ресурс LVMVolumeGroup перейдет в состояние `Operational`:

```shell
sudo -i d8 k get lvg vg-on-dvp-worker -w
```

Пример вывода:

```console
NAME               THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE       ALLOCATED SIZE   VG   AGE
vg-on-dvp-worker   1/1         True                    Ready   worker-0   360484Mi   30064Mi          vg   1h
```

Создайте пул LVM-томов:

```bash
sudo -i d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: ReplicatedStoragePool
metadata:
  name: sds-pool
spec:
  type: LVM
  lvmVolumeGroups:
    - name: vg-on-dvp-worker
EOF
```

Дождитесь, когда созданный ресурс ReplicatedStoragePool перейдет в состояние `Completed`:

```shell
sudo -i d8 k get rsp sds-pool -w
```

Пример вывода:

```console
NAME         PHASE       TYPE   AGE
sds-pool     Completed   LVM    87d
```

Создайте StorageClass:

```bash
sudo -i d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: ReplicatedStorageClass
metadata:
  name: sds-r1
spec:
  replication: None
  storagePool: sds-pool
  reclaimPolicy: Delete
  topology: Ignored
EOF
```

Проверьте, что ресурсы StorageClass появились в кластере:

```bash
sudo -i d8 k get storageclass
```

Установите StorageClass как используемый в кластере по умолчанию (укажите имя StorageClass):

```shell
DEFAULT_STORAGE_CLASS=sds-r1
sudo -i d8 k patch mc global --type='json' -p='[{"op": "replace", "path": "/spec/settings/defaultClusterStorageClass", "value": "'"$DEFAULT_STORAGE_CLASS"'"}]'
```
