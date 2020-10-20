title: "Cloud provider — Openstack: Развертывание"

## Поддерживаемые схемы размещения

Схема размещения описывается объектом `OpenStackClusterConfiguration`. Его поля:

* `layout` — архитектура расположения ресурсов в облаке.
  * Варианты — `Standard`, `StandardWithNoRouter`, `Simple` или `SimpleWithInternalNetwork` (описание ниже).
* `Standard` — настройки для layout'а `Standard`.
  * `internalNetworkCIDR` - адресация для внутренней сети нод кластера
  * `internalNetworkDNSServers` - список адресов рекурсивных DNS внутренней сети
  * `internalNetworkSecurity` - флаг, который определяет необходимо ли настраивать [SecurityGroups](/candi/cloud-providers/openstack/docs#%D0%BA%D0%B0%D0%BA-%D0%BF%D1%80%D0%BE%D0%B2%D0%B5%D1%80%D0%B8%D1%82%D1%8C-%D0%BF%D0%BE%D0%B4%D0%B4%D0%B5%D1%80%D0%B6%D0%B8%D0%B2%D0%B0%D0%B5%D1%82-%D0%BB%D0%B8-%D0%BF%D1%80%D0%BE%D0%B2%D0%B0%D0%B9%D0%B4%D0%B5%D1%80-securitygroups) и [AllowedAddressPairs](https://docs.openstack.org/developer/dragonflow/specs/allowed_address_pairs.html) на портах внутренней сети
  * `externalNetworkName` - имя сети для внешнего взаимодействия
* `StandardWithNoRouter` — настройки для layout'а `StandardWithNoRouter`.
  * `internalNetworkCIDR` - адресация для внутренней сети нод кластера
  * `externalNetworkName` - имя сети для внешнего взаимодействия
  * `externalNetworkDHCP` - флаг, который указывает включен ли DHCP в сети, указанной в качестве внешней
  * `internalNetworkSecurity` - флаг, который определяет необходимо ли настраивать [SecurityGroups](/candi/cloud-providers/openstack/docs#%D0%BA%D0%B0%D0%BA-%D0%BF%D1%80%D0%BE%D0%B2%D0%B5%D1%80%D0%B8%D1%82%D1%8C-%D0%BF%D0%BE%D0%B4%D0%B4%D0%B5%D1%80%D0%B6%D0%B8%D0%B2%D0%B0%D0%B5%D1%82-%D0%BB%D0%B8-%D0%BF%D1%80%D0%BE%D0%B2%D0%B0%D0%B9%D0%B4%D0%B5%D1%80-securitygroups) и [AllowedAddressPairs](https://docs.openstack.org/developer/dragonflow/specs/allowed_address_pairs.html) на портах внутренней сети
* `Simple` — настройки для layout'а `Simple`.
  * `externalNetworkName` - имя сети для внешнего взаимодействия
  * `externalNetworkDHCP` - флаг, который указывает включен ли DHCP в сети, указанной в качестве внешней
  * `podNetworkMode` - определяет способ организации трафика в той сети, которая используется для коммуникации между подами (обычно это internal сеть, но бывают исключения).
    * Допустимые значение:
      * `DirectRouting` – между узлами работает прямая маршрутизация, в этом режиме отключены SecurityGroups.
      * `VXLAN` – между узлами НЕ работает прямая маршрутизация, необходимо использовать VXLAN, в этом режиме отключены SecurityGroups.
* `SimpleWithInternalNetwork` — настройки для layout'а `SimpleWithInternalNetwork`.
  * `internalSubnetName` - имя подсети, в которой будут работать ноды кластера
  * `podNetworkMode` - определяет способ организации трафика в той сети, которая используется для коммуникации между подами (обычно это internal сеть, но бывают исключения).
    * Допустимые значение:
      * `DirectRouting` – между узлами работает прямая маршрутизация, в этом режиме отключены SecurityGroups.
      * `DirectRoutingWithPortSecurityEnabled` - между узлами работает прямая маршрутизация, но только если в OpenStack явно разрешить на Port'ах диапазон адресов используемых во внутренней сети.
        * **Внимание!** Убедитесь, что у `username` есть доступ на редактирование AllowedAddressPairs на Port'ах, подключенных в сеть `internalNetworkName`. Обычно, в OpenStack, такого доступа нет, если сеть имеет флаг `shared`.
      * `VXLAN` – между узлами НЕ работает прямая маршрутизация, необходимо использовать VXLAN, в этом режиме отключены SecurityGroups.
  * `externalNetworkName` - имя сети для внешнего взаимодействия
  * `masterWithExternalFloatingIP` - флаг, который указывает создавать ли floatingIP на мастер нодах
* `provider` — передаются [параметры подключения]((/candi/cloud-providers/openstack/docs#credentials-%D0%B4%D0%BB%D1%8F-%D0%B7%D0%B0%D0%BF%D0%BE%D0%BB%D0%BD%D0%B5%D0%BD%D0%B8%D1%8F-provider)) к api openstack, они совпадают с параметрами, передаваемыми в поле `connection` в модуле [cloud-provider-openstack]({{ site.baseurl }}/modules/030-cloud-provider-openstack/#параметры).
* `masterNodeGroup` — спецификация для описания NG мастера.
  * `replicas` — сколько мастер-узлов создать.
  * `instanceClass` — частичное содержимое полей [OpenStackInstanceClass](/modules/030-cloud-provider-openstack/cr.html#openstackinstanceclass). Обязательными параметрами являются `flavorName`, `imageName`. Допустимые параметры:
    * `flavorName`
    * `imageName`
    * `rootDiskSize`
    * `additionalSecurityGroups`
    * `additionalTags`
  * `volumeTypeMap` — словарь типов дисков для хранения данных etcd и конфигурационных файлов kubernetes. Если указан параметр `rootDiskSize`, то этот же тип диска будет использован под загрузочный диск виртуальной машины. Всегда рекомендуется использовать самые быстрые диски, предоставляемые провайдером.
    * Обязательный параметр.
    * Формат — словарь (ключ - имя зоны, значение - тип диска).
    * Пример:
      ```
        ru-1a: fast-ru-1a
        ru-1b: fast-ru-1b
      ```
      В случае если значение указанное в `replicas` превышает количество элементов в словаре, то мастер ноды, чей номер превышает
      длину словаря получают значения заново начиная с начала словаря. Если для словаря из примера указанно `replicas: 5`, то с типом
      диска `ru-1a` будут master-0, master-2, master-4, а с типом диска `ru-1b` будут master-1, master-3.
* `nodeGroups` — массив дополнительных NG для создания статичных узлов (например, для выделенных фронтов или шлюзов). Настройки NG:
  * `name` — имя NG, будет использоваться для генерации имени нод.
  * `replicas` — сколько узлов создать.
  * `instanceClass` — частичное содержимое полей [OpenStackInstanceClass](/modules/030-cloud-provider-openstack/cr.html#openstackinstanceclass). Обязательными параметрами являются `flavorName`, `imageName`, `mainNetwork`. Допустимые параметры:
    * `flavorName`
    * `imageName`
    * `rootDiskSize`
    * `mainNetwork`
    * `additionalSecurityGroups`
    * `additionalTags`
    * `additionalNetworks`
    * `networksWithSecurityDisabled` - в этом списке необходимо перечислить все сети из `mainNetwork` и `additionalNetworks`, в которых **НЕЛЬЗЯ** настраивать `SecurityGroups` и `AllowedAddressPairs` на портах.
      * Формат — массив строк.
    * `floatingIPPools` - список сетей, в которых заказывать Floating IP для нод
      * Формат — массив строк.
    * `configDrive` - флаг, указывающий будет ли монтироваться на ноду дополнительный диск, содержащий конфигурацию для бутстрапа ноды. Необходимо устанавливать, если в сети, указанной в качестве `mainNetwork` отключен DHCP.
      * Опциональный параметр.
      * По-умолчанию `false`
  * `zones` — ограничение набора зон, в которых разрешено создавать ноды.
    * Опциональный параметр.
    * Формат — массив строк.
* `sshPublicKey` — публичный ключ для доступа на ноды.
  * Обязательный параметр.
  * Формат — строкa.
* `tags` - словарь тегов, которые будут созданы на всех ресурсах, имеющих такую возможность
  * Опциональный параметр.
  * Формат — ключ-значение.

### Standard
Создаётся внутренняя сеть кластера со шлюзом в публичную сеть, ноды не имеют публичных ip адресов. Для мастера заказывается
floating ip.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vSTIcQnxcwHsgANqHE5Ry_ZcetYX2lTFdDjd3Kip5cteSbUxwRjR3NigwQzyTMDGX10_Avr_mizOB5o/pub?w=960&h=720)
<!--- Исходник: https://docs.google.com/drawings/d/1hjmDn2aJj3ru3kBR6Jd6MAW3NWJZMNkend_K43cMN0w/edit --->

```
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackClusterConfiguration
layout: Standard
standard:
  internalNetworkCIDR: 192.168.199.0/24                   # required
  internalNetworkDNSServers:                              # required
  - 8.8.8.8
  - 4.2.2.2
  internalNetworkSecurity: true|false                     # optional, default true
  externalNetworkName: shared                             # required
masterNodeGroup:
  replicas: 3
  instanceClass:
    flavorName: m1.large                                  # required
    imageName: ubuntu-18-04-cloud-amd64                   # required
    rootDiskSize: 50                                      # optional, local disk is used if not specified
    additionalSecurityGroups:                             # optional, additional security groups
    - sec_group_1
    - sec_group_2
    additionalTags:
      severity: critical
      environment: production
  volumeTypeMap:                                          # required, volume type map for etcd and kubernetes certs (always use fastest disk supplied by provider).
    ru-1a: fast-ru-1a                                     # If rootDiskSize specified than this volume type will be also used for master root volume
    ru-1b: fast-ru-1b
    ru-1c: fast-ru-1c
nodeGroups:
- name: front
  replicas: 2
  instanceClass:
    flavorName: m1.small                                  # required
    imageName: ubuntu-18-04-cloud-amd64                   # required
    rootDiskSize: 20                                      # optional, local disk is used if not specified
    configDrive: false                                    # optional, default false, determines if config drive is required during vm bootstrap process. It's needed if there is no dhcp in network that is used as default gateway
    mainNetwork: kube                                     # required, network will be used as default gateway
    additionalNetworks:                                   # optional
    - office
    - shared
    networksWithSecurityDisabled:                         # optional, if there are networks with disabled port security their names must be specified 
    - office
    floatingIPPools:                                      # optional, list of network pools where to order floating ips
    - public
    - shared
    additionalSecurityGroups:                             # optional, additional security groups
    - sec_group_1
    - sec_group_2
  zones:
  - ru-1a
  - ru-1b
sshPublicKey: "ssh-rsa ewasfef3wqefwefqf43qgqwfsd"
tags:
  project: cms
  owner: default
provider:
  ...
```

### StandardWithNoRouter
Создаётся внутренняя сеть кластера без доступа в публичную сеть. Все ноды, включая мастер, создаются с двумя интерфейсами:
один в публичную сеть, другой во внутреннюю сеть. Данная схема размещения должна использоваться, если необходимо, чтобы
все ноды кластера были доступны напрямую.

**Внимание**
В данной конфигурации не поддерживается LoadBalancer. Это связано с тем, что в openstack нельзя заказать floating ip для
сети без роутера, поэтому нельзя заказать балансировщик с floating ip. Если заказывать internal loadbalancer, у которого
virtual ip создаётся в публичной сети, то он всё равно доступен только с нод кластера.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vR9Vlk22tZKpHgjOeQO2l-P0hyAZiwxU6NYGaLUsnv-OH0so8UXNnvrkNNiAROMHVI9iBsaZpfkY-kh/pub?w=960&h=720)
<!--- Исходник: https://docs.google.com/drawings/d/1gkuJhyGza0bXB2lcjdsQewWLEUCjqvTkkba-c5LtS_E/edit --->

```
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackClusterConfiguration
layout: StandardWithNoRouter
standardWithNoRouter:
  internalNetworkCIDR: 192.168.199.0/24                   # required
  externalNetworkName: ext-net                            # required
  externalNetworkDHCP: false                              # optional, whether dhcp is enabled in specified external network (default true)   
  internalNetworkSecurity: true|false                     # optional, default true
masterNodeGroup:
  replicas: 3
  instanceClass:
    flavorName: m1.large                                  # required
    imageName: ubuntu-18-04-cloud-amd64                   # required
    rootDiskSize: 50                                      # optional, local disk is used if not specified
    additionalSecurityGroups:                             # optional, additional security groups
    - sec_group_1
    - sec_group_2
  volumeTypeMap:                                          # required, volume type map for etcd and kubernetes certs (always use fastest disk supplied by provider).
    nova: ceph-ssd                                        # If rootDiskSize specified than this volume type will be also used for master root volume
nodeGroups:
- name: front
  replicas: 2
  instanceClass:
    flavorName: m1.small                                  # required
    imageName: ubuntu-18-04-cloud-amd64                   # required
    rootDiskSize: 20                                      # optional, local disk is used if not specified
    configDrive: false                                    # optional, default false, determines if config drive is required during vm bootstrap process. It's needed if there is no dhcp in network that is used as default gateway
    mainNetwork: kube                                     # required, network will be used as default gateway
    additionalNetworks:                                   # optional
    - office
    - shared
    networksWithSecurityDisabled:                         # optional, if there are networks with disabled port security their names must be specified 
    - office
    floatingIPPools:                                      # optional, list of network pools where to order floating ips
    - public
    - shared
    additionalSecurityGroups:                             # optional, additional security groups
    - sec_group_1
    - sec_group_2
sshPublicKey: "ssh-rsa ewasfef3wqefwefqf43qgqwfsd"
provider:
  ...
```

### Simple

Master нода и ноды кластера подключаются к существующей сети. Данная схема размещения может понадобиться, если необходимо
объединить кластер кубернетес с уже имеющимися виртуальными машинами.

**Внимание!**

В данной конфигурации не поддерживается LoadBalancer. Это связано с тем, что в openstack нельзя заказать floating ip для
сети без роутера, поэтому нельзя заказать балансировщик с floating ip. Если заказывать internal loadbalancer, у которого
virtual ip создаётся в публичной сети, то он всё равно доступен только с нод кластера.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vTZbaJg7oIvoh2hkEW-DKbqeujhOiJtv_JSvfvDfXE9-mX_p6uggoY1Z9N2EAJ79c7IMfQC9ttQAmaP/pub?w=960&h=720) 
<!--- Исходник: https://docs.google.com/drawings/d/1l-vKRNA1NBPIci3Ya8r4dWL5KA9my7_wheFfMR38G10/edit --->

```
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackClusterConfiguration
layout: Simple
simple:
  externalNetworkName: ext-net                            # required
  externalNetworkDHCP: false                              # optional, default true
  podNetworkMode: VXLAN                                   # optional, by default VXLAN, may also be DirectRouting or DirectRoutingWithPortSecurityEnabled
masterNodeGroup:
  replicas: 3
  instanceClass:
    flavorName: m1.large                                  # required
    imageName: ubuntu-18-04-cloud-amd64                   # required
    rootDiskSize: 50                                      # optional, local disk is used if not specified
    additionalSecurityGroups:                             # optional, additional security groups
    - sec_group_1
    - sec_group_2
  volumeTypeMap:                                          # required, volume type map for etcd and kubernetes certs (always use fastest disk supplied by provider).
    nova: ceph-ssd                                        # If rootDiskSize specified than this volume type will be also used for master root volume
nodeGroups:
- name: front
  replicas: 2
  instanceClass:
    flavorName: m1.small                                  # required
    imageName: ubuntu-18-04-cloud-amd64                   # required
    rootDiskSize: 20                                      # optional, local disk is used if not specified
    configDrive: false                                    # optional, default false, determines if config drive is required during vm bootstrap process. It's needed if there is no dhcp in network that is used as default gateway
    mainNetwork: kube                                     # required, network will be used as default gateway
    additionalNetworks:                                   # optional
    - office
    - shared
    networksWithSecurityDisabled:                         # optional, if there are networks with disabled port security their names must be specified 
    - office
    floatingIPPools:                                      # optional, list of network pools where to order floating ips
    - public
    - shared
    additionalSecurityGroups:                             # optional, additional security groups
    - sec_group_1
    - sec_group_2
sshPublicKey: "ssh-rsa ewasfef3wqefwefqf43qgqwfsd"
provider:
  ...
```

### SimpleWithInternalNetwork

Master нода и ноды кластера подключаются к существующей сети. Данная схема размещения может понадобиться, если необходимо
объединить кластер кубернетес с уже имеющимися виртуальными машинами.

**Внимание!**

В данной схеме размещения не происходит управление `SecurityGroups`, а подразумевается что они были ранее созданы.
Для настройки политик безопасности необходимо явно указывать как `additionalSecurityGroups` в OpenStackClusterConfiguration
для masterNodeGroup и других nodeGroups, так и `additionalSecurityGroups` при создании `OpenStackInstanceClass` в кластере.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vQOcYZPtHBqMtlNx9PDcMrqI0WEwRssL-oXONnrOoKNaIx1fcEODo9dK2zOoF1wbKeKJlhphFTuefB-/pub?w=960&h=720) 
<!--- Исходник: https://docs.google.com/drawings/d/1H9HGOn4abpmZwIhpwwdZSSO9izvyOZakG8HpmmzZZEo/edit --->


```
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackClusterConfiguration
layout: SimpleWithInternalNetwork
simpleWithInternalNetwork:
  internalSubnetName: pivot-standard                      # required, all cluster nodes have to be in the same subnet
  podNetworkMode: DirectRoutingWithPortSecurityEnabled    # optional, by default DirectRoutingWithPortSecurityEnabled, may also be DirectRouting or VXLAN
  externalNetworkName: ext-net                            # optional, if set will be used for load balancer default configuration and ordering master floating ip
  masterWithExternalFloatingIP: false                     # optional, default value is true
masterNodeGroup:
  replicas: 3
  instanceClass:
    flavorName: m1.large                                  # required
    imageName: ubuntu-18-04-cloud-amd64                   # required
    rootDiskSize: 50                                      # optional, local disk is used if not specified
    additionalSecurityGroups:                             # optional, additional security groups
    - sec_group_1
    - sec_group_2
  volumeTypeMap:                                          # required, volume type map for etcd and kubernetes certs (always use fastest disk supplied by provider).
    nova: ceph-ssd                                        # If rootDiskSize specified than this volume type will be also used for master root volume
nodeGroups:
- name: front
  replicas: 2
  instanceClass:
    flavorName: m1.small                                  # required
    imageName: ubuntu-18-04-cloud-amd64                   # required
    rootDiskSize: 20                                      # optional, local disk is used if not specified
    configDrive: false                                    # optional, default false, determines if config drive is required during vm bootstrap process. It's needed if there is no dhcp in network that is used as default gateway
    mainNetwork: kube                                     # required, network will be used as default gateway
    additionalNetworks:                                   # optional
    - office
    - shared
    networksWithSecurityDisabled:                         # optional, if there are networks with disabled port security their names must be specified 
    - office
    floatingIPPools:                                      # optional, list of network pools where to order floating ips
    - public
    - shared
    additionalSecurityGroups:                             # optional, additional security groups
    - sec_group_1
    - sec_group_2
sshPublicKey: "ssh-rsa ewasfef3wqefwefqf43qgqwfsd"
provider:
  ...
```

## Credentials для заполнения provider

На данный момент deckhouse подключается к api openstack, используя доступы пользователя, с которыми он обращается к openstack cli.
Все необходимые данные указаны в openrc файле, который можно скачать по [инструкции](https://docs.openstack.org/zh_CN/user-guide/common/cli-set-environment-variables-using-openstack-rc.html).
Если у вашего провайдера свой собственный web интерфейс, то шаги для скачивания openrc файла могут отличаться. Ниже инструкция для MCS и Selectel.

### MCS - mail.ru cloud solutions

1. Перейти по [ссылке](https://mcs.mail.ru/app/project/keys/)
2. На открывшейся странице нажать на кнопку "Скачать openrc версии 3"

### Selectel

1. Перейти по [ссылке](https://my.selectel.ru/vpc)
2. В левом боковом меню выбрать пункт "Доступ"
3. В открывшемся окне выбрать пользователя и нажать на кнопку "Скачать"


## Как проверить поддерживает ли провайдер SecurityGroups

Достаточно выполнить команду `openstack security group list`. Если в ответ вы не получите ошибок, то это значит, что [Security Groups](https://docs.openstack.org/nova/pike/admin/security-groups.html) поддерживаются.
