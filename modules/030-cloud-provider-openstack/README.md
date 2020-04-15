# Модуль cloud-provider-openstack

## Содержимое модуля

1. cloud-controller-manager — контроллер для управления ресурсами OpenStack из Kubernetes.
    1. Создаёт LoadBalancer'ы для Service-объектов Kubernetes с типом LoadBalancer.
    2. Синхронизирует метаданные OpenStack Servers и Kubernetes Nodes. Удаляет из Kubernetes ноды, которых более нет в OpenStack.
2. flannel — DaemonSet. Настраивает PodNetwork между нодами.
3. CSI storage — для заказа дисков в Cinder (block). Manilla (filesystem) пока не поддерживается.
4. Регистрация в модуле [cloud-instance-manager](modules/040-cloud-instance-manager), чтобы [OpenStackInstanceClass'ы](#OpenStackInstanceClass) можно было использовать в [CloudInstanceClass'ах](modules/040-cloud-instance-manager/README.md#CloudInstanceGroup-custom-resource).

## Конфигурация

### Включение модуля

Модуль автоматически включается для всех облачных кластеров развёрнутых в openstack.  

### Параметры
Настройки модуля устанавливаются автоматически на основании [выбранной схемы размещения](cluster-manager/README.md).

Если вам необходимо настроить модуль, потому что, например, у вас bare metal кластер, для которого нужно включить
возможность добавлять дополнительные инстансы из OpenStack, то смотрите параметры ниже
<details>
<summary>Развернуть</summary>

> **Внимание!** При изменении конфигурационных параметров приведенных в этой секции (параметров, указываемых в ConfigMap deckhouse) **перекат существующих Machines НЕ производится** (новые Machines будут создаваться с новыми параметрами). Перекат происходит только при изменении параметров `CloudInstanceGroup` и `OpenStackInstanceClass`. См. подробнее в документации модуля [cloud-instance-manager](/modules/040-cloud-instance-manager/README.md#Как-мне-перекатить-машины-с-новой-конфигурацией).

* `connection` - Параметры подключения к api cloud provider'a
    * `authURL` — OpenStack Identity API URL.
    * `caCert` — если OpenStack API имеет self-signed сертификат, можно указать CA x509 сертификат, использовавшийся для подписи.
        * Формат — строка. Сертификат в PEM формате.
        * Опциональный параметр.
    * `domainName` — имя домена.
    * `tenantName` — имя проекта.
        * Не может использоваться вместе с `tenantID`.
    * `tenantID` — id проекта.
        * Не может использоваться вместе с `tenantName`.
    * `username` — имя пользователя с полными правами на проект.
    * `password` — пароль к пользователю.
    * `region` — регион OpenStack, где будет развёрнут кластер.
* `internalNetworkNames` — имена сетей, подключённые к виртуальной машине, и используемые cloud-controller-manager для проставления InternalIP в `.status.addresses` в Node API объект.
    * Формат — массив строк. Например,
        ```yaml
        internalNetworkNames:
        - KUBE-3
        - devops-internal
        ```
* `externalNetworkNames` — имена сетей, подключённые к виртуальной машине, и используемые cloud-controller-manager для проставления ExternalIP в `.status.addresses` в Node API объект.
    * Формат — массив строк. Например,

        ```yaml
        internalNetworkNames:
        - KUBE-3
        - devops-internal
        ```
* `podNetworkMode` - определяет способ организации трафика в той сети, которая используется для коммуникации между подами (обычно это internal сеть, но бывают исключения).
    * Допустимые значение:
      * `DirectRouting` – между узлами работает прямая маршрутизация.
      * `DirectRoutingWithPortSecurityEnabled` - между узлами работает прямая маршрутизация, но только если в OpenStack явно разрешить на Port'ах диапазон адресов используемых во внутренней сети.
          * **Внимание!** Убедитесь, что у `username` есть доступ на редактирование AllowedAddressPairs на Port'ах, подключенных в сеть `internalNetworkName`. Обычно, в OpenStack, такого доступа нет, если сеть имеет флаг `shared`.
      * `VXLAN` – между узлами НЕ работает прямая маршрутизация, необходимо использовать VXLAN.
    * Опциональный параметр. По-умолчанию `DirectRoutingWithPortSecurityEnabled`.
* `instances` — параметры instances, которые используются при создании:
    * `sshKeyPairName` — имя OpenStack ресурса `keypair`, который будет использоваться при заказе instances.
        * Опциональный параметр.
    * `securityGroups` — Список securityGroups, которые нужно прикрепить к заказанным instances. Используется для задания firewall правил по отношению к заказываемым instances.
        * Опциональный параметр.
        * Формат — массив строк.
* `internalSubnet` — subnet CIDR, использующийся для внутренней межнодовой сети. Используется для настройки параметра `--iface-regex` во flannel.
    * Формат — string. Например, `10.201.0.0/16`.
    * Опциональный параметр.

#### Пример конфигурации

```yaml
cloudProviderOpenstackEnabled: "true"
cloudProviderOpenstack: |
  connection:
    authURL: https://test.tests.com:5000/v3/
    domainName: default
    tenantName: default
    username: jamie
    password: nein
    region: HetznerFinland
  externalNetworkNames:
  - public
  internalNetworkNames:
  - kube
  instances:
    sshKeyPairName: my-ssh-keypair
    securityGroups:
    - default
    - allow-ssh-and-icmp
  internalSubnet: "10.0.201.0/16"
```
</details>

### Заказ нод в кластере

Управляйте количеством и процессом заказа машин в облаке с помощью модуля [cloud-instance-manager](modules/040-cloud-instance-manager).

#### OpenStackInstanceClass custom resource

Ресурс описывает параметры группы OpenStack servers, которые будет использовать machine-controller-manager из модуля [cloud-instance-manager](modules/040-cloud-instance-manager). На этот ресурс ссылается ресурс `CloudInstanceClass` из вышеупомянутого модуля.

Все опции идут в `.spec`.

* `flavorName` — тип заказываемых server'ов
* `imageName` — имя образа.
    * **Внимание!** Сейчас поддерживается и тестируется только Ubuntu 18.04.
    * Увидеть список всех доступных образов можно найти командой: `openstack image list`
* `rootDiskSize` — если параметр присутствует, OpenStack server будет создан на Cinder volume с указанным размером и стандартным для кластера типом.
    * Опциональный параметр.
    * Формат — integer. В гигабайтах.
* `mainNetwork` — путь до network, которая будет подключена к виртуальной машине, как основная сеть (шлюз по-умолчанию).
* `additionalNetworks` - список сетей, которые будут подключены к инстансу.
    * Опциональный параметр.
    * Формат — массив строк.
    * Пример:
    ```yaml
    - enp6t4snovl2ko4p15em
    - enp34dkcinm1nr5999lu
    ```

##### Пример OpenStackInstanceClass

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackInstanceClass
metadata:
  name: test
spec:
  flavorName: m1.large
  imageName: ubuntu-18-04-cloud-amd64
```

## Как мне поднять гибридный (вручную заведённые ноды) кластер?

1. Удалить flannel из kube-system: `kubectl -n kube-system delete ds flannel-ds`;
2. [Включить](#Пример-конфигурации) модуль и прописать ему необходимые для работы параметры.

**Важно!** Cloud-controller-manager синхронизирует состояние между OpenStack и Kubernetes, удаляя из Kubernetes те узлы, которых нет в OpenStack. В гибридном кластере такое поведение не всегда соответствует потребности, поэтому если узел кубернетес запущен не с параметром `--cloud-provider=external`, то он автоматически игнорируется (Deckhouse прописывает `static://` в ноды в в `.spec.providerID`, а cloud-controller-manager такие узлы игнорирует).

### Подключение storage в гибридном кластере

Если вам требуются PersistentVolumes на нодах, подключаемых к кластеру из openstack, то необходимо создать StorageClass с нужным OpenStack volume type. Получить список типов можно командой `openstack volume type list`.
Например, для volume type `ceph-ssd`:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ceph-ssd
provisioner: csi-cinderplugin # обязательно такой
parameters:
  type: ceph-ssd
volumeBindingMode: WaitForFirstConsumer
```
