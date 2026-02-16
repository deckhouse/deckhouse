---
title: "Кластерные образы"
permalink: ru/virtualization-platform/documentation/admin/platform-management/virtualization/cluster-images.html
lang: ru
---

## Образы

Ресурс [VirtualImage](/modules/virtualization/cr.html#virtualimage) предназначен для загрузки образов виртуальных машин и их последующего использования для создания дисков виртуальных машин.

{% alert level="warning" %}
Обратите внимание, что [VirtualImage](/modules/virtualization/cr.html#virtualimage) — это проектный ресурс, то есть он доступен только в том проекте или пространстве имен, в котором был создан. Для использования образов на уровне всего кластера предназначен отдельный ресурс — [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage).
{% endalert %}

При подключении к виртуальной машине доступ к образу предоставляется в режиме «только чтение».

Процесс создания образа включает следующие шаги:

- Пользователь создаёт ресурс [VirtualImage](/modules/virtualization/cr.html#virtualimage).
- После создания образ автоматически загружается из указанного в спецификации источника в хранилище DVCR или PVC в зависимости от типа.
- После завершения загрузки, ресурс становится доступным для создания дисков.

Существуют различные типы образов:

- **ISO-образ** — установочный образ, используемый для начальной установки операционной системы. Такие образы выпускаются производителями ОС и используются для установки на физические и виртуальные серверы.
- **Образ диска с предустановленной системой** — содержит уже установленную и настроенную операционную систему, готовую к использованию после создания виртуальной машины. Готовые образы можно получить на ресурсах разработчиков дистрибутива, либо создать самостоятельно.

Примеры ресурсов для получения образов виртуальной машины:

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

Поддерживаются следующие форматы образов с предустановленной системой:

- qcow2
- raw
- vmdk
- vdi

Также файлы образов могут быть сжаты одним из следующих алгоритмов сжатия: gz, xz.

После создания ресурса, тип и размер образа определяются автоматически и эта информация отражается в статусе ресурса.

В статусе образа отображаются два размера:

- STOREDSIZE (размер в хранилище) — объём, который образ фактически занимает в хранилище (DVCR или PVC). Для образов, загруженных в сжатом виде (например, `.gz`, `.xz`), это значение меньше распакованного размера.
- UNPACKEDSIZE (распакованный размер) — размер образа после распаковки. Он используется при создании диска из образа и задаёт минимальный размер диска, который можно создать.

{% alert level="info" %}
При создании диска из образа укажите размер диска не меньше значения `UNPACKEDSIZE`.  
Если размер не задан, диск будет создан с размером, соответствующим распакованному размеру образа.
{% endalert %}

Образы могут быть загружены из различных источников, таких как HTTP-серверы, где расположены файлы образов, или контейнерные реестры. Также доступна возможность загрузки образов напрямую из командной строки с использованием утилиты curl.

Образы могут быть созданы из других образов и дисков виртуальных машин.

Проектный образ поддерживает два варианта хранения:

- `ContainerRegistry` - тип по умолчанию, при котором образ хранится в `DVCR`.
- `PersistentVolumeClaim` - тип, при котором в качестве хранилища для образа используется `PVC`. Этот вариант предпочтителен, если используется хранилище с поддержкой быстрого клонирования `PVC`, что позволяет быстрее создавать диски из образов.

{% alert level="warning" %}
Использование образа с параметром `storage: PersistentVolumeClaim` поддерживается только для создания дисков в том же классе хранения (StorageClass).
{% endalert %}

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

## Создание golden image для Linux

Golden image — это предварительно настроенный образ виртуальной машины, который можно использовать для быстрого создания новых ВМ с уже установленным программным обеспечением и настройками.

1. Создайте виртуальную машину, установите на неё необходимое программное обеспечение и выполните все требуемые настройки.

1. Установите и настройте qemu-guest-agent (рекомендуется):

   - Для RHEL/CentOS:

   ```bash
   yum install -y qemu-guest-agent
   ```

   - Для Debian/Ubuntu:

   ```bash
   apt-get update
   apt-get install -y qemu-guest-agent
   ```

1. Включите и запустите сервис:

   ```bash
   systemctl enable qemu-guest-agent
   systemctl start qemu-guest-agent
   ```

1. Установите политику запуска ВМ [runPolicy: AlwaysOnUnlessStoppedManually](/modules/virtualization/stable/cr.html#virtualmachine-v1alpha2-spec-runpolicy) — это потребуется, чтобы ВМ можно было выключить.

1. Подготовьте образ. Очистите неиспользуемые блоки файловой системы:

   ```bash
   fstrim -v /
   fstrim -v /boot
   ```

1. Очистите сетевые настройки:

   - Для RHEL:

   ```bash
   nmcli con delete $(nmcli -t -f NAME,DEVICE con show | grep -v ^lo: | cut -d: -f1)
   rm -f /etc/sysconfig/network-scripts/ifcfg-eth*
   ```

   - Для Debian/Ubuntu:

   ```bash
   rm -f /etc/network/interfaces.d/*
   ```

1. Очистите системные идентификаторы:

   ```bash
   echo -n > /etc/machine-id
   rm -f /var/lib/dbus/machine-id
   ln -s /etc/machine-id /var/lib/dbus/machine-id
   ```

1. Удалите SSH host keys:

   ```bash
   rm -f /etc/ssh/ssh_host_*
   ```

1. Очистите systemd journal:

   ```bash
   journalctl --vacuum-size=100M --vacuum-time=7d
   ```

1. Очистите кеш пакетных менеджеров:

   - Для RHEL:

   ```bash
   yum clean all
   ```

   - Для Debian/Ubuntu:

   ```bash
   apt-get clean
   ```

1. Очистите временные файлы:

   ```bash
   rm -rf /tmp/*
   rm -rf /var/tmp/*
   ```

1. Очистите логи:

   ```bash
   find /var/log -name "*.log" -type f -exec truncate -s 0 {} \;
   ```

1. Очистите историю команд:

   ```bash
   history -c
   ```

   Для RHEL: выполните сброс и восстановление контекстов SELinux (выберите один из вариантов):

   - Вариант 1: Проверка и восстановление контекстов сразу:

     ```bash
     restorecon -R /
     ```

   - Вариант 2: Запланировать relabel при следующей загрузке:

     ```bash
     touch /.autorelabel
     ```

1. Убедитесь, что в `/etc/fstab` используются UUID или LABEL вместо имён устройств (например, `/dev/sdX`). Для проверки выполните:

   ```bash
   blkid
   cat /etc/fstab
   ```

1. Очистите состояние cloud-init, логи и seed (рекомендуемый способ):

   ```bash
   cloud-init clean --logs --seed
   ```

1. Выполните финальную синхронизацию и очистку буферов:

   ```bash
   sync
   echo 3 > /proc/sys/vm/drop_caches
   ```

1. Выключите виртуальную машину:

   ```bash
   poweroff
   ```

1. Создайте ресурс `VirtualImage` из диска подготовленной ВМ:

   ```bash
   d8 k apply -f -<<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualImage
   metadata:
     name: <image-name>
     namespace: <namespace>
   spec:
     dataSource:
       type: ObjectRef
       objectRef:
         kind: VirtualDisk
         name: <source-disk-name>
   EOF
   ```

   Альтернативно, создайте `ClusterVirtualImage`, чтобы образ был доступен на уровне кластера для всех проектов:

    ```bash
    d8 k apply -f -<<EOF
    apiVersion: virtualization.deckhouse.io/v1alpha2
    kind: ClusterVirtualImage
    metadata:
      name: <image-name>
    spec:
      dataSource:
        type: ObjectRef
        objectRef:
          kind: VirtualDisk
          name: <source-disk-name>
          namespace: <namespace>
    EOF
    ```

1. Создайте диск ВМ из созданного образа:

   ```bash
   d8 k apply -f -<<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualDisk
   metadata:
     name: <vm-disk-name>
     namespace: <namespace>
   spec:
     dataSource:
       type: ObjectRef
       objectRef:
         kind: VirtualImage
         name: <image-name>
   EOF
   ```

После выполнения этих шагов у вас будет golden image, который можно использовать для быстрого создания новых виртуальных машин с предустановленным программным обеспечением и настройками.

### Создание образа с HTTP-сервера

Рассмотрим вариант создания кластерного образа.

1. Чтобы создать ресурс ClusterVirtualImage, выполните следующую команду:

    ```yaml
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

1. Проверьте результат создания ресурса ClusterVirtualImage, выполнив следующую команду:

    ```shell
    d8 k get clustervirtualimage ubuntu-24-04
   ```

    Есть укороченный вариант команды:

    ```shell
    d8 k get cvi ubuntu-24-04
    ```

    В результате будет выведена информация о ресурсе `ClusterVirtualImage`:

    ```console
    NAME           PHASE   CDROM   PROGRESS   AGE
    ubuntu-24-04   Ready   false   100%       23h
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
d8 k get cvi ubuntu-24-04 -w
```

В результате будет выведена информация о прогрессе создания образа:

```console
NAME           PHASE          CDROM   PROGRESS   AGE
ubuntu-24-04   Provisioning   false              4s
ubuntu-24-04   Provisioning   false   0.0%       4s
ubuntu-24-04   Provisioning   false   28.2%      6s
ubuntu-24-04   Provisioning   false   66.5%      8s
ubuntu-24-04   Provisioning   false   100.0%     10s
ubuntu-24-04   Provisioning   false   100.0%     16s
ubuntu-24-04   Ready          false   100%       18s
```

В описании ресурса `ClusterVirtualImage` можно получить дополнительную информацию о скачанном образе:

```shell
d8 k describe cvi ubuntu-24-04
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

### Очистка хранилища образов

{% alert level="info" %}
Доступно с [версии 1.2.0](/products/virtualization-platform/documentation/release-notes.html#v120) и выше.
{% endalert %}

Со временем создание и удаление ресурсов ClusterVirtualImage, VirtualImage, VirtualDisk приводит к накоплению
неактуальных образов во внутрикластерном хранилище. Для поддержания хранилища в актуальном состоянии предусмотрена сборка мусора по расписанию.
По умолчанию эта функция отключена. Для включения очистки нужно задать расписание в настройках модуля в ресурсе ModuleConfig/virtualization:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  # ...
  settings:
    dvcr:
      gc:
        schedule: "0 20 * * *"
  # ...
```

На время работы сборки мусора хранилище переводится в режим «только чтение». Все создаваемые в это время ресурсы будут ожидать окончания очистки.

Для проверки наличия неактуальных образов в хранилище можно выполнить такую команду:

```bash
d8 k -n d8-virtualization exec deploy/dvcr -- dvcr-cleaner gc check
```

На экран будут выведены сведения о состоянии хранилища и список неактуальных образов, которые могут быть удалены.

```console
Found 2 cvi, 5 vi, 1 vd manifests in registry
Found 1 cvi, 5 vi, 11 vd resources in cluster
  Total     Used    Avail     Use%
36.3GiB  13.1GiB  22.4GiB      39%
Images eligible for cleanup:
KIND                   NAMESPACE            NAME
ClusterVirtualImage                         debian-12
VirtualDisk            default              debian-10-root
VirtualImage           default              ubuntu-2204
```
