---
title: "Образы"
permalink: ru/virtualization-platform/documentation/user/resource-management/images.html
lang: ru
---

Ресурс [VirtualImage](../../../reference/cr/virtualimage.html) предназначен для загрузки образов виртуальных машин и их последующего использования для создания дисков виртуальных машин. Этот ресурс доступен только в пространстве имен или проекте, в котором он был создан.

При подключении к виртуальной машине доступ к образу предоставляется в режиме «только чтение».

Процесс создания образа включает следующие шаги:

1. Пользователь создаёт ресурс [VirtualImage](../../../reference/cr/virtualimage.html).
1. После создания образ автоматически загружается из указанного в спецификации источника в хранилище (DVCR).
1. После завершения загрузки ресурс становится доступным для создания дисков.

Существуют различные типы образов:

- **ISO-образ** — установочный образ, используемый для начальной установки операционной системы. Такие образы выпускаются производителями ОС и используются для установки на физические и виртуальные серверы.
- **Образ диска с предустановленной системой** — содержит уже установленную и настроенную операционную систему, готовую к использованию после создания виртуальной машины. Готовые образы можно получить на ресурсах разработчиков дистрибутива, либо создать самостоятельно.

Примеры ресурсов для получения образов виртуальной машины:

| Дистрибутив                                                                       | Пользователь по умолчанию |
|-----------------------------------------------------------------------------------|---------------------------|
| [AlmaLinux](https://almalinux.org/get-almalinux/#Cloud_Images)                    | `almalinux`               |
| [AlpineLinux](https://alpinelinux.org/cloud/)                                     | `alpine`                  |
| [AltLinux](https://ftp.altlinux.ru/pub/distributions/ALTLinux/)                   | `altlinux`                     |
| [AstraLinux](https://download.astralinux.ru/ui/native/mg-generic/alse/cloudinit/) | `astra`                     |
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

После создания ресурса, тип и размер образа определяются автоматически, и эта информация отражается в статусе ресурса.

Образы могут быть загружены из различных источников, таких как HTTP-серверы, где расположены файлы образов, или контейнерные реестры. Также доступна возможность загрузки образов напрямую из командной строки с использованием утилиты curl.

Образы могут быть созданы из других образов и дисков виртуальных машин.

Для проектных образов поддерживается два варианта хранения:

- `ContainerRegistry` — тип по умолчанию, при котором образ хранится в `DVCR`.
- `PersistentVolumeClaim` — тип, при котором в качестве хранилища для образа используется `PVC`. Этот вариант предпочтителен, если используется хранилище с поддержкой быстрого клонирования `PVC`. В этом случае диски из образов будут создаваться быстрее.

## Создание образа с HTTP-сервера

Рассмотрим вариант создания образа с вариантом хранения в DVCR. Выполните следующую команду для создания [VirtualImage](../../../reference/cr/virtualimage.html):

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualImage
metadata:
  name: ubuntu-22-04
spec:
  # Сохраним образ в DVCR
  storage: ContainerRegistry
  # Источник для создания образа.
  dataSource:
    type: HTTP
    http:
      url: https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
EOF
```

Проверьте результат создания [VirtualImage](../../../reference/cr/virtualimage.html):

```bash
d8 k get virtualimage ubuntu-22-04

# Укороченный вариант команды
d8 k get vi ubuntu-22-04
```

Пример вывода:

```console
NAME           PHASE   CDROM   PROGRESS   AGE
ubuntu-22-04   Ready   false   100%       23h
```

После создания ресурс [VirtualImage](../../../reference/cr/virtualimage.html) может находиться в следующих состояниях:

- `Pending` — ожидание готовности всех зависимых ресурсов, требующихся для создания образа.
- `WaitForUserUpload` — ожидание загрузки образа пользователем (состояние присутствует только для `type=Upload`).
- `Provisioning` — идет процесс создания образа.
- `Ready` — образ создан и готов для использования.
- `Failed` — произошла ошибка в процессе создания образа.
- `Terminating` — идет процесс удаления образа; процесс может «зависнуть» в этом состоянии, если образ еще подключен к виртуальной машине.

До тех пор, пока образ не перешёл в фазу `Ready`, содержимое всего блока `.spec` допускается изменять. При изменении процесс создания диска запустится заново. После перехода в фазу `Ready` содержимое блока `.spec` менять нельзя!

Диагностика проблем с ресурсом осуществляется путем анализа информации в блоке `.status.conditions`.

Отследить процесс создания образа можно путем добавления ключа `-w` к предыдущей команде:

```bash
d8 k get vi ubuntu-22-04 -w
```

Пример вывода:

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

В описании ресурса [VirtualImage](../../../reference/cr/virtualimage.html) можно получить дополнительную информацию о скачанном образе:

```bash
d8 k describe vi ubuntu-22-04
```

Как создать образ с HTTP-сервера в веб-интерфейсе:

- Перейдите на вкладку «Проекты» и выберите нужный проект.
- Перейдите в раздел «Виртуализация» -> «Образы дисков».
- Нажмите «Создать образ».
- Из списка выберите «Загрузить данные по ссылке (HTTP)».
- В открывшейся форме в поле «Имя образа» введите имя образа.
- В поле «Хранилище» выберите `ContainerRegistry`.
- В поле «URL» укажите ссылку на образ.
- Нажмите кнопку «Создать».
- Статус образа отображается слева вверху, под именем образа.

Теперь рассмотрим пример создания образа с хранением его в PVC:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualImage
metadata:
  name: ubuntu-22-04-pvc
spec:
  # Настройки хранения проектного образа.
  storage: PersistentVolumeClaim
  persistentVolumeClaim:
    # Подставьте ваше название StorageClass.
    storageClassName: i-sds-replicated-thin-r2
  # Источник для создания образа.
  dataSource:
    type: HTTP
    http:
      url: https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
EOF
```

Проверить результат создания [VirtualImage](../../../reference/cr/virtualimage.html):

```bash
d8 k get vi ubuntu-22-04-pvc
```

Пример вывода:

```console
NAME              PHASE   CDROM   PROGRESS   AGE
ubuntu-22-04-pvc  Ready   false   100%       23h
```

Если параметр `.spec.persistentVolumeClaim.storageClassName` не указан, то будет использован `StorageClass` по умолчанию на уровне кластера, либо для образов, если он указан в [настройках модуля](../../admin/install/steps/virtualization.html#описание-параметров).

Как в веб-интерфейсе создать образ с его хранением в PVC:

- Перейдите на вкладку «Проекты» и выберите нужный проект.
- Перейдите в раздел «Виртуализация» -> «Образы дисков».
- Нажмите «Создать образ».
- Из списка выберите «Загрузить данные по ссылке (HTTP)».
- В открывшейся форме в поле «Имя образа» введите имя образа.
- В поле «Хранилище» выберите `PersistentVolumeClaim`.
- В поле «Класс хранилища» можно выбрать StorageClass или оставить выбранный по умолчанию.
- В поле «URL» укажите ссылку на образ.
- Нажмите кнопку «Создать».
- Статус образа отображается слева вверху, под именем образа.

## Создание образа из container registry

Образ, хранящийся в container registry, имеет определенный формат. Рассмотрим на примере:

Для начала загрузите образ локально:

```bash
curl -L https://cloud-images.ubuntu.com/minimal/releases/jammy/release/ubuntu-22.04-minimal-cloudimg-amd64.img -o ubuntu2204.img
```

Затем создайте Dockerfile со следующим содержимым:

```Dockerfile
FROM scratch
COPY ubuntu2204.img /disk/ubuntu2204.img
```

Далее нужно собрать образ и загрузить его в container registry. В качестве container registry в примере ниже использован docker.io. Для выполнения вам необходимо иметь учетную запись сервиса и настроенное окружение.

```bash
docker build -t docker.io/<username>/ubuntu2204:latest
```

где `username` — имя пользователя, указанное при регистрации в docker.io.

Загрузите созданный образ в container registry:

```bash
docker push docker.io/<username>/ubuntu2204:latest
```

Чтобы использовать этот образ, создайте в качестве примера ресурс:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualImage
metadata:
  name: ubuntu-2204
spec:
  storage: ContainerRegistry
  dataSource:
    type: ContainerImage
    containerImage:
      image: docker.io/<username>/ubuntu2204:latest
EOF
```

Как создать образ из Container Registry в веб-интерфейсе:

- Перейдите на вкладку «Проекты» и выберите нужный проект.
- Перейдите в раздел «Виртуализация» -> «Образы дисков».
- Нажмите «Создать образ».
- Из списка выберите «Загрузить данные из образа контейнера».
- В открывшейся форме в поле «Имя образа» введите имя образа.
- В поле «Хранилище» выберите `ContainerRegistry`.
- В поле «Образ в реестре контейнеров» укажите `docker.io/<username>/ubuntu2204:latest`.
- Нажмите кнопку «Создать».
- Статус образа отображается слева вверху, под именем образа.

## Загрузка образа из командной строки

Чтобы загрузить образ из командной строки, предварительно создайте следующий ресурс, как представлено ниже на примере [VirtualImage](../../../reference/cr/virtualimage.html):

```yaml
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

После создания, ресурс перейдет в фазу `WaitForUserUpload`, это означает, что он готов для загрузки образа.

Доступно два варианта загрузки: с узла кластера и с произвольного узла за пределами кластера:

```bash
d8 k get vi some-image -o jsonpath="{.status.imageUploadURLs}"  | jq
```

Пример вывода:

```json
{
  "external": "https://virtualization.example.com/upload/g2OuLgRhdAWqlJsCMyNvcdt4o5ERIwmm",
  "inCluster": "http://10.222.165.239/upload"
}
```

В качестве примера загрузите образ Cirros:

```bash
curl -L http://download.cirros-cloud.net/0.5.1/cirros-0.5.1-x86_64-disk.img -o cirros.img
```

Выполните загрузку образа с использованием команды:

```bash
curl https://virtualization.example.com/upload/g2OuLgRhdAWqlJsCMyNvcdt4o5ERIwmm --progress-bar -T cirros.img | cat
```

После завершения загрузки образ должен быть создан и перейти в фазу `Ready`.

```bash
d8 k get vi some-image
```

Пример вывода:

```console
NAME         PHASE   CDROM   PROGRESS   AGE
some-image   Ready   false   100%       1m
```

Как загрузить образ из командной строки в веб-интерфейсе:

- Перейдите на вкладку «Проекты» и выберите нужный проект.
- Перейдите в раздел «Виртуализация» -> «Образы дисков».
- Нажмите «Создать образ», далее в выпадающем меню выберите «Загрузить с компьютера».
- В поле «Имя образа» введите имя образа.
- В поле «Загрузить файл» нажмите ссылку «Выберите файл на вашем компьютере».
- Выберите файл в открывшемся файловом менеджере.
- Нажмите кнопку «Создать».
- Дождитесь, пока образ перейдет в состояние `Готов`.

## Создание образа из диска

Существует возможность создать образ из [диска](./disks.html). Для этого необходимо выполнить одно из следующих условий:

- Диск не должен быть подключен ни к одной из виртуальных машин.
- Виртуальная машина, к которой подключен диск, находится в выключенном состоянии.

Пример создания образа из диска:

```yaml
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

Как в веб-интерфейсе создать образ из диска:

- Перейдите на вкладку «Проекты» и выберите нужный проект.
- Перейдите в раздел «Виртуализация» -> «Образы дисков».
- Нажмите «Создать образ».
- Из списка выберите «Записать данные из диска».
- В открывшейся форме в поле «Имя образа» введите `linux-vm-root`.
- В поле «Хранилище» выберите `ContainerRegistry`.
- В поле «Диск» выберите из выпадающего списка необходимый диск.
- Нажмите кнопку «Создать».
- Статус образа отображается слева вверху, под его именем.

## Настройки классов хранения для образов

Настройки классов хранения для образов определяется в параметре `.spec.settings.virtualImages` настроек модуля.
Пример:

```yaml
spec:
  ...
  settings:
    virtualImages:
       allowedStorageClassNames:
       - sc-1
       - sc-2
       defaultStorageClassName: sc-1
```

`allowedStorageClassNames` — (опционально) это список допустимых `StorageClass` для создания `VirtualImage`, которые можно явно указать в спецификации ресурса.
`defaultStorageClassName` — (опционально) это `StorageClass`, используемый по умолчанию при создании `VirtualImage`, если параметр `.spec.persistentVolumeClaim.storageClassName` не задан.
