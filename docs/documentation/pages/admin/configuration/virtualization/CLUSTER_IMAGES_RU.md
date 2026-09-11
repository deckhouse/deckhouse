---
title: "Кластерные образы виртуальных машин"
permalink: ru/admin/configuration/virtualization/cluster-images.html
description: "Кластерные образы виртуальных машин: типы и форматы, создание образа с HTTP-сервера, из хранилища образов контейнеров и загрузкой из командной строки."
search: кластерный образ, ClusterVirtualImage, форматы образов, загрузка образа
lang: ru
---

Образ хранит содержимое диска, из которого владельцы проектов создают диски виртуальных машин. Кластерный образ [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) доступен во всех неймспейсах и проектах кластера, поэтому загруженный однажды образ используют сразу все проекты.

Образ появляется в кластере в три шага:

1. Администратор создаёт ресурс [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) и указывает в нём источник данных.
1. Модуль загружает образ из этого источника во внутреннее хранилище (DVCR).
1. Загруженный образ становится доступен для создания дисков.

Источником образа может быть HTTP-сервер с файлом образа, хранилище образов контейнеров или файл на вашем компьютере, который вы загружаете из командной строки. Кроме того, образ можно создать из другого образа, из диска виртуальной машины или из снимка диска.

Ход создания образа показывает колонка `PHASE` в выводе `d8 k get cvi`, её значения описаны в поле [`.status.phase`](/modules/virtualization/cr.html#clustervirtualimage-v1alpha2-status-phase). Следить за созданием в реальном времени помогает ключ `-w`, а если образ надолго остаётся не готов, причину подскажет блок [`.status.conditions`](/modules/virtualization/cr.html#clustervirtualimage-v1alpha2-status-conditions) и команда `d8 k describe cvi`.

Пока образ не перешёл в фазу `Ready`, блок `.spec` можно менять, и после изменения загрузка начнётся заново. У готового образа блок `.spec` изменить уже нельзя. Все параметры образа описаны в [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage).

## Типы и форматы образов

Образы бывают двух видов:

- **ISO-образ** — установочный образ для первоначальной установки операционной системы (ОС). Такие образы выпускают производители ОС и применяют их для установки на физические и виртуальные серверы.
- **Образ диска с предустановленной системой** — содержит уже установленную и настроенную ОС, готовую к работе сразу после создания виртуальной машины (ВМ). Такие образы публикуют разработчики дистрибутивов, либо вы готовите их самостоятельно.

Готовые образы с предустановленной системой публикуют разработчики дистрибутивов. В таблице приведены страницы загрузки и имена пользователей, которые заданы в этих образах по умолчанию:

<a id="image-resources-table"></a>

| Дистрибутив                                                                       | Пользователь по умолчанию |
| --------------------------------------------------------------------------------- | ------------------------- |
| [AlmaLinux](https://almalinux.org/get-almalinux/#Cloud_Images)                    | `almalinux`               |
| [AlpineLinux](https://alpinelinux.org/cloud/)                                     | `alpine`                  |
| [AltLinux](https://ftp.altlinux.ru/pub/distributions/ALTLinux/)                   | `altlinux`                |
| [AstraLinux](https://download.astralinux.ru/ui/native/mg-generic/alse/cloudinit/) | `astra`                   |
| [CentOS](https://cloud.centos.org/centos/)                                        | `cloud-user`              |
| [Debian](https://cdimage.debian.org/images/cloud/)                                | `debian`                  |
| [Rocky](https://rockylinux.org/download/)                                         | `rocky`                   |
| [Ubuntu](https://cloud-images.ubuntu.com/)                                        | `ubuntu`                  |

Модуль принимает файл образа в следующих форматах:

- `qcow2`;
- `raw`;
- `vmdk`;
- `vdi`;
- `vhd`;
- `vhdx`.

Образ можно передать сжатым алгоритмом `gz`, `xz` или `zst`, модуль распакует его при загрузке.

Тип и размер образа модуль определяет сам и записывает их в статус ресурса. Размеров два, и оба видны в выводе команды `d8 k get cvi -o wide`:

- `STOREDSIZE` — объём, который образ занимает в хранилище. Для образа, загруженного в сжатом виде, он меньше распакованного размера. По этой колонке удобно оценивать, сколько места образы занимают в DVCR.
- `UNPACKEDSIZE` — размер образа после распаковки. Он задаёт минимальный размер диска, который получится создать из этого образа.

{% alert level="info" %}
Создавая диск из образа, указывайте размер не меньше значения `UNPACKEDSIZE`.
Если размер не задан, диск создаётся ровно по распакованному размеру образа.
{% endalert %}

## Создание кластерного образа с HTTP-сервера

Проще всего создать образ, указав ссылку на файл, который лежит на HTTP-сервере.

{% tabs cvi-http %}

{% tab "В командной строке" %}

1. Создайте ресурс [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage):

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: ClusterVirtualImage
   metadata:
     name: ubuntu-24-04
   spec:
     # Источник для создания образа.
     dataSource:
       type: HTTP
       http:
         url: https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
   EOF
   ```

1. Проверьте, что образ создан:

   ```bash
   d8 k get clustervirtualimage ubuntu-24-04

   # Короткий вариант команды.
   d8 k get cvi ubuntu-24-04
   ```

   Пример вывода:

   ```console
   NAME           PHASE   CDROM   PROGRESS   AGE
   ubuntu-24-04   Ready   false   100%       23h
   ```

Чтобы модуль сверил скачанный файл с контрольной суммой, добавьте в источник блок [`checksum`](/modules/virtualization/cr.html#clustervirtualimage-v1alpha2-spec-datasource-http-checksum). Если файл не совпал ни с одной из указанных сумм, образ перейдёт в фазу `Failed`.

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Система», далее в раздел «Виртуализация» → «Кластерные образы».
1. Нажмите кнопку «Создать», затем в блоке «Источник» выберите «По ссылке».
1. В поле «Имя образа» введите имя образа.
1. В поле «URL» укажите ссылку на образ.
1. Нажмите кнопку «Создать».
1. Дождитесь, когда образ перейдёт в состояние «Готов».

{% endtab %}

{% endtabs %}

## Создание кластерного образа из хранилища образов контейнеров

Модуль умеет забирать образ из внешнего хранилища образов контейнеров, но файл диска должен лежать в образе контейнера по пути `/disk`. Ниже показано, как подготовить такой образ контейнера и создать из него кластерный образ.

{% tabs cvi-registry %}

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

1. Создайте ресурс [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage), указав путь к образу контейнера:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: ClusterVirtualImage
   metadata:
     name: ubuntu-2404
   spec:
     dataSource:
       type: ContainerImage
       containerImage:
         image: docker.io/<USERNAME>/ubuntu2404:latest
   EOF
   ```

Модуль работает только с теми хранилищами, где включён TLS. Если хранилище использует собственный центр сертификации, передайте цепочку сертификатов в параметре [`caBundle`](/modules/virtualization/cr.html#clustervirtualimage-v1alpha2-spec-datasource-containerimage-cabundle), а учётные данные для доступа к закрытому хранилищу возьмите из секрета, указанного в параметре `imagePullSecret`.

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Система», далее в раздел «Виртуализация» → «Кластерные образы».
1. Нажмите кнопку «Создать», затем в блоке «Источник» выберите «Из реестра».
1. В поле «Имя образа» введите имя образа.
1. В поле «Образ в реестре контейнеров» укажите ссылку на образ.
1. Нажмите кнопку «Создать».
1. Дождитесь, когда образ перейдёт в состояние «Готов».

{% endtab %}

{% endtabs %}

## Загрузка кластерного образа из командной строки

Если файл образа лежит на вашем компьютере, загрузите его напрямую. Модуль создаёт для этого временную точку приёма данных и ждёт загрузки.

{% tabs cvi-upload %}

{% tab "В командной строке" %}

1. Создайте ресурс [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) с источником `Upload`:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: ClusterVirtualImage
   metadata:
     name: some-image
   spec:
     # Настройки источника образа.
     dataSource:
       type: Upload
   EOF
   ```

   Ресурс перейдёт в фазу `WaitForUserUpload` и будет готов принять файл. Начните загрузку в течение 10 минут, иначе ресурс перейдёт в фазу `Failed` и его придётся создать заново.

1. Получите адреса, по которым принимается файл:

   ```bash
   d8 k get cvi some-image -o jsonpath="{.status.imageUploadURLs}" | jq
   ```

   Пример вывода:

   <!-- markdownlint-disable MD031 -->
   ```console
   {
     "external":"https://virtualization.example.com/upload/<SECRET_URL>",
     "inCluster":"http://10.222.165.239/upload"
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
   d8 k get cvi some-image
   ```

   Пример вывода:

   ```console
   NAME         PHASE   CDROM   PROGRESS   AGE
   some-image   Ready   false   100%       1m
   ```

Загруженный файл тоже можно сверить с контрольной суммой, для этого задайте блок [`checksum`](/modules/virtualization/cr.html#clustervirtualimage-v1alpha2-spec-datasource-upload-checksum) в источнике данных.

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Система», далее в раздел «Виртуализация» → «Кластерные образы».
1. Нажмите кнопку «Создать», затем в блоке «Источник» выберите «Загрузить».
1. В поле «Имя образа» введите имя образа.
1. В блоке «Загрузить файл» перетащите файл в выделенную область или нажмите «выберите на вашем компьютере».
1. Выберите файл в открывшемся файловом менеджере.
1. Нажмите кнопку «Создать».
1. Дождитесь, когда образ перейдёт в состояние «Готов».

{% endtab %}

{% endtabs %}
