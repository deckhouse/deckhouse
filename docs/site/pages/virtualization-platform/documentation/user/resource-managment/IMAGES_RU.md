---
title: "Образы"
permalink: ru/virtualization-platform/documentation/user/resource-management/images.html
lang: ru
---

Ресурс [`VirtualImage`](../../../reference/cr.html#virtualimage) предназначен для загрузки образов виртуальных машин и их последующего использования для создания дисков виртуальных машин. Этот ресурс доступен только в пространстве имен или проекте, в котором он был создан.

Процесс создания образа включает следующие шаги:

- Пользователь создаёт ресурс [`VirtualImage`](../../../reference/cr.html#virtualimage).
- После создания образ автоматически загружается из указанного в спецификации источника в хранилище (DVCR).
- После завершения загрузки ресурс становится доступным для создания дисков.

Существуют различные типы образов:

- ISO-образ — установочный образ, используемый для начальной установки операционной системы. Такие образы выпускаются производителями ОС и используются для установки на физические и виртуальные серверы.
- Образ диска с предустановленной системой — содержит уже установленную и настроенную операционную систему, готовую к использованию после создания виртуальной машины. Эти образы предлагаются несколькими производителями и могут быть представлены в таких форматах, как qcow2, raw, vmdk и другие.

Примеры ресурсов для получения образов виртуальной машины:

- [Ubuntu](https://cloud-images.ubuntu.com);
- [Alt Linux](https://ftp.altlinux.ru/pub/distributions/ALTLinux/platform/images/cloud/x86_64);
- [Astra Linux](https://download.astralinux.ru/ui/native/mg-generic/alse/cloudinit).

После создания ресурса, тип и размер образа определяются автоматически, и эта информация отражается в статусе ресурса.

Образы могут быть загружены из различных источников, таких как HTTP-серверы, где расположены файлы образов, или контейнерные реестры. Также доступна возможность загрузки образов напрямую из командной строки с использованием утилиты curl.

Образы могут быть созданы из других образов и дисков виртуальных машин.

Для проектных образов поддерживается два варианта хранения:

- `ContainerRegistry` - тип по умолчанию, при котором образ хранится в `DVCR`.
- `Kubernetes` - тип, при котором в качестве хранилища для образа используется `PVC`. Этот вариант предпочтителен, если используется хранилище с поддержкой быстрого клонирования `PVC`. В этом случае диски из образов будут создаваться быстрее.

### Создание образа с HTTP-сервера

Рассмотрим вариант создания образа с вариантом хранения в DVCR. Выполните следующую команду для создания [VirtualImage](../../../reference/cr.html#virtualimage):

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualImage
metadata:
  name: ubuntu-22.04
spec:
  # Сохраним образ в DVCR
  storage: ContainerRegistry
  # Источник для создания образа.
  dataSource:
    type: HTTP
    http:
      url: "https://cloud-images.ubuntu.com/minimal/releases/jammy/release/ubuntu-22.04-minimal-cloudimg-amd64.img"
EOF
```

Проверить результат создания [VirtualImage](../../../reference/cr.html#virtualimage):

```bash
d8 k get virtualimage ubuntu-22.04
# или более короткий вариант
d8 k get vi ubuntu-22.04

# NAME           PHASE   CDROM   PROGRESS   AGE
# ubuntu-22.04   Ready   false   100%       23h
```

После создания ресурс [VirtualImage](../../../reference/cr.html#virtualimage) может находиться в следующих состояниях:

- `Pending` - ожидание готовности всех зависимых ресурсов, требующихся для создания образа.
- `WaitForUserUpload` - ожидание загрузки образа пользователем (состояние присутствует только для `type=Upload`).
- `Provisioning` - идет процесс создания образа.
- `Ready` - образ создан и готов для использования.
- `Failed` - произошла ошибка в процессе создания образа.
- `Terminating` - идет процесс удаления образа; процесс может «зависнуть» в этом состоянии, если образ еще подключен к виртуальной машине.

До тех пор, пока образ не перешёл в фазу `Ready`, содержимое всего блока `.spec` допускается изменять. При изменении процесс создания диска запустится заново. После перехода в фазу `Ready` содержимое блока `.spec` менять нельзя!

Отследить процесс создания образа можно путем добавления ключа `-w` к предыдущей команде:

```bash
d8 k get vi ubuntu-22.04 -w

# NAME           PHASE          CDROM   PROGRESS   AGE
# ubuntu-22.04   Provisioning   false              4s
# ubuntu-22.04   Provisioning   false   0.0%       4s
# ubuntu-22.04   Provisioning   false   28.2%      6s
# ubuntu-22.04   Provisioning   false   66.5%      8s
# ubuntu-22.04   Provisioning   false   100.0%     10s
# ubuntu-22.04   Provisioning   false   100.0%     16s
# ubuntu-22.04   Ready          false   100%       18s
```

В описании ресурса [VirtualImage](../../../reference/cr.html#virtualimage) можно получить дополнительную информацию о скачанном образе:

```bash
d8 k describe vi ubuntu-22.04
```

Теперь рассмотрим пример создания образа с хранением его в PVC:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualImage
metadata:
  name: ubuntu-22.04-pvc
spec:
  # Настройки хранения проектного образа.
  storage: Kubernetes
  # Источник для создания образа.
  dataSource:
    type: HTTP
    http:
      url: "https://cloud-images.ubuntu.com/minimal/releases/jammy/release/ubuntu-22.04-minimal-cloudimg-amd64.img"
EOF
```

Проверить результат создания [VirtualImage](../../../reference/cr.html#virtualimage):

```bash
d8 k get vi ubuntu-22.04-pvc

# NAME              PHASE   CDROM   PROGRESS   AGE
# ubuntu-22.04-pvc  Ready   false   100%       23h
```

### Создание образа из Container Registry

Образ, хранящийся в `Container Registry`, имеет определенный формат. Рассмотрим на примере:

Для начала загрузите образ локально:

```bash
curl -L https://cloud-images.ubuntu.com/minimal/releases/jammy/release/ubuntu-22.04-minimal-cloudimg-amd64.img -o ubuntu2204.img
```

Затем создайте `Dockerfile` со следующим содержимым:

```Dockerfile
FROM scratch
COPY ubuntu2204.img /disk/ubuntu2204.img
```

Далее нужно собрать образ и загрузить его в `Container registry`. В качестве `Container registry` в примере ниже использован docker.io. Для выполнения вам необходимо иметь учетную запись сервиса и настроенное окружение.

```bash
docker build -t docker.io/<username>/ubuntu2204:latest
```

где `username` — имя пользователя, указанное при регистрации в docker.io.

Загрузите созданный образ в `Container registry`:

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

### Загрузка образа из командной строки

Чтобы загрузить образ из командной строки, предварительно создайте следующий ресурс, как представлено ниже на примере [VirtualImage](../../../reference/cr.html#virtualimage):

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

# {
#   "external":"https://virtualization.example.com/upload/g2OuLgRhdAWqlJsCMyNvcdt4o5ERIwmm",
#   "inCluster":"http://10.222.165.239/upload"
# }
```

В качестве примера рассмотрим загрузку образ Cirros:

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
# NAME         PHASE   CDROM   PROGRESS   AGE
# some-image   Ready   false   100%       1m
```

### Создание образа из диска

Существует возможность создать образ из [диска](./disk.html). Для этого необходимо выполнить одно из следующих условий:

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
