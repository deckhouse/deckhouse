---
title: "Кластерные образы"
permalink: ru/virtualization-platform/documentation/admin/platform-management/virtualization/cluster-images.html
lang: ru
---

## Образы

Ресурс [`ClusterVirtualImage`](/modules/virtualization/cr.html#clustervirtualimage) используется для загрузки образов виртуальных машин в внутрикластерное хранилище, что позволяет создавать диски для виртуальных машин. Этот ресурс доступен в любом пространстве имен или проекте кластера.

Процесс создания образа включает следующие шаги:

1. Пользователь создаёт ресурс [`ClusterVirtualImage`](/modules/virtualization/cr.html#clustervirtualimage).
1. После создания образ автоматически загружается из указанного в спецификации источника в хранилище (DVCR).
1. После завершения загрузки ресурс становится доступным для создания дисков.

Существуют различные типы образов:

- **ISO-образ** — установочный образ, используемый для начальной установки операционной системы (ОС). Такие образы выпускаются производителями ОС и используются для установки на физические и виртуальные серверы.
- **Образ диска с предустановленной системой** — содержит уже установленную и настроенную операционную систему, готовую к использованию после создания виртуальной машины. Готовые образы можно получить на ресурсах разработчиков дистрибутива, либо создать самостоятельно.

Примеры ресурсов для получения образов диска виртуальной машины с предустановленной системой:

- Ubuntu:
  - [24.04 LTS (Noble Numbat)](https://cloud-images.ubuntu.com/noble/current/);
  - [22.04 LTS (Jammy Jellyfish)](https://cloud-images.ubuntu.com/jammy/current/);
  - [20.04 LTS (Focal Fossa)](https://cloud-images.ubuntu.com/focal/current/);
  - [Minimal images](https://cloud-images.ubuntu.com/minimal/releases/);
- Debian:
  - [12 bookworm](https://cdimage.debian.org/images/cloud/bookworm/latest/);
  - [11 bullseye](https://cdimage.debian.org/images/cloud/bullseye/latest/);
- AlmaLinux:
  - [9](https://repo.almalinux.org/almalinux/9/cloud/x86_64/images/);
  - [8](https://repo.almalinux.org/almalinux/8/cloud/x86_64/images/);
- RockyLinux:
  - [9.5](https://dl.rockylinux.org/vault/rocky/9.5/images/x86_64/);
  - [8.10](https://download.rockylinux.org/pub/rocky/8.10/images/x86_64/);
- CentOS:
  - [10 Stream](https://cloud.centos.org/centos/10-stream/x86_64/images/);
  - [9 Stream](https://cloud.centos.org/centos/9-stream/x86_64/images/);
  - [8 Stream](https://cloud.centos.org/centos/8-stream/x86_64/);
  - [8](https://cloud.centos.org/centos/8/x86_64/images/);
- Alt Linux:
  - [p10](https://ftp.altlinux.ru/pub/distributions/ALTLinux/p10/images/cloud/x86_64/);
  - [p9](https://ftp.altlinux.ru/pub/distributions/ALTLinux/p9/images/cloud/x86_64/);
- [Astra Linux](https://download.astralinux.ru/ui/native/mg-generic/alse/cloudinit).

Поддерживаются следующие форматы образов с предустановленной системой:

- `qcow2`;
- `raw`;
- `vmdk`;
- `vdi`.

Образы могут быть сжаты одним из следующих алгоритмов сжатия: `gz`, `xz`.

После создания ресурса ClusterVirtualImage тип и размер образа определяются автоматически, и эта информация отражается в статусе ресурса.

Образы могут быть загружены из различных источников, таких как HTTP-серверы, где расположены файлы образов, или контейнерные реестры. Также доступна возможность загрузки образов напрямую из командной строки с использованием утилиты `curl`.

Образы могут быть созданы на основе других образов и дисков виртуальных машин.

С полным описанием параметров конфигурации ресурса `ClusterVirtualImage` можно ознакомиться [в документации](/modules/virtualization/cr.html#clustervirtualimage).

## Увеличение размера DVCR

Чтобы увеличить размер диска для DVCR, необходимо установить больший размер в конфигурации модуля `virtualization`, чем текущий размер.

1. Проверьте текущий размер DVCR:

    ```shell
    d8 k get mc virtualization -o jsonpath='{.spec.settings.dvcr.storage.persistentVolumeClaim}'
    ```

   Пример вывода:

    ```text
    {"size":"58G","storageClass":"linstor-thick-data-r1"}
    ```

1. Задайте размер:

    ```shell
    d8 k patch mc virtualization \
      --type merge -p '{"spec": {"settings": {"dvcr": {"storage": {"persistentVolumeClaim": {"size":"59G"}}}}}}'
    ```

   Пример вывода:

    ```text
   moduleconfig.deckhouse.io/virtualization patched
    ```

1. Проверьте изменение размера:

    ```shell
    d8 k get mc virtualization -o jsonpath='{.spec.settings.dvcr.storage.persistentVolumeClaim}'
    ```

   Пример вывода:

    ```text
    {"size":"59G","storageClass":"linstor-thick-data-r1"}
    ```

1. Проверьте текущее состояние DVCR:

    ```shell
    d8 k get pvc dvcr -n d8-virtualization
    ```

   Пример вывода:

    ```text
    NAME STATUS VOLUME                                    CAPACITY    ACCESS MODES   STORAGECLASS           AGE
    dvcr Bound  pvc-6a6cedb8-1292-4440-b789-5cc9d15bbc6b  57617188Ki  RWO            linstor-thick-data-r1  7d
    ```

### Создание образа с HTTP-сервера

Рассмотрим вариант создания кластерного образа.

1. Чтобы создать ресурс ClusterVirtualImage, выполните следующую команду:

    ```yaml
    d8 k apply -f - <<EOF
    apiVersion: virtualization.deckhouse.io/v1alpha2
    kind: ClusterVirtualImage
    metadata:
      name: ubuntu-22-04
    spec:
      # Источник для создания образа.
      dataSource:
        type: HTTP
        http:
          url: https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
    EOF
    ```

1. Проверьте результат создания ресурса ClusterVirtualImage, выполнив следующую команду:

    ```shell
    d8 k get clustervirtualimage ubuntu-22-04
   ```

    Есть укороченный вариант команды:

    ```shell
    d8 k get cvi ubuntu-22-04
    ```

    В результате будет выведена информация о ресурсе `ClusterVirtualImage`:

    ```console
    NAME           PHASE   CDROM   PROGRESS   AGE
    ubuntu-22-04   Ready   false   100%       23h
    ```

После создания ресурс `ClusterVirtualImage` может находиться в следующих состояниях (фазах):

- `Pending` — ожидание готовности всех зависимых ресурсов, требующихся для создания образа;
- `WaitForUserUpload` — ожидание загрузки образа пользователем (фаза присутствует только для `type=Upload`);
- `Provisioning` — идёт процесс создания образа;
- `Ready` — образ создан и готов для использования;
- `Failed` — произошла ошибка в процессе создания образа;
- `Terminating` — идёт процесс удаления образа. Образ может «зависнуть» в данном состоянии, если он ещё подключен к виртуальной машине.

До тех пор, пока образ не перейдет в фазу `Ready`, содержимое всего блока `.spec` можно изменять. В случае изменения будет инициирован повторный процесс создания образа.
После перехода в фазу `Ready` содержимое блока `.spec` менять нельзя, поскольку на этом этапе образ считается полностью созданным и готовым к использованию. Внесение изменений в этот блок после того, как образ достиг состояния `Ready`, может нарушить его целостность или повлиять на корректность его дальнейшего использования.

Диагностика проблем с ресурсом осуществляется путем анализа информации в блоке `.status.conditions`.

Чтобы отследить процесс создания образа, добавьте ключ `-w` к команде проверки результата создания ресурса:

```shell
d8 k get cvi ubuntu-22-04 -w
```

В результате будет выведена информация о прогрессе создания образа:

```console
NAME           PHASE          CDROM   PROGRESS   AGE
ubuntu-22-04   Provisioning   false              4s
ubuntu-22-04   Provisioning   false   0.0%       4s
ubuntu-22-04   Provisioning   false   28.2%      6s
ubuntu-22-04   Provisioning   false   66.5%      8s
ubuntu-22-04   Provisioning   false   100.0%     10s
ubuntu-22-04   Provisioning   false   100.0%     16s
ubuntu-22-04   Ready          false   100%       18s
```

В описании ресурса `ClusterVirtualImage` можно получить дополнительную информацию о скачанном образе:

```shell
d8 k describe cvi ubuntu-22-04
```

Как создать образ с HTTP-сервера в веб-интерфейсе:

- Перейдите на вкладку «Система», далее в раздел «Виртуализация» → «Кластерные образы».
- Нажмите «Создать образ», далее в выпадающем меню выберите «Загрузить данные по ссылке (HTTP)».
- В поле «Имя образа» введите имя образа.
- В поле «URL» укажите ссылку на образ.
- Нажмите «Создать».
- Дождитесь пока образ перейдет в состояние `Готов`.

### Создание образа из реестра контейнеров

Образ, хранящийся в реестре контейнеров, имеет определённый формат. Рассмотрим на примере:

1. Загрузите образ локально:

    ```shell
    curl -L https://cloud-images.ubuntu.com/minimal/releases/jammy/release/ubuntu-22.04-minimal-cloudimg-amd64.img -o ubuntu2204.img
    ```

1. Создайте `Dockerfile` со следующим содержимым:

    ```shell
    FROM scratch
    COPY ubuntu2204.img /disk/ubuntu2204.img
    ```

1. Соберите образ и загрузите его в реестр контейнеров. В качестве реестра контейнеров в примере ниже использован `docker.io`. Для выполнения вам необходимо иметь учётную запись сервиса и настроенное окружение:

    ```shell
    docker build -t docker.io/<username>/ubuntu2204:latest
    ```

    где `username` — имя пользователя, указанное при регистрации [в docker.io](https://www.docker.com/).

1. Загрузите созданный образ в container registry:

    ```shell
    docker push docker.io/<username>/ubuntu2204:latest
    ```

1. Чтобы использовать этот образ, создайте в качестве примера ресурс:

    ```yaml
    d8 k apply -f - <<EOF
    apiVersion: virtualization.deckhouse.io/v1alpha2
    kind: ClusterVirtualImage
    metadata:
      name: ubuntu-2204
    spec:
      dataSource:
        type: ContainerImage
        containerImage:
          image: docker.io/<username>/ubuntu2204:latest
    EOF
    ```

Как создать образ из реестра контейнеров в веб-интерфейсе:

- Перейдите на вкладку «Система», далее в раздел «Виртуализация» → «Кластерные образы».
- Нажмите «Создать образ», далее в выпадающем списке выберите «Загрузить данные из образа контейнера».
- В поле «Имя образа» введите имя образа.
- В поле «Образ в реестре контейнеров» укажите ссылку на образ.
- Нажмите «Создать».
- Дождитесь, пока образ перейдет в состояние `Готов`.

### Загрузка образа из командной строки

Чтобы загрузить образ из командной строки, предварительно создайте ресурс, как показано на примере `ClusterVirtualImage`:

```yaml
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

После создания ресурс перейдёт в фазу `WaitForUserUpload`, что говорит о готовности к загрузке образа.

Существует два варианта загрузки — с узла кластера или с произвольного узла за пределами кластера:

```shell
d8 k get cvi some-image -o jsonpath="{.status.imageUploadURLs}"  | jq
```

Пример вывода:

```text
{
  "external":"https://virtualization.example.com/upload/g2OuLgRhdAWqlJsCMyNvcdt4o5ERIwmm",
  "inCluster":"http://10.222.165.239/upload"
}
```

Здесь:

- `inCluster` — URL-адрес, который используется, если необходимо выполнить загрузку образа с одного из узлов кластера;
- `external` — URL-адрес, который используется во всех остальных случаях.

В качестве примера загрузите образ Cirros:

```shell
curl -L http://download.cirros-cloud.net/0.5.1/cirros-0.5.1-x86_64-disk.img -o cirros.img
```

Затем выполните загрузку образа с помощью следующей команды:

```shell
curl https://virtualization.example.com/upload/g2OuLgRhdAWqlJsCMyNvcdt4o5ERIwmm --progress-bar -T cirros.img | cat
```

После завершения загрузки образ должен быть создан и переведён в фазу `Ready`. Для проверки состояния образа выполните команду:

```shell
d8 k get cvi some-image
```

В результате будет выведена информация о состоянии образа:

```console
NAME         PHASE   CDROM   PROGRESS   AGE
some-image   Ready   false   100%       1m
```

Как выполнить операцию в веб-интерфейсе:

- Перейдите на вкладку «Система», далее в раздел «Виртуализация» → «Кластерные образы».
- Нажмите «Создать образ», далее в выпадающем меню выберите «Загрузить с компьютера».
- В поле «Имя образа» введите имя образа.
- В поле «Загрузить файл» нажмите ссылку «Выберите файл на вашем компьютере».
- Выберите файл в открывшемся файловом менеджере.
- Нажмите кнопку «Создать».
- Дождитесь, пока образ перейдет в состояние `Готов`.
