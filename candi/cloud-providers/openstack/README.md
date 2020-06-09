---
title: Cloud provider - Openstack
sidebar: candi
hide_sidebar: false
---

## Поддерживаемые схемы размещения

Каждая схема размещения должна быть описана двумя объектами OpenStackClusterConfiguration и OpenStackInitConfiguration.

**`OpenStackClusterConfiguration`** - содержит в себе описание схемы размещения, набор полей зависит от выбранной схемы и описан
для каждой из них ниже.
* В поле `provider` передаются параметры подключения к api openstack, они совпадают с параметрами
передаваемыми в поле `connection` в модуле [cloud-provider-openstack](/modules/030-cloud-provider-openstack/README.md#параметры).

**`OpenStackInitConfiguration`** - содержит в себе параметры, используемые во время бутстрапа кластера.
* Поле `masterInstanceClass` определяет параметры, с которыми будет создан instance для мастера — может содержать в себе **только** параметры `flavorName`, `imageName`, `rootDiskSizeInGb` и `securityGroups`. Остальные параметры указывать **нельзя** — они будут сконфигурированы автоматически на основании выбранной
схемы размещения. Подробнее про данные параметры описано в документации модуля [cloud-provider-openstack](/modules/030-cloud-provider-openstack/README.md#openstackinstanceclass-custom-resource).

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
provider:
  ...
---
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackInitConfiguration
masterInstanceClass:
  flavorName: m1.large                                      # required
  imageName: ubuntu-18.04-cloud-amd64                       # required
  rootDiskSizeInGb: 50                                      # optional, ephemeral disks are use if not specified
  securityGroups:                                           # optional
  - sec_group_1
  - sec_group_2
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
provider:
  ...
---
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackInitConfiguration
masterInstanceClass:
  flavorName: m1.large                                      # required
  imageName: ubuntu-18.04-cloud-amd64                       # required
  rootDiskSizeInGb: 50                                      # optional, ephemeral disks are use if not specified
  securityGroups:                                           # optional
  - sec_group_1
  - sec_group_2
```

### Simple

Master нода и ноды кластера подключаются к существующей сети. Данная схема размещения может понадобиться, если необходимо
объединить кластер кубернетес с уже имеющимися виртуальными машинами.

**Внимание**
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
provider:
  ...
---
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackInitConfiguration
masterInstanceClass:
  flavorName: m1.large                                      # required
  imageName: ubuntu-18.04-cloud-amd64                       # required
  rootDiskSizeInGb: 50                                      # optional, ephemeral disks are use if not specified
  securityGroups:                                           # optional
  - sec_group_1
  - sec_group_2
```

### SimpleWithInternalNetwork

Master нода и ноды кластера подключаются к существующей сети. Данная схема размещения может понадобиться, если необходимо
объединить кластер кубернетес с уже имеющимися виртуальными машинами.

Доступ к мастеру осуществляется либо через bastion, либо на master надо вручную навешивать floating ip

![resources](https://docs.google.com/drawings/d/e/2PACX-1vQOcYZPtHBqMtlNx9PDcMrqI0WEwRssL-oXONnrOoKNaIx1fcEODo9dK2zOoF1wbKeKJlhphFTuefB-/pub?w=960&h=720) 
<!--- Исходник: https://docs.google.com/drawings/d/1H9HGOn4abpmZwIhpwwdZSSO9izvyOZakG8HpmmzZZEo/edit --->


```
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackClusterConfiguration
layout: SimpleWithInternalNetwork
simpleWithInternalNetwork:
  internalSubnetName: pivot-standard                      # required, all cluster nodes have to be in the same subnet
  podNetworkMode: DirectRoutingWithPortSecurityEnabled    # optional, by default DirectRoutingWithPortSecurityEnabled, may also be DirectRouting or VXLAN
  externalNetworkName: ext-net                            # optional, if set will be used for ordering load balancer floating ip
provider:
  ...
---
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackInitConfiguration
masterInstanceClass:
  flavorName: m1.large                                      # required
  imageName: ubuntu-18.04-cloud-amd64                       # required
  rootDiskSizeInGb: 50                                      # optional, ephemeral disks are use if not specified
  securityGroups:                                           # optional
  - sec_group_1
  - sec_group_2
```

