---
title: "Модуль sds-node-configurator: сценарии конфигурации sds-модулей"
linkTitle: "Сценарии конфигурации"
description: "Сценарии конфигурации sds-модулей с помощью sds-node-configurator"
---

{% alert level="warning" %}
Работоспособность модуля гарантируется только при использовании стоковых ядер, поставляемых вместе с поддерживаемыми дистрибутивами.

Работоспособность модуля при использовании других ядер или дистрибутивов возможна, но не гарантируется.
{% endalert %}

{% alert level="info" %}
Если вы создаёте виртуальные машины клонированием,
необходимо изменить UUID у групп томов (VG) на созданных таким образом виртуальных машинах, выполнив команду `vgchange -u`.
Данная команда сгенерирует новые UUID для всех VG на виртуальной машине.
При необходимости команду можно добавить в скрипт `cloud-init`.

Изменить UUID у VG можно, только если в группе томов нет активных логических томов (LV).
Чтобы деактивировать логический том, отмонтируйте его и выполните следующую команду:

```shell
lvchange -an <VG_or_LV_NAME>
```

, где `<VG_or_LV_NAME>` — название VG, для деактивации всех томов в группе, или название LV, для деактивации конкретного тома.
{% endalert %}

## Способы и сценарии конфигурации дисковой подсистемы узлов

На данной странице рассматриваются 2 способа конфигурации дисковой подсистемы на узлах кластера Kubernetes
в зависимости от условий организации хранилища:

- [Хранилище с одинаковыми дисками](#хранилище-с-одинаковыми-дисками).
- [Комбинированное хранилище](#комбинированное-хранилище).

Для каждого из способов конфигурации дисковой подсистемы на узлах существует два сценария конфигурации:

- «Полное зеркало». Мы рекомендуем использовать данный сценарий конфигурации, поскольку он достаточно надёжен и прост в настройке.
- «Частичное зеркало».

Особенности, плюсы и минусы сценариев приведены в таблице:

| Сценарий конфигурации | Особенности реализации                                                                                                                                                                                                            | Плюсы                                                                                                                    | Минусы                                                                                                                          |
|-----------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------|
| «Полное зеркало» | <ul><li>Диски не делятся на разделы, делается зеркало из дисков целиком</li><li>Используется одна VG для корневой системы и для данных</li></ul>                                                                                  | <ul><li>Надежно</li><li>Просто в настройке и использовании</li><li>Удобно распределять место между разными SDS</li></ul> | <ul><li>Избыточное место на диске для программно-определяемых хранилищ (SDS), которые сами реплицируют данные</li></ul>         |
| «Частичное зеркало» | <ul><li>Диски делятся на 2 раздела</li><li>Из первых разделов каждого диска создается зеркало. На нем создается VG, куда устанавливается ОС</li><li>Из вторых разделов диска создается VG для данных, без зеркалирования</li><ul> | <ul><li>Надежно</li><li>Максимально эффективное использование места                                                      | <ul><li>Сложно в настройке и использовании</li><li>Очень сложно перераспределять место между safe- и unsafe-разделами</li></ul> |

Различия в порядке конфигурации дисковой подсистемы в зависимости от выбранного сценария конфигурации изображены на схеме:

![Флоу с разными сценариями](images/sds-node-configurator-scenaries.ru.png)

## Хранилище с одинаковыми дисками

Этот способ подразумевает использование на узле дисков одного типа.

### Полное зеркало

Мы рекомендуем использовать этот сценарий конфигурации, поскольку он достаточно надёжен и прост в настройке.

Чтобы настроить узел по этому сценарию, выполните следующее:

1. Соберите зеркало из дисков целиком (аппаратно или программно). Это зеркало будет использоваться одновременно для корневой системы и для данных.
2. При установке операционной системы:
   * создайте VG с именем `main` на зеркале;
   * создайте LV с именем `root` в VG `main`;
   * установите операционную систему на LV `root`.
3. Установите тег `storage.deckhouse.io/enabled=true` для VG `main`, используя следующую команду:

   ```shell
   vgchange main --addtag storage.deckhouse.io/enabled=true
   ```

4. Добавьте подготовленный узел в кластер Deckhouse.

   Если узел подходит под `nodeSelector`, который указан в `spec.nodeSelector` модулей `sds-replicated-volume` или `sds-local-volume`,
   то на этом узле запустится агент модуля `sds-node-configurator`,
   который определит VG `main` и добавит соответствующий этой VG ресурс `LVMVolumeGroup` в кластер Deckhouse.
   Дальше ресурс `LVMVolumeGroup` можно использовать для создания томов в модулях `sds-replicated-volume` или `sds-local-volume`.

#### Пример настройки модулей SDS (одинаковые диски, «Полное зеркало»)

В данном примере предполагается, что вы настроили три узла по сценарию «Полное зеркало».
В кластере Deckhouse при этом появятся три ресурса `LVMVolumeGroup` со случайно сгенерированными именами.
В будущем добавится возможность указывать имя для ресурсов `LVMVolumeGroup`,
которые создаются в процессе автоматического обнаружения VG, с помощью тега `LVM` с желаемым именем ресурса.

Чтобы вывести список ресурсов `LVMVolumeGroup`, выполните следующую команду:

```shell
kubectl get lvmvolumegroups.storage.deckhouse.io
```

В результате будет выведен следующий список:

```console
NAME                                      THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE      ALLOCATED SIZE   VG     AGE
vg-08d3730c-9201-428d-966c-45795cba55a6   0/0         True                    Ready   worker-2   25596Mi   0                main   61s
vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d   0/0         True                    Ready   worker-0   25596Mi   0                main   4m17s
vg-c7863e12-c143-42bb-8e33-d578ce50d6c7   0/0         True                    Ready   worker-1   25596Mi   0                main   108s
```

##### Настройка модуля `sds-local-volume` (одинаковые диски, «Полное зеркало»)

Чтобы настроить модуль `sds-local-volume` по сценарию «Полное зеркало», создайте ресурс `LocalStorageClass`
и добавьте в него все ресурсы `LVMVolumeGroup`, чтобы VG `main` использовалась на всех узлах в модуле `sds-local-volume`:

```yaml
kubectl apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: LocalStorageClass
metadata:
  name: local-sc
spec:
  lvm:
    lvmVolumeGroups:
      - name: vg-08d3730c-9201-428d-966c-45795cba55a6
      - name: vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d
      - name: vg-c7863e12-c143-42bb-8e33-d578ce50d6c7
    type: Thick
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
EOF
```

##### Настройка модуля `sds-replicated-volume` (одинаковые диски, «Полное зеркало»)

Чтобы настроить модуль `sds-replicated-volume` по сценарию «Полное зеркало», выполните следующее:

1. Создайте ресурс `ReplicatedStoragePool` и добавьте в него все ресурсы `LVMVolumeGroup`,
   чтобы VG `main` использовалась на всех узлах в модуле `sds-replicated-volume`:

   ```yaml
   kubectl apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStoragePool
   metadata:
     name: data
   spec:
     type: LVM
     lvmVolumeGroups:
       - name: vg-08d3730c-9201-428d-966c-45795cba55a6
       - name: vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d
       - name: vg-c7863e12-c143-42bb-8e33-d578ce50d6c7
   EOF
   ```

2. Создайте ресурс `ReplicatedStorageClass` и в поле `storagePool` укажите имя созданного ранее ресурса `ReplicatedStoragePool`:

   ```yaml
   kubectl apply -f -<<EOF
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-r1
   spec:
     storagePool: data
     replication: None
     reclaimPolicy: Delete
     topology: Ignored # Если указать данную топологию, в кластере не должно быть зон (узлов с метками topology.kubernetes.io/zone)
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-r2
   spec:
     storagePool: data
     replication: Availability
     reclaimPolicy: Delete
     topology: Ignored
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-r3
   spec:
     storagePool: data
     replication: ConsistencyAndAvailability
     reclaimPolicy: Delete
     topology: Ignored
   EOF
   ```

### Частичное зеркало

{% alert level="warning" %}
Не поддерживается использование разделов с одинаковыми PARTUUID,
а также изменение PARTUUID раздела, который используется для создания VG.
При создании таблицы разделов рекомендуется выбрать формат `GPT`,
так как PARTUUID в `MBR` является псевдослучайным и содержит в себе номер раздела.
Помимо этого, в `MBR` нельзя задать атрибут PARTLABEL,
который может пригодиться для последующей идентификации раздела в Deckhouse.
{% endalert %}

В данном сценарии используются два раздела на каждом диске:
один для корневой системы и хранения данных SDS, которые не реплицируются,
и другой для данных SDS, которые реплицируются.
Первый раздел каждого диска используется для создания зеркала, а второй – для создания отдельной VG без зеркалирования.
Это позволяет максимально эффективно использовать место на диске.

Чтобы настроить узел по сценарию «Частичное зеркало», выполните следующее:

1. При установке операционной системы:
   * создайте по два раздела на каждом диске;
   * соберите зеркало из первых разделов на каждом диске;
   * создайте VG с именем `main-safe` на зеркале;
   * создайте LV с именем `root` в VG `main-safe`;
   * установите операционную систему на LV `root`.
2. Установите тег `storage.deckhouse.io/enabled=true` для VG `main-safe`, используя следующую команду:
  
   ```shell
   vgchange main-safe --addtag storage.deckhouse.io/enabled=true
   ```

3. Создайте VG с именем `main-unsafe` из вторых разделов каждого диска.
4. Установите тег `storage.deckhouse.io/enabled=true` для VG `main-unsafe`, используя следующую команду:

   ```shell
   vgchange main-unsafe --addtag storage.deckhouse.io/enabled=true
   ```

5. Добавьте подготовленный узел в кластер Deckhouse.

   Если узел подходит под `nodeSelector`, который указан в `spec.nodeSelector` модулей `sds-replicated-volume` или `sds-local-volume`,
   то на этом узле запустится агент модуля `sds-node-configurator`,
   который определит VG `main-safe` и `main-unsafe` и добавит соответствующие этим VG ресурсы `LVMVolumeGroup` в кластер Deckhouse.
   Дальше ресурсы `LVMVolumeGroup` можно использовать для создания томов в модулях `sds-replicated-volume` или `sds-local-volume`.

#### Пример настройки модулей SDS (одинаковые диски, «Частичное зеркало»)

В данном примере предполагается, что вы настроили три узла по сценарию «Частичное зеркало».
В кластере Deckhouse при этом появятся шесть ресурсов `LVMVolumeGroup` со случайно сгенерированными именами.
В будущем добавится возможность указывать имя для ресурсов `LVMVolumeGroup`,
которые создаются в процессе автоматического обнаружения VG, с помощью тега `LVM` с желаемым именем ресурса.

Чтобы вывести список ресурсов `LVMVolumeGroup`, выполните следующую команду:

```shell
kubectl get lvmvolumegroups.storage.deckhouse.io
```

В результате будет выведен следующий список:

```console
NAME                                      THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE      ALLOCATED SIZE   VG            AGE
vg-08d3730c-9201-428d-966c-45795cba55a6   0/0         True                    Ready   worker-2   25596Mi   0                main-safe     61s
vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d   0/0         True                    Ready   worker-0   25596Mi   0                main-safe     4m17s
vg-c7863e12-c143-42bb-8e33-d578ce50d6c7   0/0         True                    Ready   worker-1   25596Mi   0                main-safe     108s
vg-deccf08a-44d4-45f2-aea9-6232c0eeef91   0/0         True                    Ready   worker-2   25596Mi   0                main-unsafe   61s
vg-e0f00cab-03b3-49cf-a2f6-595628a2593c   0/0         True                    Ready   worker-0   25596Mi   0                main-unsafe   4m17s
vg-fe679d22-2bc7-409c-85a9-9f0ee29a6ca2   0/0         True                    Ready   worker-1   25596Mi   0                main-unsafe   108s
```

##### Настройка модуля `sds-local-volume` (одинаковые диски, «Частичное зеркало»)

Чтобы настроить модуль `sds-local-volume` по сценарию «Частичное зеркало», создайте ресурс `LocalStorageClass`
и добавьте в него ресурсы `LVMVolumeGroup`, чтобы на всех узлах в модуле `sds-local-volume` использовалась только VG `main-safe`:

```yaml
kubectl apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: LocalStorageClass
metadata:
  name: local-sc
spec:
  lvm:
    lvmVolumeGroups:
      - name: vg-08d3730c-9201-428d-966c-45795cba55a6
      - name: vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d
      - name: vg-c7863e12-c143-42bb-8e33-d578ce50d6c7
    type: Thick
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
EOF
```

##### Настройка модуля `sds-replicated-volume` (одинаковые диски, «Частичное зеркало»)

Чтобы настроить модуль `sds-replicated-volume` по сценарию «Частичное зеркало», выполните следующее:

1. Создайте ресурс `ReplicatedStoragePool` с именем `data-safe` и добавьте в него ресурсы `LVMVolumeGroup`,
   чтобы на всех узлах в модуле `sds-replicated-volume` в `ReplicatedStorageClass` с параметром `replication: None`
   использовалась только VG `main-safe`:

   ```yaml
   kubectl apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStoragePool
   metadata:
     name: data-safe
   spec:
     type: LVM
     lvmVolumeGroups:
       - name: vg-08d3730c-9201-428d-966c-45795cba55a6
       - name: vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d
       - name: vg-c7863e12-c143-42bb-8e33-d578ce50d6c7
   EOF
   ```

2. Создайте ресурс `ReplicatedStoragePool` с именем `data-unsafe` и добавьте в него ресурсы `LVMVolumeGroup`,
   чтобы на всех узлах в модуле `sds-replicated-volume` в `ReplicatedStorageClass` с параметром `replication: Availability` или
   `replication: ConsistencyAndAvailability` использовалась только VG `main-unsafe`:

   ```yaml
   kubectl apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStoragePool
   metadata:
     name: data-unsafe
   spec:
     type: LVM
     lvmVolumeGroups:
       - name: vg-deccf08a-44d4-45f2-aea9-6232c0eeef91
       - name: vg-e0f00cab-03b3-49cf-a2f6-595628a2593c
       - name: vg-fe679d22-2bc7-409c-85a9-9f0ee29a6ca2
   EOF
   ```

3. Создайте ресурс `ReplicatedStorageClass` и в поле `storagePool` укажите имя созданных ранее ресурсов `ReplicatedStoragePool`,
   чтобы на всех узлах использовались VG `main-safe` и `main-unsafe`:

   ```yaml
   kubectl apply -f -<<EOF
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-r1
   spec:
     storagePool: data-safe # Обратите внимание, что из-за replication: None для этого ресурса используется data-safe; следовательно, репликация данных для постоянных томов (PV), созданных с этим StorageClass, проводиться не будет
     replication: None
     reclaimPolicy: Delete
     topology: Ignored # Если указать данную топологию, в кластере не должно быть зон (узлов с метками topology.kubernetes.io/zone)
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-r2
   spec:
     storagePool: data-unsafe # Обратите внимание, что из-за replication: Availability для этого ресурса используется data-unsafe; следовательно, будет проводиться репликация данных для PV, созданных с этим StorageClass
     replication: Availability
     reclaimPolicy: Delete
     topology: Ignored
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-r3
   spec:
     storagePool: data-unsafe # Обратите внимание, что из-за replication: ConsistencyAndAvailability для этого ресурса используется data-unsafe; следовательно, будет проводиться репликация данных для PV, созданных с этим StorageClass
     replication: ConsistencyAndAvailability
     reclaimPolicy: Delete
     topology: Ignored
   EOF
   ```

## Комбинированное хранилище

Комбинированное хранилище предполагает одновременное использование на узле дисков разных типов.

В ситуации, когда для создания хранилища комбинируются диски разных типов, мы рекомендуем сделать зеркало из дисков одного типа и установить на него операционную систему по сценарию [«Полное зеркало»](#полное-зеркало), но не использовать для SDS.

Для SDS используйте диски других типов (далее — дополнительные диски), отличающихся от тех, что используются для зеркала под операционную систему.

Рекомендации по использованию дополнительных дисков в зависимости от их типа:

|Тип диска|Рекомендуемые цели использования|
|---------|--------------------------------|
|NVMe SSD |Создание томов, требующих высокой производительности|
|SATA SSD |Создание томов, не требующих высокой производительности|
|HDD      |Создание томов, не требующих высокой производительности|

Дополнительные диски можно настраивать по любому из сценариев «Полное зеркало» либо «Частичное зеркало».

Ниже будет рассмотрен процесс настройки дополнительных дисков на примере следующих типов:

- NVMe SSD.
- SATA SSD.
- HDD.

### Настройка дополнительных дисков (Полное зеркало)

{% alert level="warning" %}
Ниже описан порядок действий по настройке дополнительных дисков для случая первичного развертывания и конфигурирования кластера при подключении к узлам по SSH.
Если у вас уже есть работающий кластер и вы добавляете на его узлы дополнительные диски, рекомендуется создавать и настраивать VG с помощью ресурса [LVMVolumeGroup](./usage.html#создание-ресурса-lvmvolumegroup), вместо выполнения на узле приведенных ниже команд.
{% endalert %}

Чтобы настроить дополнительные диски на узле по сценарию «Полное зеркало», выполните следующее:

1. Соберите зеркало из всех дополнительных дисков определенного типа целиком (аппаратно или программно).
2. Создайте VG с именем `<vg-name>` на зеркале.
3. Установите тег `storage.deckhouse.io/enabled=true` для VG `<vg-name>`, используя следующую команду:

   ```shell
   vgchange <vg-name> --addtag storage.deckhouse.io/enabled=true
   ```

{% alert level="info" %}
В примере выше `<vg-name>` замените на информативное имя, в зависимости от типа дополнительных дисков.

Примеры имен VG для дополнительных дисков разных типов:

- `ssd-nvme` — для дисков NVMe SSD.
- `ssd-sata` — для дисков SATA SSD.
- `hdd` — для дисков HDD.
{% endalert %}

#### Пример настройки модулей SDS (комбинированное хранилище, «Полное зеркало»)

В данном примере предполагается, что вы настроили три узла по сценарию «Полное зеркало».
В кластере Deckhouse при этом появятся три ресурса `LVMVolumeGroup` со случайно сгенерированными именами.
В будущем добавится возможность указывать имя для ресурсов `LVMVolumeGroup`,
которые создаются в процессе автоматического обнаружения VG, с помощью тега `LVM` с желаемым именем ресурса.

Чтобы вывести список ресурсов `LVMVolumeGroup`, выполните следующую команду:

```shell
kubectl get lvmvolumegroups.storage.deckhouse.io
```

В результате будет выведен список вида:

```console
NAME                                      THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE      ALLOCATED SIZE   VG          AGE
vg-08d3730c-9201-428d-966c-45795cba55a6   0/0         True                    Ready   worker-2   25596Mi   0                <vg-name>   61s
vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d   0/0         True                    Ready   worker-0   25596Mi   0                <vg-name>   4m17s
vg-c7863e12-c143-42bb-8e33-d578ce50d6c7   0/0         True                    Ready   worker-1   25596Mi   0                <vg-name>   108s
```

где `<vg-name>` — имя, присвоенное VG на зеркале на предыдущем шаге.

##### Настройка модуля `sds-local-volume` (комбинированное хранилище, «Полное зеркало»)

Чтобы настроить модуль `sds-local-volume` по сценарию «Полное зеркало», создайте ресурс `LocalStorageClass`
и добавьте в него все ресурсы `LVMVolumeGroup`, чтобы VG `<vg-name>` использовалась на всех узлах в модуле `sds-local-volume`:

```yaml
kubectl apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: LocalStorageClass
metadata:
  name: <local-storage-class-name>
spec:
  lvm:
    lvmVolumeGroups:
      - name: vg-08d3730c-9201-428d-966c-45795cba55a6
      - name: vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d
      - name: vg-c7863e12-c143-42bb-8e33-d578ce50d6c7
    type: Thick
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
EOF
```

{% alert level="info" %}
В примере выше `<local-storage-class-name>` замените на информативное имя, в зависимости от типа дополнительных дисков.

Примеры информативных имен ресурса `LocalStorageClass` для дополнительных дисков разных типов:

- `local-sc-ssd-nvme` — для дисков NVMe SSD.
- `local-sc-ssd-sata` — для дисков SATA SSD.
- `local-sc-ssd-hdd` — для дисков HDD.
{% endalert %}

##### Настройка модуля `sds-replicated-volume` (комбинированное хранилище, «Полное зеркало»)

Чтобы настроить модуль `sds-replicated-volume` по сценарию «Полное зеркало», выполните следующее:

1. Создайте ресурс `ReplicatedStoragePool` и добавьте в него все ресурсы `LVMVolumeGroup`,
   чтобы VG `<vg-name>` использовалась на всех узлах в модуле `sds-replicated-volume`:

   ```yaml
   kubectl apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStoragePool
   metadata:
     name: <replicated-storage-pool-name>
   spec:
     type: LVM
     lvmVolumeGroups:
       - name: vg-08d3730c-9201-428d-966c-45795cba55a6
       - name: vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d
       - name: vg-c7863e12-c143-42bb-8e33-d578ce50d6c7
   EOF
   ```

   > В примере выше `<replicated-storage-pool-name>` замените на информативное имя, в зависимости от типа дополнительных дисков.
   >
   > Примеры информативных имен ресурса `ReplicatedStoragePool` для дополнительных дисков разных типов:
   >
   > - `data-ssd-nvme` — для дисков NVMe SSD.
   > - `data-ssd-sata` — для дисков SATA SSD.
   > - `data-hdd` — для дисков HDD.

2. Создайте ресурс `ReplicatedStorageClass` и в поле `storagePool` укажите имя созданного ранее ресурса `ReplicatedStoragePool`:

   ```yaml
   kubectl apply -f -<<EOF
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-ssd-nvme-r1
   spec:
     storagePool: <replicated-storage-pool-name>
     replication: None
     reclaimPolicy: Delete
     topology: Ignored # Если указать данную топологию, в кластере не должно быть зон (узлов с метками topology.kubernetes.io/zone)
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-ssd-nvme-r2
   spec:
     storagePool: <replicated-storage-pool-name>
     replication: Availability
     reclaimPolicy: Delete
     topology: Ignored
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-ssd-nvme-r3
   spec:
     storagePool: <replicated-storage-pool-name>
     replication: ConsistencyAndAvailability
     reclaimPolicy: Delete
     topology: Ignored
   EOF
   ```

### Настройка дополнительных дисков (Частичное зеркало)

{% alert level="warning" %}
Не поддерживается использование разделов с одинаковыми PARTUUID,
а также изменение PARTUUID раздела, который используется для создания VG.
При создании таблицы разделов рекомендуется выбрать формат `GPT`,
так как PARTUUID в `MBR` является псевдослучайным и содержит в себе номер раздела.
Помимо этого, в `MBR` нельзя задать атрибут PARTLABEL,
который может пригодиться для последующей идентификации раздела в Deckhouse.
{% endalert %}

{% alert level="warning" %}
Ниже описан порядок действий по настройке дополнительных дисков для случая первичного развертывания и конфигурирования кластера при подключении к узлам по SSH.
Если у вас уже есть работающий кластер и вы добавляете на его узлы дополнительные диски, рекомендуется создавать и настраивать VG с помощью ресурса [LVMVolumeGroup](./usage.html#создание-ресурса-lvmvolumegroup), вместо выполнения на узле приведенных ниже команд.
{% endalert %}

В данном сценарии используются два раздела на каждом диске:
один для хранения данных SDS, которые не реплицируются,
и другой для данных SDS, которые реплицируются.
Первый раздел каждого диска используется для создания зеркала, а второй – для создания отдельной VG без зеркалирования.
Это позволяет максимально эффективно использовать место на диске.

Чтобы настроить узел с дополнительными дисками по сценарию «Частичное зеркало», выполните следующее:

1. создайте по два раздела на каждом дополнительном диске;
2. соберите зеркало из первых разделов на каждом диске;
3. создайте VG с именем `<vg-name>-safe` на зеркале;
4. создайте VG с именем `<vg-name>-unsafe` из вторых разделов каждого диска;
5. установите тег `storage.deckhouse.io/enabled=true` для VG `<vg-name>-safe` и `<vg-name>-unsafe`, используя следующую команду:

   ```shell
   vgchange <vg-name>-safe --addtag storage.deckhouse.io/enabled=true
   vgchange <vg-name>-unsafe --addtag storage.deckhouse.io/enabled=true
   ```

   > В примере выше `<vg-name>` замените на информативный префикс, в зависимости от типа дополнительных дисков.
   >
   > Примеры информативного префикса `<vg-name>` для дополнительных дисков разных типов:
   >
   > - `ssd-nvme` — для дисков NVMe SSD.
   > - `ssd-sata` — для дисков SATA SSD.
   > - `hdd` — для дисков HDD.

#### Пример настройки модулей SDS (комбинированное хранилище, «Частичное зеркало»)

В данном примере предполагается, что вы настроили три узла по сценарию «Частичное зеркало».
В кластере Deckhouse при этом появятся шесть ресурсов `LVMVolumeGroup` со случайно сгенерированными именами.
В будущем добавится возможность указывать имя для ресурсов `LVMVolumeGroup`,
которые создаются в процессе автоматического обнаружения VG, с помощью тега `LVM` с желаемым именем ресурса.

Чтобы вывести список ресурсов `LVMVolumeGroup`, выполните следующую команду:

```shell
kubectl get lvmvolumegroups.storage.deckhouse.io
```

В результате будет выведен список вида:

```console
NAME                                      THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE      ALLOCATED SIZE   VG                AGE
vg-08d3730c-9201-428d-966c-45795cba55a6   0/0         True                    Ready   worker-2   25596Mi   0                <vg-name>-safe     61s
vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d   0/0         True                    Ready   worker-0   25596Mi   0                <vg-name>-safe     4m17s
vg-c7863e12-c143-42bb-8e33-d578ce50d6c7   0/0         True                    Ready   worker-1   25596Mi   0                <vg-name>-safe     108s
vg-deccf08a-44d4-45f2-aea9-6232c0eeef91   0/0         True                    Ready   worker-2   25596Mi   0                <vg-name>-unsafe   61s
vg-e0f00cab-03b3-49cf-a2f6-595628a2593c   0/0         True                    Ready   worker-0   25596Mi   0                <vg-name>-unsafe   4m17s
vg-fe679d22-2bc7-409c-85a9-9f0ee29a6ca2   0/0         True                    Ready   worker-1   25596Mi   0                <vg-name>-unsafe   108s
```

где `<vg-name>` — префикс имени, присвоенного VG, созданным на предыдущем шаге.

##### Настройка модуля `sds-local-volume` (комбинированное хранилище, «Частичное зеркало»)

Чтобы настроить модуль `sds-local-volume` по сценарию «Частичное зеркало», создайте ресурс `LocalStorageClass`
и добавьте в него ресурсы `LVMVolumeGroup`, чтобы на всех узлах в модуле `sds-local-volume` использовалась только VG `<vg-name>-safe`:

```yaml
kubectl apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: LocalStorageClass
metadata:
  name: <local-storage-class-name>
spec:
  lvm:
    lvmVolumeGroups:
      - name: vg-08d3730c-9201-428d-966c-45795cba55a6
      - name: vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d
      - name: vg-c7863e12-c143-42bb-8e33-d578ce50d6c7
    type: Thick
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
EOF
```

{% alert level="info" %}
В примере выше `<local-storage-class-name>` замените на информативное имя, в зависимости от типа дополнительных дисков.

Примеры информативных имен ресурса `LocalStorageClass` для дополнительных дисков разных типов:

- `local-sc-ssd-nvme` — для дисков NVMe SSD.
- `local-sc-ssd-sata` — для дисков SATA SSD.
- `local-sc-hdd` — для дисков HDD.
{% endalert %}

##### Настройка модуля `sds-replicated-volume` (комбинированное хранилище, «Частичное зеркало»)

Чтобы настроить модуль `sds-replicated-volume` по сценарию «Частичное зеркало», выполните следующее:

1. Создайте ресурс `ReplicatedStoragePool` с именем `data-<vg-name>-safe` и добавьте в него ресурсы `LVMVolumeGroup`,
   чтобы на всех узлах в модуле `sds-replicated-volume` в `ReplicatedStorageClass` с параметром `replication: None`
   использовалась только VG `<vg-name>-safe`:

   ```yaml
   kubectl apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStoragePool
   metadata:
     name: data-<vg-name>-safe
   spec:
     type: LVM
     lvmVolumeGroups:
       - name: vg-08d3730c-9201-428d-966c-45795cba55a6
       - name: vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d
       - name: vg-c7863e12-c143-42bb-8e33-d578ce50d6c7
   EOF
   ```

   > В примере выше `data-<vg-name>-safe` замените на информативное имя, в зависимости от типа дополнительных дисков.
   >
   > Примеры информативных имен ресурса `ReplicatedStoragePool` для дополнительных дисков разных типов:
   >
   > - `data-ssd-nvme-safe` — для дисков NVMe SSD.
   > - `data-ssd-sata-safe` — для дисков SATA SSD.
   > - `data-hdd-safe` — для дисков HDD.

2. Создайте ресурс `ReplicatedStoragePool` с именем `data-<vg-name>-unsafe` и добавьте в него ресурсы `LVMVolumeGroup`,
   чтобы на всех узлах в модуле `sds-replicated-volume` в `ReplicatedStorageClass` с параметром `replication: Availability` или
   `replication: ConsistencyAndAvailability` использовалась только VG `<vg-name>-unsafe`:

   ```yaml
   kubectl apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStoragePool
   metadata:
     name: data-<vg-name>-unsafe
   spec:
     type: LVM
     lvmVolumeGroups:
       - name: vg-deccf08a-44d4-45f2-aea9-6232c0eeef91
       - name: vg-e0f00cab-03b3-49cf-a2f6-595628a2593c
       - name: vg-fe679d22-2bc7-409c-85a9-9f0ee29a6ca2
   EOF
   ```

   > В примере выше `data-<vg-name>-unsafe` замените на информативное имя, в зависимости от типа дополнительных дисков.
   >
   > Примеры информативных имен ресурса `ReplicatedStoragePool` для дополнительных дисков разных типов:
   >
   > - `data-ssd-nvme-unsafe` — для дисков NVMe SSD.
   > - `data-ssd-sata-unsafe` — для дисков SATA SSD.
   > - `data-hdd-unsafe` — для дисков HDD.

3. Создайте ресурс `ReplicatedStorageClass` и в поле `storagePool` укажите имя созданных ранее ресурсов `ReplicatedStoragePool`,
   чтобы на всех узлах использовались VG `<vg-name>-safe` и `<vg-name>-unsafe`:

   ```yaml
   kubectl apply -f -<<EOF
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-ssd-nvme-r1
   spec:
     storagePool: data-<vg-name>-safe # Обратите внимание, что из-за replication: None для этого ресурса используется data-<vg-name>-safe; следовательно, репликация данных для PV, созданных с этим StorageClass, проводиться не будет
     replication: None
     reclaimPolicy: Delete
     topology: Ignored # Если указать данную топологию, в кластере не должно быть зон (узлов с метками topology.kubernetes.io/zone)
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-ssd-nvme-r2
   spec:
     storagePool: data-<vg-name>-unsafe # Обратите внимание, что из-за replication: Availability для этого ресурса используется data-<vg-name>-unsafe; следовательно, будет проводиться репликация данных для PV, созданных с этим StorageClass
     replication: Availability
     reclaimPolicy: Delete
     topology: Ignored
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-ssd-nvme-r3
   spec:
     storagePool: data-<vg-name>-unsafe # Обратите внимание, что из-за replication: ConsistencyAndAvailability для этого ресурса используется data-<vg-name>-unsafe; следовательно, будет проводиться репликация данных для PV, созданных с этим StorageClass
     replication: ConsistencyAndAvailability
     reclaimPolicy: Delete
     topology: Ignored
   EOF
   ```

   > В примере выше `data-<vg-name>-unsafe` замените на информативное имя, в зависимости от типа дополнительных дисков.
   >
   > Примеры информативных имен ресурса `ReplicatedStoragePool` для дополнительных дисков разных типов:
   >
   > - `data-ssd-nvme-unsafe` — для дисков NVMe SSD.
   > - `data-ssd-sata-unsafe` — для дисков SATA SSD.
   > - `data-hdd-unsafe` — для дисков HDD.
   >
   > `data-<vg-name>-safe` замените на информативное имя, в зависимости от типа дополнительных дисков.
   >
   > Примеры информативных имен ресурса `ReplicatedStoragePool` для дополнительных дисков разных типов:
   >
   > - `data-ssd-nvme-safe` — для дисков NVMe SSD.
   > - `data-ssd-sata-safe` — для дисков SATA SSD.
   > - `data-hdd-safe` — для дисков HDD.
