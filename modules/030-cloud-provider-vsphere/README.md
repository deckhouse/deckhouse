# Модуль cloud-provider-vsphere

## Содержимое модуля

1. cloud-controller-manager — контроллер для управления ресурсами Vsphere из Kubernetes.
    1. Синхронизирует метаданные vSphere VirtualMachines и Kubernetes Nodes. Удаляет из Kubernetes ноды, которых более нет в vSphere.
2. flannel — DaemonSet. Настраивает PodNetwork между нодами.
3. CSI storage — для заказа дисков на datastore через механизм First-Class Disk.
4. Регистрация в модуле [cloud-instance-manager](modules/040-cloud-instance-manager), чтобы [VsphereInstanceClass'ы](#VsphereInstanceClass-custom-resource) можно было использовать в [CloudInstanceClass'ах](modules/040-cloud-instance-manager/README.md#CloudInstanceGroup-custom-resource).

## Конфигурация

### Включение модуля

Модуль по-умолчанию **выключен**. Для включения:

1. Корректно [настроить](#Требования-к-окружениям) окружение.
2. Установить deckhouse с помощью `install.sh`, добавив ему параметр — `--extra-config-map-data base64_encoding_of_custom_config`.
3. Настроить параметры модуля.

### Параметры

* `host` — домен vCenter сервера.
* `username` — логин.
* `password` — пароль.
* `insecure` — можно выставить в `true`, если vCenter имеет самоподписанный сертификат.
    * Формат — bool.
    * Опциональный параметр. По-умолчанию `false`.
* `regionTagCategory`— имя **категории** тэгов, использующихся для идентификации региона (vSphere Datacenter).
    * Формат — string.
    * Опциональный параметр. По-умолчанию `k8s-region`.
* `zoneTagCategory`: имя **категории** тэгов, использующихся для идентификации зоны (vSphere Cluster).
    * Формат — string.
    * Опциональный параметр. По-умолчанию `k8s-zone`.
* `defaultDatastore`: имя vSphere Datastore, который будет использоваться в качестве default StorageClass.
    * Формат — string.
    * Опциональный параметр. По-умолчанию будет использован лексикографически первый Datastore.
* `region` — тэг, прикреплённый к vSphere Datacenter, в котором будут происходить все операции: заказ VirtualMachines, размещение их дисков на datastore, подключение к network.
* `sshKeys` — список public SSH ключей в plain-text формате.
    * Формат — массив строк.
    * Опциональный параметр. По-умолчанию разрешённых ключей для пользователя по-умолчанию не будет.
* `internalSubnet` — subnet CIDR, использующийся для внутренней межнодовой сети. Используется для настройки параметра `--iface-regex` во flannel.
    * Формат — string. Например, `10.201.0.0/16`.
    * Опциональный параметр.
* `externalNetworkName` — имя сети (не полный путь, а просто имя), подключённой к VirtualMachines, и используемой vsphere-cloud-controller-manager для проставления ExternalIP в `.status.addresses` в Node API объект.
    * Формат — string. Например, `MAIN-1`.
    * Опциональный параметр.
* `internalNetworkName` — имя сети (не полный путь, а просто имя), подключённой к VirtualMachines, и используемой vsphere-cloud-controller-manager для проставления InternalIP в `.status.addresses` в Node API объект.
    * Формат — string. Например, `KUBE-3`.
    * Опциональный параметр.

#### Пример конфигурации

```yaml
cloudProviderVsphereEnabled: "true"
cloudProviderVsphere: |
  host: vc-3.internal
  username: user
  password: password
  insecure: true
  region: moscow-x001
  sshKeys:
  - "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQD5sAcceTHeT6ZnU+PUF1rhkIHG8/B36VWy/j7iwqqimC9CxgFTEi8MPPGNjf+vwZIepJU8cWGB/By1z1wLZW3H0HMRBhv83FhtRzOaXVVHw38ysYdQvYxPC0jrQlcsJmLi7Vm44KwA+LxdFbkj+oa9eT08nQaQD6n3Ll4+/8eipthZCDFmFgcL/IWy6DjumN0r4B+NKHVEdLVJ2uAlTtmiqJwN38OMWVGa4QbvY1qgwcyeCmEzZdNCT6s4NJJpzVsucjJ0ZqbFqC7luv41tNuTS3Moe7d8TwIrHCEU54+W4PIQ5Z4njrOzze9/NlM935IzpHYw+we+YR+Nz6xHJwwj i@my-PC"
  internalSubnet: "10.0.201.0/16"
  externalNetworkName: "MAIN-1"
  internalNetworkName: "KUBE-3"
```

### VsphereInstanceClass custom resource

Ресурс описывает параметры группы vSphere VirtualMachines, которые будет использовать machine-controller-manager из модуля [cloud-instance-manager](modules/040-cloud-instance-manager). На `VsphereInstanceClass` ссылается ресурс `CloudInstanceClass` из вышеупомянутого модуля.

Все опции идут в `.spec`.

* `numCPUs` — количество виртуальных процессорных ядер, выделяемых VirtualMachine.
    * Формат — integer
* `memory` — количество памяти, выделенных VirtualMachine.
    * Формат — integer. В мебибайтах.
* `template` — путь до VirtualMachine Template, который будет склонирован для создания новой VirtualMachine.
    * Пример — `dev/golden_image`
* `virtualMachineFolder` — путь до VirtualMachine Folder, в котором будут создаваться склонированные виртуальные машины.
    * Пример — `dev`
* `network` — путь до network, которая будет подключена к виртуальной машине.
    * Пример — `k8s-msk-178`
* `datastore` — путь до Datastore, на котором будет созданы склонированные виртуальные машины.
    * Пример — `lun-1201`
* `cloudInitSteps` — параметры bootstrap фазы.
    * `version` — версия. По сути, имя директории [здесь](modules/040-cloud-instance-manager/cloud-init-steps).
        * По-умолчанию `ubuntu-18.04-1.0`.
        * **WIP!** Precooked версия требует специально подготовленного образа.
    * `options` — ассоциативный массив параметров. Уникальный для каждой `version` и описано в [`README.md`](modules/040-cloud-instance-manager/cloud-init-steps) соответствующих версий. Пример для [ubuntu-18.04-1.0](modules/040-cloud-instance-manager/cloud-init-steps/ubuntu-18.04-1.0):

        ```yaml
        options:
          kubernetesVersion: "1.15.3"
        ```

#### Пример VsphereInstanceClass

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: VsphereInstanceClass
metadata:
  name: test
spec:
  numCPUs: 2
  memory: 2048
  template: dev/golden_image
  virtualMachineFolder: dev
  network: k8s-msk-178
  datastore: lun-1201
```

### Storage

StorageClass будет создан автоматически для каждого Datastore из зон(-ы). Для указания default StorageClass, необходимо в конфигурацию модуля добавить параметр `defaultDataStore`.

## Требования к окружениям

1. Требования к версии vSphere: `v6.7U2`.
2. vCenter, до которого есть доступ изнутри кластера с master нод.
3. Создать Datacenter, а в нём:

    1. VirtualMachine template со [специальным](https://github.com/vmware/cloud-init-vmware-guestinfo) cloud-init datasource внутри.
        * Подготовить образ Ubuntu 18.04, например, можно с помощью [скрипта](install-kubernetes/vsphere/prepare-template).
    2. Network, доступная на всех ESXi, на которых будут создаваться VirtualMachines.
    3. Datastore (или несколько), подключённый ко всем ESXi, на которых будут создаваться VirtualMachines.
        * На CluDatastore-ы **необходимо** "повесить" тэг из категории тэгов, указанный в `zoneTagCategory` (по-умолчанию, `k8s-zone`). Этот тэг будет обозначать **зону**. Все Cluster'а из конкретной зоны должны иметь доступ ко всем Datastore'ам, с идентичной зоной.
    4. Cluster, в который добавить необходимые используемые ESXi.
        * На Cluster **необходимо** "повесить" тэг из категории тэгов, указанный в `zoneTagCategory` (по-умолчанию, `k8s-zone`). Этот тэг будет обозначать **зону**.
    5. Folder для создаваемых VirtualMachines.
        * Опциональный. По-умолчанию будет использоваться root vm папка.

4. На созданный Datacenter **необходимо** "повесить" тэг из категории тэгов, указанный в `regionTagCategory` (по-умолчанию, `k8s-region`). Этот тэг будет обозначать **регион**.
5. Настроенная(-ые) Kubernetes master ноды. [Пример](install-kubernetes/common/ansible/kubernetes/tasks/master.yml) настройки ОС для master'а через kubeadm. Для созданных vSphere VirtualMachine прописать extraConfig согласно [инструкции](modules/030-cloud-provider-vsphere/docs/csi/disk_uuid.md).

## Как мне поднять кластер

1. Настройте инфраструктурное окружение в соответствии с [требованиями](#требования-к-окружениям) к окружению.
2. [Установите](#включение-модуля) deckhouse с помощью `install.sh`, передав флаг `--extra-config-map-data base64_encoding_of_custom_config` с [параметрами](#параметры) модуля.
3. [Создайте](#VsphereInstanceClass-custom-resource) один или несколько `VsphereInstanceClass`
4. Управляйте количеством и процессом заказа машин в облаке с помощью модуля [cloud-instance-manager](modules/040-cloud-instance-manager).
