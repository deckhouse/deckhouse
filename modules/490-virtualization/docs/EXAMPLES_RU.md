---
title: "Модуль virtualization: примеры конфигурации"
---

## Получение списка доступных образов

Deckhouse поставляется уже с несколькими базовыми образами, которые вы можете использовать для создания виртуальных машин. Для того чтобы получить их список, выполните:

```shell
kubectl get cvmi
```

Пример вывода:

```shell
NAME           AGE
alpine-3.16    30d
centos-7       30d
centos-8       30d
debian-9       30d
debian-10      30d
fedora-36      30d
rocky-9        30d
ubuntu-16.04   30d
ubuntu-18.04   30d
ubuntu-20.04   30d
ubuntu-22.04   30d
```

## Создание виртуальной машины

Минимальный ресурс для создания виртуальной машины выглядит так:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachine
metadata:
  name: vm100
  namespace: default
spec:
  running: true
  resources:
    memory: 512M
    cpu: "1"
  userName: ubuntu
  sshPublicKey: "ssh-rsa asdasdkflkasddf..."
  bootDisk:
    source:
      kind: ClusterVirtualMachineImage
      name: ubuntu-22.04
    size: 10Gi
    storageClassName: linstor-thindata-r2
    autoDelete: true
```

В параметре [bootDisk](cr.html#virtualmachine-v1alpha1-spec-bootdisk), можно указать и имя существующего диска виртуальной машины. В этом случае он будет подключен к ней напрямую без выполнения операции клонирования.
Этот параметр также определяет имя создаваемого диска. Если он не указан, то по умолчанию используется шаблон `<vm_name>-boot`.

Пример:

```yaml
bootDisk:
  name: "myos"
  size: 10Gi
  autoDelete: false
```

Параметр [autoDelete](cr.html#virtualmachine-v1alpha1-spec-bootdisk-autodelete) позволяет указать, должен ли диск быть удалён после удаления виртуальной машины.

## Работа с IP-адресами

Для каждой виртуальной машины назначается отдельный IP-адрес, который она использует на протяжении всей своей жизни.  
Для этого используется механизм IPAM (IP Address Management), который представляет собой два ресурса: [VirtualMachineIPAddressClaim](cr.html#virtualmachineipaddressclaim) и [VirtualMachineIPAddressLease](cr.html#virtualmachineipaddresslease).

Ресурс `VirtualMachineIPAddressLease` является кластерным и отражает сам факт выданного для виртуальной машины адреса. Ресурс `VirtualMachineIPAddressClaim` является пользовательским ресурсом и используется для запроса такого адреса. Создав `VirtualMachineIPAddressClaim` вы можете запросить желаемый IP-адрес для виртуальной машины.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressClaim
metadata:
  name: vm100
  namespace: default
spec:
  address: 10.10.10.10
  static: true
```

Желаемый IP-адрес должен находиться в диапазоне сетей, определенных в параметре [vmCIDRs](configuration.html#parameters-vmcidrs) конфигурации модуля, и не должен быть занят какой-либо другой виртуальной машиной.

В случае если `VirtualMachineIPAddressClaim` не был создан пользователем заранее, то он создастся автоматически при создании виртуальной машины. В этом случае будет назначен следующий свободный IP-адрес из диапазона `vmCIDRs`.

При удалении виртуальной машины удалится также и связанный с ней `VirtualMachineIPAddressClaim`. Для того чтобы этого не происходило нужно пометить такой IP-адрес как статический. Для этого нужно отредактировать созданный `VirtualMachineIPAddressClaim` и установить в нём поле `static: true`. После удаления виртуальной машины, статический IP-адрес остаётся зарезервированным в пространстве имен.

Чтобы посмотреть список всех выданных IP-адресов, выполните следующую команду:

```shell
kubectl get vmip
```

Пример вывода команды:

```console
NAME    ADDRESS       STATIC   STATUS   VM      AGE
vm1     10.10.10.0    false    Bound    vm1     9d
vm100   10.10.10.10   true     Bound    vm100   172m
```

Для того чтобы освободить IP-адрес, просто удалите ресурс `VirtualMachineIPAddressClaim`:

```shell
kubectl delete vmip vm100
```

`VirtualMachineIPAddressClaim` по умолчанию называются так же как и виртуальная машина, но можно использовать произвольное имя, указав его в параметре [ipAddressClaimName](cr.html#virtualmachine-v1alpha1-spec-ipaddressclaimname).

## Создание дисков для хранения персистентных данных

Дополнительные диски необходимо создавать вручную.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineDisk
metadata:
  name: mydata
spec:
  storageClassName: linstor-data
  size: 10Gi
```

Имеется возможность создать диск из существующего образа, для этого достаточно указать данные соответствующего ресурса [ClusterVirtualMachineImage](cr.html#clustervirtualmachineimage) в параметре [source](cr.html#virtualmachinedisk-v1alpha1-spec-source).

Пример:

```yaml
source:
  kind: ClusterVirtualMachineImage
  name: centos-7
```

Подключение дополнительных дисков выполняется с помощью параметра [diskAttachments](cr.html#virtualmachine-v1alpha1-spec-diskattachments).

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachine
metadata:
  name: vm100
  namespace: default
spec:
  running: true
  resources:
    memory: 512M
    cpu: "1"
  userName: admin
  sshPublicKey: "ssh-rsa asdasdkflkasddf..."
  bootDisk:
    source:
      kind: ClusterVirtualMachineImage
      name: ubuntu-22.04
    size: 10Gi
    storageClassName: linstor-fast
    autoDelete: true
  diskAttachments:
  - name: mydata
    bus: virtio
```

## Использование cloud-init

При необходимости вы можете передать конфигурацию cloud-init в параметре [cloud-init](cr.html#virtualmachine-v1alpha1-spec-cloudinit).

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachine
metadata:
  name: vm1
  namespace: default
spec:
  running: true
  resources:
    memory: 512M
    cpu: "1"
  userName: admin
  sshPublicKey: "ssh-rsa asdasdkflkasddf..."
  bootDisk:
    source:
      kind: ClusterVirtualMachineImage
      name: ubuntu-22.04
    size: 10Gi
  cloudInit:
    userData: |-
      password: hackme
      chpasswd: { expire: False }
```

Конфигурацию cloud-init можно также хранить в Secret'е и передать виртуальной машине следующим образом:

```yaml
cloudInit:
  secretRef:
    name: my-vmi-secret
```
