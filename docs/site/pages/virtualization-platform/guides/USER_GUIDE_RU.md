---
title: "Руководство пользователя"
permalink: ru/virtualization-platform/guides/user-guide.html
lang: ru
---

{% alert level="info" %}
С детальным описанием параметров настройки ресурсов приведенных в данном документе вы можете ознакомится в разделе [Custom Resources](cr.html)
{% endalert %}

## Быстрый старт

Пример создания виртуальной машины с Ubuntu 22.04.

1. Создайте namespace для виртуальных машин с помощью команды:

   ```bash
   kubectl create ns vms
   ```

2. Создайте диск виртуальной машины из внешнего источника. Пример:

   ```yaml
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualDisk
   metadata:
     name: linux-disk
     namespace: vms
   spec:
     persistentVolumeClaim:
       size: 10Gi
       storageClassName: linstor-thin-r2 # Подставьте ваше название SC `kubectl get storageclass`.
     dataSource:
       type: HTTP
       http:
         url: "https://cloud-images.ubuntu.com/minimal/releases/jammy/release-20230615/ubuntu-22.04-minimal-cloudimg-amd64.img"
   ```

После создания `VirtualDisk` в namespace vms, запустится `pod` с именем `vd-importer-*`, который осуществит загрузку заданного образа.

3. Посмотрите текущий статус ресурса с помощью команды:

   ```bash
   kubectl -n vms get virtualdisk -o wide

   # NAME         PHASE   CAPACITY   PROGRESS   STORAGECLASS        TARGETPVC                                            AGE
   # linux-disk   Ready   10Gi       100%       linstor-thin-r2   vd-linux-disk-2ee8a41a-a0ed-4a65-8718-c18c74026f3c   5m59s
   ```

4. Создайте виртуальную машину из следующей спецификации:

   ```yaml
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualMachine
   metadata:
     name: linux-vm
     namespace: vms
     labels:
       vm: linux
   spec:
     virtualMachineClassName: generic # Класс виртуальный машины, который определяет тп vCPU, политику размера ресурсов и размещение виртуальной машины на узлах кластера.
     runPolicy: AlwaysOn # Виртуальная машина должна быть всегда включена.
     enableParavirtualization: true # Использовать паравиртуализацию (virtio).
     osType: Generic
     bootloader: BIOS
     cpu:
       cores: 1
       coreFraction: 10% # Запросить 10% процессорного времени одного ядра.
     memory:
       size: 1Gi
     provisioning: # Пример cloud-init-сценария для создания пользователя cloud с паролем cloud.
       type: UserData
       userData: |
         #cloud-config
         users:
         - name: cloud
           passwd: $6$rounds=4096$vln/.aPHBOI7BMYR$bBMkqQvuGs5Gyd/1H5DP4m9HjQSy.kgrxpaGEHwkX7KEFV8BS.HZWPitAtZ2Vd8ZqIZRqmlykRCagTgPejt1i.
           shell: /bin/bash
           sudo: ALL=(ALL) NOPASSWD:ALL
           chpasswd: { expire: False }
           lock_passwd: false
           ssh_authorized_keys:
             - ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDTXjTmx3hq2EPDQHWSJN7By1VNFZ8colI5tEeZDBVYAe9Oxq4FZsKCb1aGIskDaiAHTxrbd2efoJTcPQLBSBM79dcELtqfKj9dtjy4S1W0mydvWb2oWLnvOaZX/H6pqjz8jrJAKXwXj2pWCOzXerwk9oSI4fCE7VbqsfT4bBfv27FN4/Vqa6iWiCc71oJopL9DldtuIYDVUgOZOa+t2J4hPCCSqEJK/r+ToHQbOWxbC5/OAufXDw2W1vkVeaZUur5xwwAxIb3wM3WoS3BbwNlDYg9UB2D8+EZgNz1CCCpSy1ELIn7q8RnrTp0+H8V9LoWHSgh3VCWeW8C/MnTW90IR
     blockDeviceRefs:
       - kind: VirtualDisk
         name: linux-disk
   ```

5. Проверьте с помощью команды, что виртуальная машина создана и запущена:

   ```bash
   kubectl -n vms get virtualmachine -o wide

   # NAME       PHASE     CORES   COREFRACTION   MEMORY   NODE           IPADDRESS    AGE
   # linux-vm   Running   1       10%            1Gi      virtlab-pt-1   10.66.10.2   61s
   ```

6. Подключитесь с помощью консоли к виртуальной машине (для выхода из консоли необходимо нажать `Ctrl+]`):

   ```bash
   d8 v console -n vms linux-vm

   # Successfully connected to linux-vm console. The escape sequence is ^]
   #
   # linux-vm login: cloud
   # Password: cloud
   # ...
   # cloud@linux-vm:~$
   ```

## Проектные образы

`VirtualImage` используются для хранения образов виртуальных машин.

Образы могут быть следующих видов:

- Образ диска виртуальной машины, который предназначен для тиражирования идентичных дисков виртуальных машин.
- ISO-образ, содержащий файлы для установки ОС. Этот тип образа подключается к виртуальной машине как cdrom.

Ресурс `VirtualImage` доступен только в том пространстве имен, в котором был создан.

Образы могут быть получены из различных источников, таких как HTTP-серверы, на которых расположены файлы образов, или контейнерные реестры (container registries), где образы сохраняются и становятся доступны для скачивания. Также существует возможность загрузить образы напрямую из командной строки, используя утилиту `curl`.

### Создание и использование образа c HTTP-ресурса

1. Создайте `VirtualImage`:

   ```yaml
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualImage
   metadata:
     name: ubuntu-img
     namespace: vms
   spec:
     storage: ContainerRegistry
     dataSource:
       type: HTTP
       http:
         url: "https://cloud-images.ubuntu.com/minimal/releases/jammy/release-20230615/ubuntu-22.04-minimal-cloudimg-amd64.img"
   ```

2. Проверьте результат с помощью команды:

   ```bash
   kubectl -n vms get virtualimage -o wide

   # NAME         PHASE   CDROM   PROGRESS   STOREDSIZE   UNPACKEDSIZE   REGISTRY URL                                   AGE
   # ubuntu-img   Ready   false   100%       285.9Mi      2.2Gi          dvcr.d8-virtualization.svc/vi/vms/ubuntu-img   29s
   ```

### Создание и использование образа из container registry

Cформируйте образ для хранения в `container registry`.

Ниже представлен пример создания образа c диском Ubuntu 22.04.

- Загрузите образ локально:

  ```bash
  curl -L https://cloud-images.ubuntu.com/minimal/releases/jammy/release-20230615/ubuntu-22.04-minimal-cloudimg-amd64.img -o ubuntu2204.img
  ```

- Создайте Dockerfile со следующим содержимым:

  ```Dockerfile
  FROM scratch
  COPY ubuntu2204.img /disk/ubuntu2204.img
  ```

- Соберите образ и загрузите его в `container registry`. В качестве `container registry` в примере ниже использован docker.io. для выполнения вам необходимо иметь учетную запись сервиса и настроенное окружение.

  ```bash
  docker build -t docker.io/username/ubuntu2204:latest
  ```

  где `username` — имя пользователя, указанное при регистрации в docker.io.

- Загрузите созданный образ в `container registry` с помощью команды:

  ```bash
  docker push docker.io/username/ubuntu2204:latest
  ```

- Чтобы использовать этот образ, создайте в качестве примера ресурс `VirtualImage`:

  ```yaml
  apiVersion: virtualization.deckhouse.io/v1alpha2
  kind: VirtualImage
  metadata:
    name: ubuntu-2204
  spec:
    storage: ContainerRegistry
    dataSource:
      type: ContainerImage
      containerImage:
        image: docker.io/username/ubuntu2204:latest
  ```

- Чтобы посмотреть ресурс и его статус, выполните команду:

  ```bash
  kubectl get virtualimage
  ```

### Загрузка образа из командной строки

1. Чтобы загрузить образ из командной строки, предварительно создайте следующий ресурс, как представлено ниже на примере `VirtualImage`:

   ```yaml
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualImage
   metadata:
     name: some-image
   spec:
     storage: ContainerRegistry
     dataSource:
       type: Upload
   ```

2. После того как ресурс будет создан, проверьте его статус с помощью команды:

   ```bash
   kubectl get virtualimages some-image -o json | jq .status.uploadCommand -r

   > uploadCommand: curl https://virtualization.example.com/upload/dSJSQW0fSOerjH5ziJo4PEWbnZ4q6ffc -T example.iso
   ```

   > ClusterVirtualImage с типом **Upload** ожидает начала загрузки образа 15 минут после создания. По истечении этого срока ресурс перейдет в состояние **Failed**.

3. Загрузите образ Cirros (представлено в качестве примера):

   ```bash
   curl -L http://download.cirros-cloud.net/0.5.1/cirros-0.5.1-x86_64-disk.img -o cirros.img
   ```

4. Выполните загрузку образа:

   ```bash
   curl https://virtualization.example.com/upload/dSJSQW0fSOerjH5ziJo4PEWbnZ4q6ffc -T cirros.img
   ```

   После завершения работы команды `curl` образ должен быть создан.

4. Проверьте, что статус созданного образа `Ready`:

   ```bash
   kubectl get virtualimages -o wide

   # NAME          PHASE   CDROM   PROGRESS   STOREDSIZE   UNPACKEDSIZE   REGISTRY URL                                 AGE
   # some-image    Ready   false   100%       285.9Mi      2.2Gi          dvcr.d8-virtualization.svc/vi/vms/some-image    2m21s
   ```

## Диски

Диски используются в виртуальных машинах для записи и хранения данных. Для хранения дисков используется хранилище, предоставляемое платформой.

Чтобы посмотреть доступные варианты, выполните команду:

```bash
kubectl get storageclass

# NAME                          PROVISIONER              RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
# ceph-pool-r2-csi-rbd          rbd.csi.ceph.com         Delete          WaitForFirstConsumer   true                   85d
# linstor-thin-r1               linstor.csi.linbit.com   Delete          WaitForFirstConsumer   true                   27d
# linstor-thin-r2               linstor.csi.linbit.com   Delete          WaitForFirstConsumer   true                   27d
# linstor-thin-r3               linstor.csi.linbit.com   Delete          WaitForFirstConsumer   true                   27d
```

### Создание пустого диска

> Существует возможность создания пустых дисков.

1. Создайте диск:

   ```yaml
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualDisk
   metadata:
     name: vd-blank
     namespace: vms
   spec:
     persistentVolumeClaim:
       storageClassName: linstor-thin-r2 # Подставьте ваше название SC `kubectl get storageclass`.
       size: 100M
   ```

   Созданный диск можно использовать для подключения к виртуальной машине.

2. Проверьте состояние созданного ресурса с помощью команды:

   ```bash
   kubectl -n vms  get virtualdisk -o wide

   #NAME         PHASE   CAPACITY   PROGRESS   STORAGECLASS        TARGETPVC                                            AGE
   #vd-blank     Ready   97657Ki    100%       linstor-thin-r1     vd-vd-blank-f2284d86-a3fc-40e4-b319-cfebfefea778     46s
   ```

### Создание диска из образа

> Можно создать диски из существующих дисковых образов, а также из внешних ресурсов, таких как образы.

При создании ресурса диска можно указать желаемый размер. Если размер не указан, то будет создан диск с размером, соответствующим исходному образу диска, который хранится в ресурсе `VirtualImage` или `ClusterVirtualImage`. Если необходимо создать диск большего размера, укажите необходимый размер.

В качестве примера рассмотрен ранее созданный `ClusterVirtualImage` с именем `ubuntu-2204`:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDisk
metadata:
  name: ubuntu-root
  namespace: vms
spec:
  persistentVolumeClaim:
    size: 10Gi
    storageClassName: linstor-thin-r2 # Подставьте ваше название SC `kubectl get storageclass`.
  dataSource:
    type: ObjectRef
    objectRef:
      kind: ClusterVirtualImage
      name: ubuntu-img
```

### Изменение размера диска

Размер дисков можно изменить только в сторону увеличения, даже если они подключены к виртуальной машине. Для этого отредактируйте поле `spec.persistentVolumeClaim.size`:

Проверим размер до изменения:

```bash
kubectl -n vms  get virtualdisk ubuntu-root -o wide

# NAME          PHASE   CAPACITY   PROGRESS   STORAGECLASS      TARGETPVC                                             AGE
# ubuntu-root   Ready   10Gi       100%       linstor-thin-r2   vd-ubuntu-root-bef82abc-469d-4b31-b6c4-0a9b2850b956   2m25s
```

Применим изменения:

```bash
kubectl -n vms patch virtualdisk ubuntu-root --type merge -p '{"spec":{"persistentVolumeClaim":{"size":"11Gi"}}}'
```

Проверим размер после изменения:

```bash
kubectl -n vms get virtualdisk ubuntu-root -o wide

# NAME          PHASE   CAPACITY   PROGRESS   STORAGECLASS      TARGETPVC                                             AGE
# ubuntu-root   Ready   11Gi       100%       linstor-thin-r2   vd-ubuntu-root-bef82abc-469d-4b31-b6c4-0a9b2850b956   4m13s
```

### Подключение дисков к запущенным виртуальным машинам

Диски могут быть подключены в работающей виртуальной машине с использованием `VirtualMachineBlockDeviceAttachment` ресурса:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineBlockDeviceAttachment
metadata:
  name: vd-blank-attachment
  namespace: vms
spec:
  virtualMachineName: linux-vm # Имя виртуальной машины, к которой будет подключен диск.
  blockDeviceRef:
    kind: VirtualDisk
    name: vd-blank # Имя подключаемого диска.
```

При удалении ресурса `VirtualMachineBlockDeviceAttachment` диск от виртуальной машины будет отключен.

Чтобы посмотреть список подключенных дисков в работающей виртуальной машине, выполните команду:

```bash
kubectl -n vms get virtualmachineblockdeviceattachments

# NAME                       PHASE
# vd-blank-attachment       Attached
```

## Виртуальные машины

Для создания виртуальной машины используется ресурс `VirtualMachine`, его параметры позволяют сконфигурировать:

- ресурсы, требуемые для работы виртуальной машины (процессор, память, диски и образы);
- правила размещения виртуальной машины на узлах кластера;
- настройки загрузчика и оптимальные параметры для гостевой ОС;
- политику запуска виртуальной машины и политику применения изменений;
- сценарии начальной конфигурации (cloud-init).

### Создание диска для виртуальной машины

Создайте диск с установленной ОС для виртуальной машины:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDisk
metadata:
  name: ubuntu-2204-root
  namespace: vms
spec:
  persistentVolumeClaim:
    size: 10Gi
  dataSource:
    type: HTTP
    http:
      url: "https://cloud-images.ubuntu.com/minimal/releases/jammy/release-20230615/ubuntu-22.04-minimal-cloudimg-amd64.img"
```

### Создание виртуальной машины

Ниже представлен пример простой конфигурации виртуальной машины, запускающей ОС Ubuntu 22.04. В примере используется сценарий первичной инициализации виртуальной машины (cloud-init), который устанавливает пакет **nginx** и создает пользователя `cloud` с паролем `cloud`:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  name: linux-vm
  namespace: vms
  labels:
    vm: linux
spec:
  virtualMachineClassName: generic
  runPolicy: AlwaysOn
  provisioning:
    type: UserData
    userData: |
      #cloud-config
      package_update: true
      packages:
        - nginx
      run_cmd:
        - systemctl daemon-relaod
        - systemctl enable --now nginx
      users:
      - name: cloud
        # password: cloud
        passwd: $6$rounds=4096$vln/.aPHBOI7BMYR$bBMkqQvuGs5Gyd/1H5DP4m9HjQSy.kgrxpaGEHwkX7KEFV8BS.HZWPitAtZ2Vd8ZqIZRqmlykRCagTgPejt1i.
        shell: /bin/bash
        sudo: ALL=(ALL) NOPASSWD:ALL
        chpasswd: { expire: False }
        lock_passwd: false
  cpu:
    cores: 1
  memory:
    size: 2Gi
  blockDeviceRefs:
    # Порядок дисков и образов в данном блоке определяет приоритет загрузки.
    - kind: VirtualDisk
      name: ubuntu-2204-root
```

При наличии приватных данных, сценарий начальной инициализации виртуальной машины может быть создан в Secret'е. Пример Secret'а приведен ниже:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: linux-vm-cloud-init
  namespace: vms
data:
  userData: # Тут cloud-init-конфиг в Base64.
type: Opaque
```

Спецификация виртуальной машины будет выглядеть следующим образом:

```yaml
spec:
  provisioning:
    type: UserDataRef
    userDataRef:
      kind: Secret
      name: linux-vm-cloud-init
```

1. Создайте виртуальную машину из манифеста представленного выше.

   После запуска виртуальная машина должна иметь статус `Ready`.

   ```bash
   kubectl -n vms get virtualmachine

   # NAME       PHASE     NODE          IPADDRESS     AGE
   # linux-vm   Running   node-name-x   10.66.10.1    5m
   ```

После создания виртуальная машина автоматически получит IP-адрес из диапазона, указанного в настройках модуля (блок `virtualMachineCIDRs`).

2. Чтобы зафиксировать IP-адрес виртуальной машины перед ее запуском, выполните следующие шаги:

   - Создайте ресурс `VirtualMachineIPAddress`, в котором зафиксирован желаемый IP-адрес виртуальной машины. Запрашиваемый адрес должен быть из диапазона адресов, указанных в настройках модуля `kubectl get mc virtualization -o jsonpath="{.spec.settings.virtualMachineCIDRs}"`.

     ```yaml
     apiVersion: virtualization.deckhouse.io/v1alpha2
     kind: VirtualMachineIPAddress
     metadata:
       name: <ip-address-name>
       namespace: <namespace>
     spec:
       type: Static
       staticIP: "W.X.Y.Z"
     ```

   - Зафиксируйте изменения в спецификации виртуальной машины:

     ```yaml
     spec:
       virtualMachineIPAddressName: <ip-address-name>
     ```

### Настройка правил размещения виртуальной машины

1. Для того, чтобы виртуальная машина запускалась на заданном наборе узлов, например, на группе узлов `system`, используйте следующий фрагмент конфигурации:

   ```yaml
   spec:
     tolerations:
       - key: "node-role.kubernetes.io/system"
         operator: Exists
         effect: NoSchedule
     nodeSelector:
       node-role.kubernetes.io/system: ""
   ```

2. Внесите изменения в ранее созданную спецификацию виртуальной машины.

### Настройка порядка применения изменений

Внесенные изменения в конфигурацию виртуальной машины не отобразятся, так как по умолчанию применяется политика изменений `Manual`. Для применения изменений виртуальную машину требуется перезагрузить.

1. Чтобы проверить статус виртуальной машины, введите командую:

   ```bash
   kubectl -n vms get linux-vm -o jsonpath='{.status}'
   ```

   В поле `.status.restartAwaitingChanges` отобразятся изменения, которые требуют подтверждения.

2. Создайте и примените ресурс, который отвечает за декларативный способ управления состоянием виртуальной машины, как представлено на примере ниже:

   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualMachineOperation
   metadata:
     name: restart-linux-vm
     namespace: vms
   spec:
     virtualMachineName: linux-vm
     type: Restart
   EOF
   ```

3. Проверьте состояние созданного ресурса:

   ```bash
   kubectl -n vms get virtualmachineoperations restart-linux-vm

   # NAME                PHASE       VM         AGE
   # restart-linux-vm    Completed   linux-vm   1m
   ```

   Если созданный ресурс находится в состоянии `Completed` - перезагрузка виртуальной машины завершилась и новые параметры конфигурации виртуальной машины применены.

   Чтобы изменения в конфигурации виртуальной машины применялись автоматически при ее перезапуске, настройте политику применения изменений следующим образом (пример ниже):

   ```yaml
   spec:
     disruptions:
       approvalMode: Automatic
   ```

### Политика запуска виртуальной машины

1. Подключитесь к виртуальной машине с использованием серийной консоли с помощью команды:

   ```bash
   d8 v console -n vms linux-vm
   ```

2. Завершите работу виртуальной машины с помощью команды:

   ```bash
   cloud@linux-vm$ sudo poweroff
   ```

Далее посмотрим на статус виртуальной машины с помощью команды:

```bash
kubectl -n vms get virtualmachine

# NAME       PHASE     NODE           IPADDRESS   AGE
# linux-vm   Running   node-name-x    10.66.10.1  5m
```

Даже несмотря на то, что виртуальная машина была выключена, она снова запустилась. Причина перезапуска:

> В отличие от традиционных систем виртуализации, мы используем политику запуска для определения состояния виртуальной машины, которая определяет требуемое состояние виртуальной машины в любое время.

> При создании виртуальной машины используется параметр `runPolicy: AlwaysOn`. Это означает, что виртуальная машина будет запущена, даже если по каким-либо причинам произошло ее отключение, перезапуск или сбой, вызвавший прекращение ее работы.

Для выключения виртуальной машины, поменяйте значение политики на `AlwaysOff`. После чего произойдет корректное завершение работы виртуальной машины.
