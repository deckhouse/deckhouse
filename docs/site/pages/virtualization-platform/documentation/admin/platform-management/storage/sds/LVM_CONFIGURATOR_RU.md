---
title: "SDS Node Configurator"
permalink: ru/virtualization-platform/documentation/admin/platform-management/storage/sds/lvm-configurator.html
lang: ru
---

## Преднастройка узлов

Перед тем как приступить к настройке возможности создания StorageClass’ов на базе LVM (Logical Volume Manager), 
необходимо создать на узлах группы томов LVM, которые в дальнейшем будут использоваться для размещения PersistentVolume’ов.
Для этого нужно создать ресурс `LVMVolumeGroup`, который позволит отразить актуальную информацию
о состоянии групп томов LVM и через который будет идти взаимодействие с ними.

Для создания группы томов LVM необходимо получить ресурсы `BlockDevices`, которые позволят узнать доступные на узлах 
блочные устройства. Управление BlockDevice’ами осуществляется автоматически и не требует вмешательства со стороны пользователя. 
Ручное изменение на `BlockDevices` может привести к нежелательному поведению. 

Чтобы узнать какие блочные устройства доступны для создания группы томов LVM, выполните команду:

```shell
d8 k get bd

# NAME                                           NODE       CONSUMABLE   SIZE           PATH
# dev-ef4fb06b63d2c05fb6ee83008b55e486aa1161aa   worker-0   false        976762584Ki    /dev/nvme1n1
# dev-0cfc0d07f353598e329d34f3821bed992c1ffbcd   worker-0   false        894006140416   /dev/nvme0n1p6
# dev-7e4df1ddf2a1b05a79f9481cdf56d29891a9f9d0   worker-1   false        976762584Ki    /dev/nvme1n1
# dev-b103062f879a2349a9c5f054e0366594568de68d   worker-1   false        894006140416   /dev/nvme0n1p6
# dev-53d904f18b912187ac82de29af06a34d9ae23199   worker-2   false        976762584Ki    /dev/nvme1n1
# dev-6c5abbd549100834c6b1668c8f89fb97872ee2b1   worker-2   false        894006140416   /dev/nvme0n1p6
```

В примере выполнения команды выше в наличии имеется 6 блочных устройств, расположенных на 3 узлах.

Чтобы создать группу томов `LVMVolumeGroup` для узла worker-0 примените следующий ресурс, предварительно заменив имена 
узла и блочных устройств на свои:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: LVMVolumeGroup
metadata:
  name: "vg-on-worker-0"
spec:
  type: Local
  local:
    # Замените на имя своего узла, для которого создаете группу томов. 
    nodeName: "worker-0"
  blockDeviceSelector:
    matchExpressions:
      - key: kubernetes.io/metadata.name
        operator: In
        values:
          # Замените на имена своих блочных устройств узла, для которого создаете группу томов. 
          - dev-ef4fb06b63d2c05fb6ee83008b55e486aa1161aa
          - dev-0cfc0d07f353598e329d34f3821bed992c1ffbcd
  # Имя группы томов LVM, которая будет создана из указанных выше блочных устройств на выбранном узле.
  actualVGNameOnTheNode: "vg"
  # Удалите, если не планируете создавать Thin-хранилища. 
  thinPools:
    - name: thin-pool-0
      size: 50%
EOF
```

StorageClass'ы могут быть созданы с двумя типами хранение: Thin и Thick. Thick хранилище обладает высокой
производительностью, сравнимой с производительностью накопителя, но не позволяет использовать snapshot’ы,
в то время как Thin позволит использовать snapshot’ы, но производительность будет ниже. 

В созданной выше группе томов LVM 50% доступного пространства будет использовано для создания Thin хранилищ. Оставшие 50% 
могут быть использованы для Thick хранилищ. Если вам не нужны Thin хранилища, то можно опустить определение Thin-пулов.

Дождитесь, когда созданный ресурс `LVMVolumeGroup` перейдет в состояние `Operational`.

```shell
d8 k get lvg vg-on-worker-0 -w

# NAME             THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE       ALLOCATED SIZE   VG   AGE
# vg-on-worker-0   1/1         True                    Ready   worker-0   360484Mi   30064Mi          vg   1h
```

Если ресурс перешел в состояние Operational, то это значит, что на узле worker-0
из блочных устройств /dev/nvme1n1 и /dev/nvme0n1p6 была создана группа томов LVM с именем vg.

Далее необходимо повторить создание ресурсов `LVMVolumeGroup` для оставшихся узлов (worker-1 и worker-2),
изменив в примере выше имя ресурса `LVMVolumeGroup`, имя узла и имена блочных устройств, соответствующих узлу.

```shell
d8 k get lvg -w

# NAME             THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE       ALLOCATED SIZE   VG   AGE
# vg-on-worker-0   1/1         True                    Ready   worker-0   360484Mi   30064Mi          vg   1h
# vg-on-worker-1   1/1         True                    Ready   worker-1   360484Mi   30064Mi          vg   1h
# vg-on-worker-2   1/1         True                    Ready   worker-2   360484Mi   30064Mi          vg   1h
```
