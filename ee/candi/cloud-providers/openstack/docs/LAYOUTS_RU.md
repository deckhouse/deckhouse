---
title: "Cloud provider — Openstack: схемы размещения"
---

## Схемы размещения
### Standard
Создаётся внутренняя сеть кластера со шлюзом в публичную сеть, узлы не имеют публичных IP-адресов. Для master-узла заказывается floating ip.

**Внимание**
Если провайдер не поддерживает SecurityGroups, то все приложения запущенные на нодах с FloatingIp будут доступны по белому IP.
Например, kube-apiserver на мастерах будет доступен по 6443 порту. Чтобы избежать этого, рекомендуется использовать схему размещения SimpleWithInternalNetwork.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vSTIcQnxcwHsgANqHE5Ry_ZcetYX2lTFdDjd3Kip5cteSbUxwRjR3NigwQzyTMDGX10_Avr_mizOB5o/pub?w=960&h=720)
<!--- Исходник: https://docs.google.com/drawings/d/1hjmDn2aJj3ru3kBR6Jd6MAW3NWJZMNkend_K43cMN0w/edit --->

```yaml
apiVersion: deckhouse.io/v1
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
sshPublicKey: "ssh-rsa <SSH_PUBLIC_KEY>"
tags:
  project: cms
  owner: default
provider:
  ...
```

### StandardWithNoRouter
Создаётся внутренняя сеть кластера без доступа в публичную сеть. Все ноды, включая мастер, создаются с двумя интерфейсами:
один в публичную сеть, другой во внутреннюю сеть. Данная схема размещения должна использоваться, если необходимо, чтобы
все узлы кластера были доступны напрямую.

**Внимание**
В данной конфигурации не поддерживается LoadBalancer. Это связано с тем, что в openstack нельзя заказать floating ip для
сети без роутера, поэтому нельзя заказать балансировщик с floating ip. Если заказывать internal loadbalancer, у которого
virtual ip создаётся в публичной сети, то он всё равно доступен только с нод кластера.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vR9Vlk22tZKpHgjOeQO2l-P0hyAZiwxU6NYGaLUsnv-OH0so8UXNnvrkNNiAROMHVI9iBsaZpfkY-kh/pub?w=960&h=720)
<!--- Исходник: https://docs.google.com/drawings/d/1gkuJhyGza0bXB2lcjdsQewWLEUCjqvTkkba-c5LtS_E/edit --->

```yaml
apiVersion: deckhouse.io/v1
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
sshPublicKey: "ssh-rsa <SSH_PUBLIC_KEY>"
provider:
  ...
```

### Simple

Master-узел и узлы кластера подключаются к существующей сети. Данная схема размещения может понадобиться, если необходимо
объединить кластер Kubernetes с уже имеющимися виртуальными машинами.

**Внимание!**

В данной конфигурации не поддерживается LoadBalancer. Это связано с тем, что в openstack нельзя заказать floating ip для
сети без роутера, поэтому нельзя заказать балансировщик с floating ip. Если заказывать internal loadbalancer, у которого
virtual ip создаётся в публичной сети, то он всё равно доступен только с нод кластера.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vTZbaJg7oIvoh2hkEW-DKbqeujhOiJtv_JSvfvDfXE9-mX_p6uggoY1Z9N2EAJ79c7IMfQC9ttQAmaP/pub?w=960&h=720)
<!--- Исходник: https://docs.google.com/drawings/d/1l-vKRNA1NBPIci3Ya8r4dWL5KA9my7_wheFfMR38G10/edit --->

```yaml
apiVersion: deckhouse.io/v1
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
sshPublicKey: "ssh-rsa <SSH_PUBLIC_KEY>"
provider:
  ...
```

### SimpleWithInternalNetwork

Master-узел и узлы кластера подключаются к существующей сети. Данная схема размещения может понадобиться, если необходимо
объединить кластер Kubernetes с уже имеющимися виртуальными машинами.

**Внимание!**

В данной схеме размещения не происходит управление `SecurityGroups`, а подразумевается что они были ранее созданы.
Для настройки политик безопасности необходимо явно указывать `additionalSecurityGroups` в OpenStackClusterConfiguration
для masterNodeGroup и других nodeGroups, и `additionalSecurityGroups` при создании `OpenStackInstanceClass` в кластере.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vQOcYZPtHBqMtlNx9PDcMrqI0WEwRssL-oXONnrOoKNaIx1fcEODo9dK2zOoF1wbKeKJlhphFTuefB-/pub?w=960&h=720)
<!--- Исходник: https://docs.google.com/drawings/d/1H9HGOn4abpmZwIhpwwdZSSO9izvyOZakG8HpmmzZZEo/edit --->

```yaml
apiVersion: deckhouse.io/v1
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
sshPublicKey: "ssh-rsa <SSH_PUBLIC_KEY>"
provider:
  ...
```

## OpenStackClusterConfiguration
Схема размещения (layout) описывается структурой `OpenStackClusterConfiguration`:
* `layout` — название схемы размещения.
  * Варианты — `Standard`, `StandardWithNoRouter`, `Simple` или `SimpleWithInternalNetwork` (описание ниже).
* `Standard` — настройки для схемы размещения `Standard`.
  * `internalNetworkCIDR` - адресация для внутренней сети нод кластера
  * `internalNetworkDNSServers` - список адресов рекурсивных DNS внутренней сети
  * `internalNetworkSecurity` - флаг, который определяет необходимо ли настраивать [SecurityGroups](faq.html#как-проверить-поддерживает-ли-провайдер-securitygroups) и [AllowedAddressPairs](https://docs.openstack.org/developer/dragonflow/specs/allowed_address_pairs.html) на портах внутренней сети
  * `externalNetworkName` - имя сети для внешнего взаимодействия
* `StandardWithNoRouter` — настройки для схемы размещения `StandardWithNoRouter`.
  * `internalNetworkCIDR` - адресация для внутренней сети нод кластера
  * `externalNetworkName` - имя сети для внешнего взаимодействия
  * `externalNetworkDHCP` - флаг, который указывает включен ли DHCP в сети, указанной в качестве внешней
  * `internalNetworkSecurity` - флаг, который определяет необходимо ли настраивать [SecurityGroups](faq.html#как-проверить-поддерживает-ли-провайдер-securitygroups) и [AllowedAddressPairs](https://docs.openstack.org/developer/dragonflow/specs/allowed_address_pairs.html) на портах внутренней сети
* `Simple` — настройки для схемы размещения `Simple`.
  * `externalNetworkName` - имя сети для внешнего взаимодействия
  * `externalNetworkDHCP` - флаг, который указывает включен ли DHCP в сети, указанной в качестве внешней
  * `podNetworkMode` - определяет способ организации трафика в той сети, которая используется для коммуникации между подами (обычно это internal сеть, но бывают исключения).
    * Допустимые значение:
      * `DirectRouting` – между узлами работает прямая маршрутизация, в этом режиме отключены SecurityGroups.
      * `VXLAN` – между узлами НЕ работает прямая маршрутизация, необходимо использовать VXLAN, в этом режиме отключены SecurityGroups.
* `SimpleWithInternalNetwork` — настройки для схемы размещения `SimpleWithInternalNetwork`.
  * `internalSubnetName` - имя подсети, в которой будут работать узлы кластера
  * `podNetworkMode` - определяет способ организации трафика в той сети, которая используется для коммуникации между подами (обычно это internal сеть, но бывают исключения).
    * Допустимые значение:
      * `DirectRouting` – между узлами работает прямая маршрутизация, в этом режиме отключены SecurityGroups.
      * `DirectRoutingWithPortSecurityEnabled` - между узлами работает прямая маршрутизация, но только если в OpenStack явно разрешить на Port'ах диапазон адресов используемых во внутренней сети.
        * **Внимание!** Убедитесь, что у `username` есть доступ на редактирование AllowedAddressPairs на Port'ах, подключенных в сеть `internalNetworkName`. Обычно, в OpenStack, такого доступа нет, если сеть имеет флаг `shared`.
      * `VXLAN` – между узлами НЕ работает прямая маршрутизация, необходимо использовать VXLAN, в этом режиме отключены SecurityGroups.
  * `externalNetworkName` - имя сети для внешнего взаимодействия
  * `masterWithExternalFloatingIP` - флаг, который указывает создавать ли floatingIP на мастер нодах
* `provider` — передаются [параметры подключения](environment.html) к API OpenStack, они совпадают с параметрами, передаваемыми в поле `connection` в модуле [cloud-provider-openstack](/ru/documentation/v1/modules/030-cloud-provider-openstack/configuration.html).
* `masterNodeGroup` — спецификация для описания NG мастера.
  * `replicas` — сколько мастер-узлов создать.
  * `instanceClass` — частичное содержимое полей [OpenStackInstanceClass](cr.html#openstackinstanceclass). Обязательными параметрами являются `flavorName`, `imageName`. Допустимые параметры:
    * `flavorName`
    * `imageName`
    * `rootDiskSize`
    * `additionalSecurityGroups`
    * `additionalTags`
  * `volumeTypeMap` — словарь типов дисков для хранения данных etcd и конфигурационных файлов kubernetes. Если указан параметр `rootDiskSize`, то этот же тип диска будет использован под загрузочный диск виртуальной машины. Всегда рекомендуется использовать самые быстрые диски, предоставляемые провайдером.
    * Обязательный параметр.
    * Формат — словарь (ключ - имя зоны, значение - тип диска).
    * Пример:
      ```yaml
      ru-1a: fast-ru-1a
      ru-1b: fast-ru-1b
      ```
      В случае если значение указанное в `replicas` превышает количество элементов в словаре, то мастер ноды, чей номер превышает
      длину словаря получают значения заново начиная с начала словаря. Если для словаря из примера указанно `replicas: 5`, то с типом
      диска `ru-1a` будут master-0, master-2, master-4, а с типом диска `ru-1b` будут master-1, master-3.
* `nodeGroups` — массив дополнительных NG для создания статичных узлов (например, для выделенных фронтов или шлюзов). Настройки NG:
  * `name` — имя NG, будет использоваться для генерации имени нод.
  * `replicas` — сколько узлов создать.
  * `instanceClass` — частичное содержимое полей [OpenStackInstanceClass](cr.html#openstackinstanceclass). Допустимые параметры:
    * `flavorName`
    * `imageName`
    * `rootDiskSize`
    * `mainNetwork`
    * `additionalSecurityGroups`
    * `additionalTags`
    * `additionalNetworks`
    * `networksWithSecurityDisabled` - в этом списке необходимо перечислить все сети из `mainNetwork` и `additionalNetworks`, в которых **НЕЛЬЗЯ** настраивать `SecurityGroups` и `AllowedAddressPairs` на портах.
      * Формат — массив строк.
    * `floatingIPPools` - список сетей, в которых заказывать Floating IP для нод.
      * Формат — массив строк.
    * `configDrive` - флаг, указывающий будет ли монтироваться на узел дополнительный диск, содержащий конфигурацию для бутстрапа ноды. Необходимо устанавливать, если в сети, указанной в качестве `mainNetwork` отключен DHCP.
      * Опциональный параметр.
      * По умолчанию `false`
  * `zones` — ограничение набора зон, в которых разрешено создавать ноды.
    * Опциональный параметр.
    * Формат — массив строк.
  * `nodeTemplate` — настройки Node-объектов в Kubernetes, которые будут добавлены после регистрации ноды.
    * `labels` — аналогично стандартному [полю](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta) `metadata.labels`
      * Пример:
        ```yaml
        labels:
          environment: production
          app: warp-drive-ai
        ```
    * `annotations` — аналогично стандартному [полю](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta) `metadata.annotations`
      * Пример:
        ```yaml
        annotations:
          ai.fleet.com/discombobulate: "true"
        ```
    * `taints` — аналогично полю `.spec.taints` из объекта [Node](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#taint-v1-core). **Внимание!** Доступны только поля `effect`, `key`, `values`.
      * Пример:

        ```yaml
        taints:
        - effect: NoExecute
          key: ship-class
          value: frigate
        ```
* `sshPublicKey` — публичный ключ для доступа на ноды.
  * Обязательный параметр.
  * Формат — строкa.
* `tags` - словарь тегов, которые будут созданы на всех ресурсах, имеющих такую возможность. Если поменять теги в рабочем кластере, то после конвержа
  необходимо пересоздать все машины, чтобы теги применились.
  * Опциональный параметр.
  * Формат — ключ-значение.
* `zones` — Глобальное ограничение набора зон, с которыми работает данный Cloud-Provider.
  * Опциональный параметр.
  * Формат — массив строк.

