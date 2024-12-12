---
title: "Кластерные образы"
permalink: ru/virtualization-platform/documentation/admin/platform-management/virtualization/cluster-images.html
lang: ru
---

## Образы

Ресурс [`ClusterVirtualImage`](../../../../reference/cr/clustervirtualimage.html) используется для загрузки образов виртуальных машин в внутрикластерное хранилище, что позволяет создавать диски для виртуальных машин. Этот ресурс доступен в любом пространстве имен или проекте кластера.

Процесс создания образа включает следующие шаги:

1. Пользователь создаёт ресурс [`ClusterVirtualImage`](../../../../reference/cr/clustervirtualimage.html).
1. После создания, образ автоматически загружается из указанного в спецификации источника в хранилище (DVCR).
1. По завершении загрузки ресурс становится доступным для создания дисков.

Существуют различные типы образов:

- ISO-образ — установочный образ, используемый для начальной установки операционной системы. Такие образы выпускаются производителями ОС и используются для установки на физические и виртуальные серверы.
- Образ диска с предустановленной системой — содержит уже установленную и настроенную операционную систему, готовую к использованию после создания виртуальной машины. Эти образы предлагаются несколькими производителями и могут быть представлены в таких форматах, как qcow2, raw, vmdk и другие.

Примеры ресурсов для получения образов диска виртуальной машины с предустановленной системой:

- [Ubuntu](https://cloud-images.ubuntu.com);
- [Alt Linux](https://ftp.altlinux.ru/pub/distributions/ALTLinux/platform/images/cloud/x86_64);
- [Astra Linux](https://download.astralinux.ru/ui/native/mg-generic/alse/cloudinit).

После создания ресурса тип и размер образа определяются автоматически, и эта информация отражается в статусе ресурса.

Образы могут быть загружены из различных источников, таких как HTTP-серверы, где расположены файлы образов, или контейнерные реестры. Также доступна возможность загрузки образов напрямую из командной строки с использованием утилиты `curl`.

Образы могут быть созданы на основе других образов и дисков виртуальных машин.

С полным описанием параметров конфигурации ресурса `ClusterVirtualImage` можно ознакомиться [в документации](../../../../reference/cr/clustervirtualimage.html).

### Создание образа с HTTP-сервера

Рассмотрим вариант создания кластерного образа.

Выполните следующую команду для создания `ClusterVirtualImage`:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: ClusterVirtualImage
metadata:
  name: ubuntu-22.04
spec:
  # Источник для создания образа.
  dataSource:
    type: HTTP
    http:
      url: "https://cloud-images.ubuntu.com/minimal/releases/jammy/release/ubuntu-22.04-minimal-cloudimg-amd64.img"
EOF
```

Проверьте результат создания `ClusterVirtualImage` с помощью следующей команды:

```shell
d8 k get clustervirtualimage ubuntu-22.04

# Укороченный вариант команды
d8 k get cvi ubuntu-22.04
```

В результате будет выведена информация о ресурсе `ClusterVirtualImage`:

```console
NAME           PHASE   CDROM   PROGRESS   AGE
ubuntu-22.04   Ready   false   100%       23h
```

После создания ресурс `ClusterVirtualImage` может находиться в следующих состояниях (фазах):

- `Pending` — ожидание готовности всех зависимых ресурсов, требующихся для создания образа.
- `WaitForUserUpload` — ожидание загрузки образа пользователем (фаза присутствует только для `type=Upload`).
- `Provisioning` — идет процесс создания образа.
- `Ready` — образ создан и готов для использования.
- `Failed` — произошла ошибка в процессе создания образа.
- `Terminating` — идет процесс удаления образа. Образ может «зависнуть» в данном состоянии, если он еще подключен к виртуальной машине.

До тех пор, пока образ не перейдет в фазу Ready, содержимое всего блока `.spec` можно изменять. В случае изменения будет инициирован повторный процесс создания образа.
После перехода в фазу Ready содержимое блока `.spec` менять нельзя, поскольку на этом этапе образ считается полностью созданным и готовым к использованию. Внесение изменений в этот блок после того, как образ достиг состояния Ready, может нарушить его целостность или повлиять на корректность его дальнейшего использования.

Отследить процесс создания образа можно путем добавления ключа `-w` к предыдущей команде:

```shell
d8 k get cvi ubuntu-22.04 -w
```

В результате будет выведена информация о прогрессе создания образа:

```console
NAME           PHASE          CDROM   PROGRESS   AGE
ubuntu-22.04   Provisioning   false              4s
ubuntu-22.04   Provisioning   false   0.0%       4s
ubuntu-22.04   Provisioning   false   28.2%      6s
ubuntu-22.04   Provisioning   false   66.5%      8s
ubuntu-22.04   Provisioning   false   100.0%     10s
ubuntu-22.04   Provisioning   false   100.0%     16s
ubuntu-22.04   Ready          false   100%       18s
```

В описании ресурса `ClusterVirtualImage` можно получить дополнительную информацию о скачанном образе:

```shell
d8 k describe cvi ubuntu-22.04
```

### Создание образа из container registry

Образ, хранящийся в container registry имеет определенный формат. Рассмотрим это на примере:

Загрузите образ локально:

```shell
curl -L https://cloud-images.ubuntu.com/minimal/releases/jammy/release/ubuntu-22.04-minimal-cloudimg-amd64.img -o ubuntu2204.img
```

Создайте `Dockerfile` со следующим содержимым:

```shell
FROM scratch
COPY ubuntu2204.img /disk/ubuntu2204.img
```

Соберите образ и загрузите его в container registry. В качестве примера используется [docker.io](https://www.docker.com/).  Для выполнения этих шагов необходимо иметь учетную запись в сервисе и настроенную среду:

```shell
docker build -t docker.io/<username>/ubuntu2204:latest
```

где `username` — имя пользователя, указанное при регистрации [в docker.io](https://www.docker.com/).

Загрузите созданный образ в container registry:

```shell
docker push docker.io/<username>/ubuntu2204:latest
```

Чтобы использовать этот образ, создайте в качестве примера ресурс:

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

### Загрузка образа из командной строки

Чтобы загрузить образ из командной строки, предварительно создайте следующий ресурс, как показано ниже на примере `ClusterVirtualImage`:

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

После создания этого ресурса, он перейдет в фазу `WaitForUserUpload`, что означает, что он готов к загрузке образа.

Существует два варианта загрузки — с узла кластера или с произвольного узла за пределами кластера:

```shell
d8 k get cvi some-image -o jsonpath="{.status.imageUploadURLs}"  | jq

# {
#   "external":"https://virtualization.example.com/upload/g2OuLgRhdAWqlJsCMyNvcdt4o5ERIwmm",
#   "inCluster":"http://10.222.165.239/upload"
# }
```

В качестве примера загрузите образ Cirros:

```shell
curl -L http://download.cirros-cloud.net/0.5.1/cirros-0.5.1-x86_64-disk.img -o cirros.img
```

Затем выполните загрузку образа с помощью следующей команды:

```shell
curl https://virtualization.example.com/upload/g2OuLgRhdAWqlJsCMyNvcdt4o5ERIwmm --progress-bar -T cirros.img | cat
```

После завершения загрузки образ должен быть создан и перейти в фазу Ready. Для проверки состояния образа выполните команду:

```shell
d8 k get cvi some-image
```

В результате будет выведена информация о состоянии образа:

```console
NAME         PHASE   CDROM   PROGRESS   AGE
some-image   Ready   false   100%       1m
```
