---
title: "Образы виртуальных машин"
permalink: ru/user/virtualization/images.html
description: "Образы виртуальных машин в проекте: источники и варианты хранения, создание с HTTP-сервера, из реестра контейнеров, из диска и снимка диска."
search: образы ВМ, VirtualImage, создание образа, загрузка образа
lang: ru
---

Образ хранит содержимое диска, из которого вы создаёте диски виртуальных машин. Образ [VirtualImage](/modules/virtualization/cr.html#virtualimage) создаётся в проекте и доступен только в том проекте или неймспейсе, где он создан.

{% alert level="warning" %}
Чтобы один и тот же образ был доступен всем проектам кластера, нужен кластерный образ [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage). Создать его может администратор, порядок описан в разделе [«Кластерные образы виртуальных машин»](../../admin/configuration/virtualization/cluster-images.html).
{% endalert %}

Виртуальная машина получает доступ к подключённому образу в режиме «только чтение».

Образ появляется в проекте в три шага:

1. Вы создаёте ресурс [VirtualImage](/modules/virtualization/cr.html#virtualimage) и указываете в нём источник данных.
1. Модуль загружает образ из этого источника в хранилище, которым в зависимости от выбранного типа выступает DVCR или PVC.
1. Загруженный образ становится доступен для создания дисков.

## Источники и варианты хранения

Источником образа может быть HTTP-сервер с файлом образа, хранилище образов контейнеров или файл на вашем компьютере, который вы загружаете из командной строки. Кроме того, образ можно создать из другого образа, из диска виртуальной машины или из снимка диска.

Виды образов, поддерживаемые форматы файлов и алгоритмы сжатия описаны в разделе [«Типы и форматы образов»](../../admin/configuration/virtualization/cluster-images.html#типы-и-форматы-образов).

Загруженный образ хранится одним из двух способов, который задаёт параметр [`.spec.storage`](/modules/virtualization/cr.html#virtualimage-v1alpha2-spec-storage):

- `ContainerRegistry` — вариант по умолчанию, образ хранится в DVCR.
- `PersistentVolumeClaim` — образ хранится в PVC. Этот вариант предпочтителен, если хранилище умеет быстро клонировать PVC, потому что диски из такого образа создаются быстрее.

{% alert level="warning" %}
Образ, сохранённый с параметром `storage: PersistentVolumeClaim`, годится для создания дисков только в том же классе хранения (StorageClass).
{% endalert %}

Ход создания образа показывает колонка `PHASE` в выводе `d8 k get vi`, её значения описаны в поле [`.status.phase`](/modules/virtualization/cr.html#virtualimage-v1alpha2-status-phase). Следить за созданием в реальном времени помогает ключ `-w`, а если образ надолго остаётся не готов, причину подскажет блок [`.status.conditions`](/modules/virtualization/cr.html#virtualimage-v1alpha2-status-conditions) и команда `d8 k describe vi`.

Пока образ не перешёл в фазу `Ready`, блок `.spec` можно менять, и после изменения загрузка начнётся заново. У готового образа блок `.spec` изменить уже нельзя. Все параметры образа описаны в [VirtualImage](/modules/virtualization/cr.html#virtualimage).

## Создание образа с HTTP-сервера

Проще всего создать образ, указав ссылку на файл, который лежит на HTTP-сервере.

{% tabs vi-http %}

{% tab "В командной строке" %}

1. Создайте ресурс [VirtualImage](/modules/virtualization/cr.html#virtualimage). В примере образ сохраняется в DVCR:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualImage
   metadata:
     name: ubuntu-24-04
   spec:
     # Сохраняем образ в DVCR.
     storage: ContainerRegistry
     # Источник для создания образа.
     dataSource:
       type: HTTP
       http:
         url: https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
   EOF
   ```

1. Проверьте, что образ создан:

   ```bash
   d8 k get virtualimage ubuntu-24-04

   # Короткий вариант команды.
   d8 k get vi ubuntu-24-04
   ```

   Пример вывода:

   ```console
   NAME           PHASE   CDROM   PROGRESS   AGE
   ubuntu-24-04   Ready   false   100%       23h
   ```

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Виртуализация» → «Образы».
1. Нажмите кнопку «Создать».
1. В блоке «Источник» выберите «По ссылке».
1. В открывшейся форме в поле «Имя образа» введите имя образа.
1. В блоке «Хранилище» в поле «Тип хранилища» выберите `ContainerRegistry`.
1. В поле «URL» укажите ссылку на образ.
1. Нажмите кнопку «Создать».
1. Проверьте статус образа на его странице.

{% endtab %}

{% endtabs %}

### Проверка целостности загруженного образа

Блок [`checksum`](/modules/virtualization/cr.html#virtualimage-v1alpha2-spec-datasource-http-checksum) заставляет модуль проверить то, что он скачал с HTTP-сервера. Образ перейдёт в фазу `Ready`, только если загруженный файл совпал со всеми указанными контрольными суммами, иначе ресурс окажется в фазе `Failed`:

```bash
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualImage
metadata:
  name: ubuntu-24-04
spec:
  storage: ContainerRegistry
  dataSource:
    type: HTTP
    http:
      url: https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
      checksum:
        sha256: 78be890d71dde316c412da2ce8332ba47b9ce7a29d573801d2777e01aa20b9b5
EOF
```

Возьмите контрольную сумму у зеркала, которое публикует образ, и укажите её в поле соответствующего алгоритма:

| Поле          | Алгоритм                               | Скорость проверки |
| ------------- | -------------------------------------- | ----------------- |
| `sha1`        | SHA-1                                  | ~1,6 ГБ/с         |
| `sha256`      | SHA-256                                | ~1,5 ГБ/с         |
| `md5`         | MD5                                    | ~700 МБ/с         |
| `sha512`      | SHA-512                                | ~570 МБ/с         |
| `streebog256` | ГОСТ Р 34.11-2012 («Стрибог»), 256 бит | ~17 МБ/с          |
| `streebog512` | ГОСТ Р 34.11-2012 («Стрибог»), 512 бит | ~17 МБ/с          |

Скорости даны как порядок величины, а не как обещание. Они измерены на процессоре x86-64 с набором инструкций SHA, а процессор без него считает SHA-1 и SHA-256 в несколько раз медленнее. На любом процессоре сохраняется другое, а именно расстояние между строками. SHA-1 и SHA-256 вычисляются отдельными инструкциями, MD5 и SHA-512 написанным вручную ассемблером, и все четыре хешируют данные быстрее, чем те приходят по сети, поэтому их стоимость остаётся незаметной на фоне самой загрузки.

Алгоритмы «Стрибог» аппаратной поддержки не имеют нигде и медленнее примерно на два порядка. Для образа в 10 ГиБ это около десяти минут одного только хеширования, и создание образа упирается уже в процессор, а не в сеть. Указывайте их только тогда, когда контрольная сумма по ГОСТ действительно требуется. Обе длины стоят одинаково, потому что ГОСТ Р 34.11-2012 использует одну и ту же функцию сжатия и для 256, и для 512 бит, а короткий вариант отличается только начальным значением.

Контрольные суммы считаются за один проход по загружаемым данным, поэтому указание нескольких сумм сразу стоит суммы их времён. Незаполненные поля не стоят ничего.

Тот же блок доступен для источника `Upload` в параметре [`dataSource.upload.checksum`](/modules/virtualization/cr.html#virtualimage-v1alpha2-spec-datasource-upload-checksum) и работает так же. Загружаемые данные проверяются по всем указанным контрольным суммам, а при несовпадении ресурс остаётся в фазе `Failed`.

### Хранение образа в PVC

Чтобы диски создавались из образа быстрее, храните его в PVC. Тогда модуль сможет клонировать том вместо повторной распаковки.

{% tabs vi-pvc %}

{% tab "В командной строке" %}

1. Создайте ресурс [VirtualImage](/modules/virtualization/cr.html#virtualimage) с типом хранения `PersistentVolumeClaim`:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualImage
   metadata:
     name: ubuntu-24-04-pvc
   spec:
     # Настройки хранения проектного образа.
     storage: PersistentVolumeClaim
     persistentVolumeClaim:
       # Подставьте название своего StorageClass.
       storageClassName: rv-thin-r2
     # Источник для создания образа.
     dataSource:
       type: HTTP
       http:
         url: https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
   EOF
   ```

   Если параметр [`.spec.persistentVolumeClaim.storageClassName`](/modules/virtualization/cr.html#virtualimage-v1alpha2-spec-persistentvolumeclaim-storageclassname) не указан, модуль возьмёт StorageClass по умолчанию на уровне кластера либо класс, заданный для образов в [настройках модуля](../../admin/configuration/virtualization/storage-classes.html).

1. Проверьте, что образ создан:

   ```bash
   d8 k get vi ubuntu-24-04-pvc
   ```

   Пример вывода:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME               PHASE   CDROM   PROGRESS   AGE
   ubuntu-24-04-pvc   Ready   false   100%       23h
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Виртуализация» → «Образы».
1. Нажмите кнопку «Создать».
1. В блоке «Источник» выберите «По ссылке».
1. В открывшейся форме в поле «Имя образа» введите имя образа.
1. В блоке «Хранилище» в поле «Тип хранилища» выберите `PersistentVolumeClaim`.
1. В поле «Класс хранилища» выберите StorageClass или оставьте вариант по умолчанию.
1. В поле «URL» укажите ссылку на образ.
1. Нажмите кнопку «Создать».
1. Проверьте статус образа на его странице.

{% endtab %}

{% endtabs %}

## Создание образа из хранилища образов контейнеров

Модуль умеет забирать образ из внешнего хранилища образов контейнеров, но файл диска должен лежать в образе контейнера по пути `/disk`. Ниже показано, как подготовить такой образ контейнера и создать из него образ проекта.

{% tabs vi-registry %}

{% tab "В командной строке" %}

1. Скачайте файл образа на локальную машину:

   ```bash
   curl -L https://cloud-images.ubuntu.com/minimal/releases/noble/release/ubuntu-24.04-minimal-cloudimg-amd64.img -o ubuntu2404.img
   ```

1. Создайте `Dockerfile` со следующим содержимым:

   ```Dockerfile
   FROM scratch
   COPY ubuntu2404.img /disk/ubuntu2404.img
   ```

1. Соберите образ контейнера. В примере используется хранилище [docker.com](https://www.docker.com/), для работы с которым нужны учётная запись и настроенное окружение:

   ```bash
   docker build -t docker.io/<USERNAME>/ubuntu2404:latest
   ```

   Здесь `<USERNAME>` — имя пользователя, указанное при регистрации в хранилище.

1. Загрузите собранный образ контейнера в хранилище:

   ```bash
   docker push docker.io/<USERNAME>/ubuntu2404:latest
   ```

1. Создайте ресурс [VirtualImage](/modules/virtualization/cr.html#virtualimage), указав путь к образу контейнера:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualImage
   metadata:
     name: ubuntu-2404
   spec:
     storage: ContainerRegistry
     dataSource:
       type: ContainerImage
       containerImage:
         image: docker.io/<USERNAME>/ubuntu2404:latest
   EOF
   ```

Модуль работает только с теми хранилищами, где включён TLS. Если хранилище использует собственный центр сертификации, передайте цепочку сертификатов в параметре [`caBundle`](/modules/virtualization/cr.html#virtualimage-v1alpha2-spec-datasource-containerimage-cabundle), а учётные данные для доступа к закрытому хранилищу возьмите из секрета, указанного в параметре `imagePullSecret`.

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Виртуализация» → «Образы».
1. Нажмите кнопку «Создать».
1. В блоке «Источник» выберите «Из реестра».
1. В открывшейся форме в поле «Имя образа» введите имя образа.
1. В блоке «Хранилище» в поле «Тип хранилища» выберите `ContainerRegistry`.
1. В поле «Образ в реестре контейнеров» укажите путь к образу контейнера.
1. Нажмите кнопку «Создать».
1. Проверьте статус образа на его странице.

{% endtab %}

{% endtabs %}

## Загрузка образа из командной строки

Если файл образа лежит на вашем компьютере, загрузите его напрямую. Модуль создаёт для этого временную точку приёма данных и ждёт загрузки.

{% tabs vi-upload %}

{% tab "В командной строке" %}

1. Создайте ресурс [VirtualImage](/modules/virtualization/cr.html#virtualimage) с источником `Upload`:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualImage
   metadata:
     name: some-image
   spec:
     # Настройки хранения проектного образа.
     storage: ContainerRegistry
     # Настройки источника образа.
     dataSource:
       type: Upload
   EOF
   ```

   Ресурс перейдёт в фазу `WaitForUserUpload` и будет готов принять файл. Начните загрузку в течение 10 минут, иначе ресурс перейдёт в фазу `Failed` и его придётся создать заново.

1. Получите адреса, по которым принимается файл:

   ```bash
   d8 k get vi some-image -o jsonpath="{.status.imageUploadURLs}" | jq
   ```

   Пример вывода:

   <!-- markdownlint-disable MD031 -->
   ```console
   {
     "external": "https://virtualization.example.com/upload/<SECRET_URL>",
     "inCluster": "http://10.222.165.239/upload"
   }
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

   Адрес `inCluster` используйте, если загружаете файл с одного из узлов кластера, а `external` — во всех остальных случаях.

1. Загрузите файл по выбранному адресу. В примере сначала скачивается образ Cirros, а затем отправляется в кластер:

   ```bash
   curl -L http://download.cirros-cloud.net/0.5.1/cirros-0.5.1-x86_64-disk.img -o cirros.img
   curl https://virtualization.example.com/upload/<SECRET_URL> --progress-bar -T cirros.img | cat
   ```

   Здесь `<SECRET_URL>` — последняя часть адреса из предыдущего шага.

1. Убедитесь, что образ перешёл в фазу `Ready`:

   ```bash
   d8 k get vi some-image
   ```

   Пример вывода:

   ```console
   NAME         PHASE   CDROM   PROGRESS   AGE
   some-image   Ready   false   100%       1m
   ```

Загруженный файл тоже можно сверить с контрольной суммой, для этого задайте блок [`checksum`](/modules/virtualization/cr.html#virtualimage-v1alpha2-spec-datasource-upload-checksum) в источнике данных. Суммы считаются по байтам, которые передаёт клиент, а при несовпадении ресурс остаётся в фазе `Failed`, и загрузку придётся повторить на пересозданном ресурсе.

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Виртуализация» → «Образы».
1. Нажмите кнопку «Создать», затем в блоке «Источник» выберите «Загрузить».
1. В поле «Имя образа» введите имя образа.
1. В блоке «Загрузить файл» перетащите файл в выделенную область или нажмите «выберите на вашем компьютере».
1. Выберите файл в открывшемся файловом менеджере.
1. Нажмите кнопку «Создать».
1. Дождитесь, когда образ перейдёт в состояние «Готов».

{% endtab %}

{% endtabs %}

## Создание образа из диска

Образ можно создать из [диска](disks.html), если диск не подключён ни к одной виртуальной машине либо машина, к которой он подключён, выключена.

{% tabs vi-from-disk %}

{% tab "В командной строке" %}

Создайте образ, указав источником нужный диск:

```bash
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualImage
metadata:
  name: linux-vm-root
spec:
  storage: ContainerRegistry
  dataSource:
    type: ObjectRef
    objectRef:
      kind: VirtualDisk
      name: linux-vm-root
EOF
```

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Виртуализация» → «Образы».
1. Нажмите кнопку «Создать».
1. В блоке «Источник» выберите «Создать из».
1. В открывшейся форме в поле «Имя образа» введите `linux-vm-root`.
1. В блоке «Хранилище» в поле «Тип хранилища» выберите `ContainerRegistry`.
1. В поле «Источник» выберите нужный диск из выпадающего списка.
1. Нажмите кнопку «Создать».
1. Проверьте статус образа на его странице.

{% endtab %}

{% endtabs %}

## Создание образа из снимка диска

Образ можно создать из [снимка диска](snapshots.html#создание-снимков-дисков), если снимок находится в фазе `Ready`.

{% tabs vi-from-snapshot %}

{% tab "В командной строке" %}

Создайте образ, указав источником снимок диска:

```bash
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualImage
metadata:
  name: linux-vm-root
spec:
  storage: ContainerRegistry
  dataSource:
    type: ObjectRef
    objectRef:
      kind: VirtualDiskSnapshot
      name: linux-vm-root-snapshot
EOF
```

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Виртуализация» → «Образы».
1. Нажмите кнопку «Создать».
1. В открывшейся форме в поле «Имя образа» введите имя образа.
1. В блоке «Хранилище» в поле «Тип хранилища» выберите тип хранения образа.
1. В блоке «Источник» выберите «Создать из».
1. В поле «Источник» раскройте список и в группе «Снимки дисков» выберите нужный снимок.
1. Нажмите кнопку «Создать».
1. Проверьте статус образа на его странице.

{% endtab %}

{% endtabs %}

Свойства образа удобно смотреть в веб-интерфейсе, в разделе «Виртуализация» → «Образы»:

- список показывает имя образа, статус, размер, формат в колонке «Тип» и область видимости в колонке «Доступность», а фильтры сужают его по статусу, образу и типу;
- на странице образа вкладка «Информация» собирает параметры создания в блоках «Хранилище» и «Источник», а в блоке «Состояние» показывает среднюю скорость загрузки, формат, распакованный размер, размер в хранилище, длительность создания, признак «CD-ROM» и путь к образу в DVCR;
- вкладки «Мета» и «YAML» показывают лейблы с аннотациями и полную спецификацию ресурса.
