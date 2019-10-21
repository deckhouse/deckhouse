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

Модуль по-умолчанию **выключен**. Для включения:

1. Корректно [настроить](#настройка-окружения) окружение.
2. Инициализировать deckhouse, передав параметр install.sh — `--extra-config-map-data base64_encoding_of_custom_config`.
3. Настроить параметры модуля.

### Параметры

* `authURL` — OpenStack Identity API URL.
* `caCert` — если OpenStack API имеет self-signed сертификат, можно указать CA x509 сертификат, использовавшийся для подписи.
    * Формат — строка. Сертификат в PEM формате.
    * Опциональный параметр.
* `domainName` — имя домена.
* `tenantName` — имя проекта.
* `username` — имя пользователя с полными правами на проект.
* `password` — пароль к пользователю.
* `region` — регион OpenStack, где будет развёрнут кластер.
* `networkName` — имя сети, которое будет использоваться для внешней коммуникации. При отсутствии `internalNetworkName`, по ней также идёт PodNetwork трафик.
* `internalNetworkName` — имя сети, которое будет использоваться для внутрикластерного взаимодействия. По ней пойдёт PodNetwork трафик.
    * Опциональный параметр.
* `addPodSubnetToPortWhitelist` — разрешить ли ходить в сеть с IP, не соответствующим адресу OpenStack server'а. Нужно для корректно работы flannel бэкэнда `host-hw`. Если `false` или не указано, то flannel запустится с бэкэндом `vxlan`.
    * Формат — bool.
    * Опциональный параметр.
    * **Внимание!** Убедитесь, что у `username` есть доступ на редактирование AllowedAddressPairs на Port'ах, подключенных в указанную сеть:
        * `networkName`, если не указан `internalNetworkName`.
        * `internalNetworkName`, если указан `internalNetworkName`.

        Обычно, в OpenStack, такого доступа нет, если сеть имеет флаг `shared`.
* `zones` — Список зон из `region`, где будут заказываться instances. Является значением по-умолчанию для поля zones в [CloudInstanceGroup](modules/040-cloud-instance-manager/README.md#CloudInstanceGroup-custom-resource) объекте.
    * Формат — массив строк.
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
  authURL: https://test.tests.com:5000/v3/
  domainName: default
  tenantName: default
  username: jamie
  password: nein
  region: HetznerFinland
  networkName: shared
  internalNetworkName: kube
  zones:
  - nova
  sshKeyPairName: my-ssh-keypair
  securityGroups:
  - default
  - allow-ssh-and-icmp
  internalSubnet: "10.0.201.0/16"
```

### OpenStackInstanceClass custom resource

Ресурс описывает параметры группы OpenStack servers, которые будет использовать machine-controller-manager из модуля [cloud-instance-manager](modules/040-cloud-instance-manager). На этот ресурс ссылается ресурс `CloudInstanceClass` из вышеупомянутого модуля.

Все опции идут в `.spec`.

* `flavorName` — тип заказываемых server'ов
* `imageName` — имя образа.
    * **Внимание!** Сейчас поддерживается и тестируется только Ubuntu 18.04.
* `cloudInitSteps` — параметры bootstrap фазы.
    * `version` — версия. По сути, имя директории [здесь](modules/040-cloud-instance-manager/cloud-init-steps).
        * По-умолчанию `ubuntu-18.04-1.0`.
        * **WIP!** Precooked версия требует специально подготовленного образа.
    * `options` — ассоциативный массив параметров. Уникальный для каждой `version` и описано в [`README.md`](modules/040-cloud-instance-manager/cloud-init-steps) соответствующих версий. Пример для [ubuntu-18.04-1.0](modules/040-cloud-instance-manager/cloud-init-steps/ubuntu-18.04-1.0):

        ```yaml
        options:
          kubernetesVersion: "1.15.3"
        ```

#### Пример OpenStackInstanceClass

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackInstanceClass
metadata:
  name: test
spec:
  flavorName: m1.large
  imageName: ubuntu-18-04-cloud-amd64
```

### Storage

Для включения PersistentVolumes необходимо создать StorageClass с нужным OpenStack volume type. Получить список типов можно командой `openstack volume type list`.
Например, для volume type `ceph-ssd`:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ceph-ssd
provisioner: csi-cinderplugin # обязательно такой
parameters:
  type: ceph-ssd
```

## Требования к окружениям

В OpenStack проекте нужно создать:

1. `keypair` (опционально).
2. `network` + `subnet` для внутрикластерного взаимодействия (опционально).
3. `security_group`, с правилами разрешающие трафик между серверами;
4. `compute_instance` с `keypair`, `network` + `subnet`, `security_group`.
    * Если нужен `host-gw` режим flannel, следует добавить опцию `allowed_address_pairs` с PodNetwork CIDR к параметрам `instance`.
5. [Пример](install-kubernetes/openstack/ansible/master.yaml) настройки ОС для master'а через kubeadm.

## Как мне поднять кластер

1. [Настройте](#настройка-окружения) облачное окружение. Возможно, [автоматически](#автоматизированная-подготовка-окружения).
2. [Установите](#включение-модуля) deckhouse с помощью `install.sh`, передав флаг `--extra-config-map-data base64_encoding_of_custom_config` с [параметрами](#параметры) модуля.
3. [Создайте](#OpenStackInstanceClass-custom-resource) один или несколько `OpenStackInstanceClass`
4. Управляйте количеством и процессом заказа машин в облаке с помощью модуля [cloud-instance-manager](modules/040-cloud-instance-manager).
