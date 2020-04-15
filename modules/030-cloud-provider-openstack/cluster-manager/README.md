## Поддерживаемые схемы размещения

### Standard
Создаётся внутренняя сеть кластера с шлюзом в публичную сеть, ноды не имеют публичных ip адресов. Для мастера заказывается
floating ip.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vSTIcQnxcwHsgANqHE5Ry_ZcetYX2lTFdDjd3Kip5cteSbUxwRjR3NigwQzyTMDGX10_Avr_mizOB5o/pub?w=960&h=720)
<!--- Исходник: https://docs.google.com/drawings/d/1hjmDn2aJj3ru3kBR6Jd6MAW3NWJZMNkend_K43cMN0w/edit --->

```
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackClusterConfig
spec:
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
bootstrap:
  masterInstanceClass:
    flavorName: m1.large                                    # required
    imageName: ubuntu-18.04-cloud-amd64                     # required
    rootDiskSizeInGb: 50                                    # optional, ephemeral disks are use if not specified
```

### StandardWithNoRouter
Создаётся внутренняя сеть кластера без доступа в публичную сеть. Все ноды, включая мастер, созаются с двумя интерфейсами:
один в публичную сеть, другой во внутреннюю сеть. Данная схема размещения должна использоваться, если необходимо, чтобы
все ноды кластера были доступны напрямую.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vR9Vlk22tZKpHgjOeQO2l-P0hyAZiwxU6NYGaLUsnv-OH0so8UXNnvrkNNiAROMHVI9iBsaZpfkY-kh/pub?w=960&h=720)
<!--- Исходник: https://docs.google.com/drawings/d/1gkuJhyGza0bXB2lcjdsQewWLEUCjqvTkkba-c5LtS_E/edit --->

```
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackClusterConfig
spec:
  layout: StandardWithNoRouter
  standardWithNoRouter:
    internalNetworkCIDR: 192.168.199.0/24                   # required
    externalNetworkName: shared                             # required
    internalNetworkSecurity: true|false                     # optional, default true
provider:
  ...
bootstrap:
  masterInstanceClass:
    flavorName: m1.large                                    # required
    imageName: ubuntu-18.04-cloud-amd64                     # required
    rootDiskSizeInGb: 50                                    # optional, ephemeral disks are use if not specified
```

### MasterAsTheGateway

Весь входящий и исходящий трафик проходит через master ноду. Использование данной схемы размещения подразумевается в основном
для кластеров LM. Если разворачиваете в облаке mcs, то рекомендуем использовать только зону доступности DP1 и image для нод
Ubuntu-18.04-201910, на других образах есть проблемы с cloud-init.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vTtCTRm6qKmy35_smoIwnIou635Zv3zexV_leiyKE1L_4BXDljdtjv6AOXQ7T6JQVjUe4nK_hbpLtPw/pub?w=960&h=720)
<!--- Исходник: https://docs.google.com/drawings/d/1blV6gIgTLYab2XZwNObNBjKeLGQefCnEq6hMLLTI0Ik/edit --->

```
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackClusterConfig
spec:
  layout: MasterAsTheGateway
  masterAsTheGateway:
    internalNetworkCIDR: 192.168.199.0/24                   # required
    internalNetworkDNSServers:                              # required
    - 8.8.8.8
    - 4.2.2.2
    externalNetworkName: shared                             # required
provider:
  ...
bootstrap:
  masterInstanceClass:
    flavorName: m1.large                                    # required
    imageName: ubuntu-18.04-cloud-amd64                     # required
    rootDiskSizeInGb: 50                                    # optional, ephemeral disks are use if not specified
```

### Simple

Master нода и ноды кластера подключаются к существующей сети. Данная схема размещения может понадобиться, если необходимо
объединить кластер кубернетес с уже имеющимися виртуальными машинами.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vTZbaJg7oIvoh2hkEW-DKbqeujhOiJtv_JSvfvDfXE9-mX_p6uggoY1Z9N2EAJ79c7IMfQC9ttQAmaP/pub?w=960&h=720) 
<!--- Исходник: https://docs.google.com/drawings/d/1l-vKRNA1NBPIci3Ya8r4dWL5KA9my7_wheFfMR38G10/edit --->

```
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackClusterConfig
spec:
  layout: Simple
  simple:
    externalSubnetName: name-from-openstack                 # required
    podNetworkMode: VXLAN                                   # optional, by default VXLAN, may also be DirectRouting or DirectRoutinWithPortSecurityEnabled
provider:
  ...
bootstrap:
  masterInstanceClass:
    flavorName: m1.large                                    # required
    imageName: ubuntu-18.04-cloud-amd64                     # required
    rootDiskSizeInGb: 50                                    # optional, ephemeral disks are use if not specified
```

### SimpleWithInternalNetwork

Master нода и ноды кластера подключаются к существующей сети. Данная схема размещения может понадобиться, если необходимо
объединить кластер кубернетес с уже имеющимися виртуальными машинами.

Доступ к мастеру осуществляется либо через bastion, либо на master надо вручную навешивать floating ip

![resources](https://docs.google.com/drawings/d/e/2PACX-1vQOcYZPtHBqMtlNx9PDcMrqI0WEwRssL-oXONnrOoKNaIx1fcEODo9dK2zOoF1wbKeKJlhphFTuefB-/pub?w=960&h=720) 
<!--- Исходник: https://docs.google.com/drawings/d/1H9HGOn4abpmZwIhpwwdZSSO9izvyOZakG8HpmmzZZEo/edit --->


```
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackClusterConfig
spec:
  layout: SimpleWithInternalNetwork
  simpleWithInternalNetwork:
    internalSubnetName: name-from-openstack                 # required
    podNetworkMode: DirectRoutinWithPortSecurityEnabled     # optional, by default DirectRoutinWithPortSecurityEnabled, may also be DirectRouting or VXLAN
provider:
  ...
bootstrap:
  masterInstanceClass:
    flavorName: m1.large                                    # required
    imageName: ubuntu-18.04-cloud-amd64                     # required
    rootDiskSizeInGb: 50                                    # optional, ephemeral disks are use if not specified
```

