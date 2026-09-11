---
title: "Диски виртуальных машин"
permalink: ru/user/virtualization/disks.html
description: "Диски виртуальных машин: влияние хранилища на поведение диска, создание пустого диска, создание из образа и загрузка из командной строки."
search: диски ВМ, VirtualDisk, создание диска, загрузка диска, WaitForFirstConsumer
lang: ru
---

Диск хранит данные виртуальной машины, включая операционную систему и файлы приложений. Описывает диск ресурс [VirtualDisk](/modules/virtualization/cr.html#virtualdisk), а его спецификация состоит из двух блоков:

- [`persistentVolumeClaim`](/modules/virtualization/cr.html#virtualdisk-v1alpha2-spec-persistentvolumeclaim) — параметры хранения, то есть StorageClass и размер;
- [`dataSource`](/modules/virtualization/cr.html#virtualdisk-v1alpha2-spec-datasource) — источник данных, которым может быть образ, другой диск или снимок.

Без блока `dataSource` создаётся пустой диск, и тогда в `persistentVolumeClaim` нужно указать хотя бы размер. Если источник задан, блок `persistentVolumeClaim` можно опустить, тогда размер модуль возьмёт из источника, а класс хранения подберёт по нему же. Когда подобрать класс не удаётся, модуль использует StorageClass по умолчанию на уровне кластера либо класс, заданный для дисков в [настройках модуля](../../admin/configuration/virtualization/storage-classes.html).

Ход создания диска показывает колонка `PHASE` в выводе `d8 k get vd`, её значения описаны в поле [`.status.phase`](/modules/virtualization/cr.html#virtualdisk-v1alpha2-status-phase). Если диск надолго остаётся не готов, причину подскажет блок [`.status.conditions`](/modules/virtualization/cr.html#virtualdisk-v1alpha2-status-conditions).

Пока диск не перешёл в фазу `Ready`, менять можно любые поля блока `.spec`, и после изменения создание начнётся заново. У готового диска остаются изменяемыми только размер и класс хранения в параметрах [`.spec.persistentVolumeClaim.size`](/modules/virtualization/cr.html#virtualdisk-v1alpha2-spec-persistentvolumeclaim-size) и [`.spec.persistentVolumeClaim.storageClassName`](/modules/virtualization/cr.html#virtualdisk-v1alpha2-spec-persistentvolumeclaim-storageclassname).

{% alert level="warning" %}
Создать диск из ISO-образа нельзя.
{% endalert %}

## Влияние хранилища на диск

Поведение диска зависит от того, какое хранилище стоит за выбранным StorageClass. Различия проявляются в двух свойствах.

Тип тома определяет формат, в котором модуль создаёт диск. На томах файловой системы (`FileSystem`, например NFS) диск создаётся в формате `qcow2`, на блочных устройствах (`Block`, например iSCSI или Ceph RBD) данные пишутся напрямую. Некоторые хранилища поддерживают оба типа.

Режим привязки тома определяет момент создания диска:

- `Immediate` — диск создаётся сразу, независимо от виртуальных машин, и подключить его можно к машине на любом узле кластера.

  ![VolumeBindingMode: Immediate](/images/virtualization/vd-immediate.ru.png)

- `WaitForFirstConsumer` — диск создаётся только после того, как его подключат к виртуальной машине, и размещается на узле, где эта машина запускается.

  ![VolumeBindingMode: WaitForFirstConsumer](/images/virtualization/vd-wffc.ru.png)

Остальные параметры, включая формат диска, модуль определяет сам по возможностям выбранного StorageClass.

Чтобы посмотреть доступные хранилища, выполните команду:

```bash
d8 k get storageclass
```

Пример вывода:

```console
NAME                   PROVISIONER                           RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
rv-thin-r1 (default)   replicated.csi.storage.deckhouse.io   Delete          Immediate              true                   48d
rv-thin-r2             replicated.csi.storage.deckhouse.io   Delete          Immediate              true                   48d
nfs-4-1-wffc           nfs.csi.k8s.io                        Delete          WaitForFirstConsumer   true                   30d
```
{: .nowrap-default }

В веб-интерфейсе тот же список доступен на вкладке «Система» в разделе «Хранилище» → «Классы хранилищ».

## Создание пустого диска

Пустой диск нужен, чтобы установить на него операционную систему или хранить данные отдельно от системного диска.

{% tabs vd-blank %}

{% tab "В командной строке" %}

1. Создайте ресурс [VirtualDisk](/modules/virtualization/cr.html#virtualdisk), указав размер и класс хранения:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualDisk
   metadata:
     name: blank-disk
   spec:
     # Настройки параметров хранения диска.
     persistentVolumeClaim:
       # Подставьте название своего StorageClass.
       storageClassName: rv-thin-r2
       size: 100Mi
   EOF
   ```

1. Проверьте, что диск создан:

   ```bash
   d8 k get vd blank-disk
   ```

   Пример вывода:

   ```console
   NAME         PHASE   CAPACITY   VIRTUALMACHINE   AGE
   blank-disk   Ready   100Mi                       1m2s
   ```
   {: .nowrap-default }

{% endtab %}

{% tab "В веб-интерфейсе" %}

Шаг можно пропустить и создать диск сразу при создании ВМ.

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Виртуализация» → «Диски».
1. Нажмите кнопку «Создать».
1. В открывшейся форме в поле «Имя диска» введите `blank-disk`.
1. В поле «Размер» задайте размер с единицами измерения, например `100Mi`.
1. В поле «Класс хранения» выберите StorageClass или оставьте вариант по умолчанию.
1. Нажмите кнопку «Создать».
1. Проверьте статус диска на его странице.

{% endtab %}

{% endtabs %}

## Создание диска из образа

Диск можно заполнить данными из образа, созданного ранее, будь то проектный [VirtualImage](/modules/virtualization/cr.html#virtualimage) или кластерный [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage).

Размер диска указывать необязательно. Если вы его не задали, модуль создаст диск ровно по распакованному размеру образа, а если задали, размер должен быть не меньше распакованного.

{% tabs vd-from-image %}

{% tab "В командной строке" %}

1. Посмотрите распакованный размер образа в колонке `UNPACKEDSIZE`:

   ```bash
   d8 k get vi ubuntu-24-04 -o wide
   ```

   Пример вывода:

   ```console
   NAME           PHASE   CDROM   PROGRESS   STOREDSIZE   UNPACKEDSIZE   REGISTRY URL                                                                              TARGETPVC   AGE
   ubuntu-24-04   Ready   false   100%       285.9Mi      2.5Gi          dvcr.d8-virtualization.svc/vi/default/ubuntu-24-04:eac95605-7e0b-4a32-bb50-cc7284fd89d0               122m
   ```
   {: .nowrap-default }

1. Создайте диск, задав размер больше распакованного:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualDisk
   metadata:
     name: linux-vm-root
   spec:
     # Настройки параметров хранения диска.
     persistentVolumeClaim:
       # Размер больше, чем распакованный размер образа.
       size: 10Gi
       # Подставьте название своего StorageClass.
       storageClassName: rv-thin-r2
     # Источник, из которого создаётся диск.
     dataSource:
       type: ObjectRef
       objectRef:
         kind: VirtualImage
         name: ubuntu-24-04
   EOF
   ```

1. Создайте второй диск, не указывая размер:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualDisk
   metadata:
     name: linux-vm-root-2
   spec:
     # Настройки параметров хранения диска.
     persistentVolumeClaim:
       # Подставьте название своего StorageClass.
       storageClassName: rv-thin-r2
     # Источник, из которого создаётся диск.
     dataSource:
       type: ObjectRef
       objectRef:
         kind: VirtualImage
         name: ubuntu-24-04
   EOF
   ```

1. Сравните размеры созданных дисков:

   ```bash
   d8 k get vd
   ```

   Пример вывода:

   ```console
   NAME              PHASE   CAPACITY   VIRTUALMACHINE   AGE
   linux-vm-root     Ready   10Gi                        7m52s
   linux-vm-root-2   Ready   2590Mi                      7m15s
   ```
   {: .nowrap-default }

   Первый диск получил заданные 10 ГиБ, второй — распакованный размер образа.

{% endtab %}

{% tab "В веб-интерфейсе" %}

Шаг можно пропустить и создать диск сразу при создании ВМ.

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Виртуализация» → «Диски».
1. Нажмите кнопку «Создать».
1. В открывшейся форме в поле «Имя диска» введите `linux-vm-root`.
1. В поле «Источник» из выпадающего списка выберите нужный образ.
1. При необходимости в поле «Размер» укажите больший размер или оставьте значение по умолчанию.
1. В поле «Класс хранения» выберите StorageClass или оставьте вариант по умолчанию.
1. Нажмите кнопку «Создать».
1. Проверьте статус диска на его странице.

{% endtab %}

{% endtabs %}

## Загрузка диска из командной строки

Если файл образа лежит на вашем компьютере, загрузите его сразу в диск. Модуль создаёт для этого временную точку приёма данных и ждёт загрузки.

{% tabs vd-upload %}

{% tab "В командной строке" %}

1. Создайте ресурс [VirtualDisk](/modules/virtualization/cr.html#virtualdisk) с источником `Upload`:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualDisk
   metadata:
     name: uploaded-disk
   spec:
     dataSource:
       type: Upload
   EOF
   ```

   Ресурс перейдёт в фазу `WaitForUserUpload` и будет готов принять файл. Начните загрузку в течение 10 минут, иначе ресурс перейдёт в фазу `Failed` и его придётся создать заново.

1. Получите адреса, по которым принимается файл:

   ```bash
   d8 k get vd uploaded-disk -o jsonpath="{.status.imageUploadURLs}" | jq
   ```

   Пример вывода:

   ```console
   {
     "external": "https://virtualization.example.com/upload/<SECRET_URL>",
     "inCluster": "http://10.222.165.239/upload"
   }
   ```
   {: .nowrap-default }

   Адрес `inCluster` используйте, если загружаете файл с одного из узлов кластера, а `external` — во всех остальных случаях.

1. Загрузите файл по выбранному адресу:

   ```bash
   curl https://virtualization.example.com/upload/<SECRET_URL> --progress-bar -T <IMAGE_FILE> | cat
   ```

   Здесь `<SECRET_URL>` — адрес из предыдущего шага, а `<IMAGE_FILE>` — путь к файлу образа на вашем компьютере.

1. Убедитесь, что диск перешёл в фазу `Ready`:

   ```bash
   d8 k get vd uploaded-disk
   ```

   Пример вывода:

   ```console
   NAME            PHASE   CAPACITY   VIRTUALMACHINE   AGE
   uploaded-disk   Ready   3Gi                         7d23h
   ```
   {: .nowrap-default }

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Виртуализация» → «Диски».
1. Нажмите кнопку «Создать».
1. В открывшейся форме в поле «Имя диска» введите имя диска.
1. В блоке «Диск» в поле «Источник данных» выберите «Загрузить».
1. Перетащите файл в выделенную область или нажмите на неё и выберите файл на компьютере.
1. В поле «Размер» укажите размер диска, а в поле «Класс хранения» — StorageClass.
1. Нажмите кнопку «Создать».

> Если у выбранного класса хранения режим привязки тома `WaitForFirstConsumer`, вариант «Загрузить» недоступен. Без потребителя диск не создаётся, и загружать файл некуда, поэтому выберите класс хранения с режимом `Immediate` либо загрузите данные как образ.
>
> По этой же причине пустой диск, созданный заранее с таким классом хранения, остаётся в статусе «ОЖИДАНИЕ ВМ» и не предлагается в окне «Диски / Образы» при подключении к виртуальной машине, ведь выбрать можно только готовый диск. Создавайте такие диски прямо из формы виртуальной машины вариантами «Пустой» или «Создать из».

{% endtab %}

{% endtabs %}

Свойства диска удобно смотреть в веб-интерфейсе, в разделе «Виртуализация» → «Диски»:

- список показывает имя диска, статус, размер, класс хранения в колонке «Класс», использующую диск виртуальную машину в колонке «Используется» и возраст ресурса;
- на странице диска вкладка «Конфигурация» показывает источник данных, размер, класс хранения и список ВМ в строке «Используется в»;
- вкладка «Диагностика» показывает имя PVC, его фазу, размер, класс хранения, имя PV и возраст, а также длительность этапов создания диска в блоке «Сводка диагностики».
