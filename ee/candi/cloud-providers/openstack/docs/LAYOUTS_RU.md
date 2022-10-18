---
title: "Cloud provider — OpenStack: схемы размещения"
---

Поддерживаются четыре схемы размещения. Ниже подробнее о каждой их них.

## Standard

Создаётся внутренняя сеть кластера со шлюзом в публичную сеть, узлы не имеют публичных IP-адресов. Для master-узла заказывается плавающий IP-адрес.

> **Внимание!**
> Если провайдер не поддерживает SecurityGroups, то все приложения, запущенные на узлах с Floating IP, будут доступны по белому IP-адресу.
> Например, `kube-apiserver` на master-узлах будет доступен на порту 6443. Чтобы избежать этого, рекомендуется использовать схему размещения [SimpleWithInternalNetwork](#simplewithinternalnetwork).

![resources](https://docs.google.com/drawings/d/e/2PACX-1vSTIcQnxcwHsgANqHE5Ry_ZcetYX2lTFdDjd3Kip5cteSbUxwRjR3NigwQzyTMDGX10_Avr_mizOB5o/pub?w=960&h=720)
<!--- Исходник: https://docs.google.com/drawings/d/1hjmDn2aJj3ru3kBR6Jd6MAW3NWJZMNkend_K43cMN0w/edit --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: OpenStackClusterConfiguration
layout: Standard
standard:
  internalNetworkCIDR: 192.168.199.0/24         # Обязательный параметр.
  internalNetworkDNSServers:                    # Обязательный параметр.
  - 8.8.8.8
  - 4.2.2.2
  internalNetworkSecurity: true|false           # Необязательный параметр, по умолчанию true.
  externalNetworkName: shared                   # Обязательный параметр.
  bastion:
    zone: ru2-b                                 # Необязательный параметр.
    volumeType: fast-ru-2b                      # Необязательный параметр.
    instanceClass:
      flavorName: m1.large                      # Обязательный параметр.
      imageName: ubuntu-20-04-cloud-amd64       # Обязательный параметр.
      # Необязательный параметр. Если не указан — используется локальный диск..
      rootDiskSize: 50
      additionalTags:
        severity: critical                      # Необязательный параметр.
        environment: production                 # Необязательный параметр.
masterNodeGroup:
  replicas: 3
  instanceClass:
    flavorName: m1.large                        # Обязательный параметр.
    imageName: ubuntu-18-04-cloud-amd64         # Обязательный параметр.
    # Необязательный параметр. Если не указан — используется локальный диск.
    rootDiskSize: 50
    # Необязательный параметр, дополнительные группы безопасности.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
    additionalTags:
      severity: critical
      environment: production
  # Обязательный параметр. Карта типов томов для сертификатов etcd и Kubernetes
  # (всегда используйте самый быстрый диск, предоставленный поставщиком).
  volumeTypeMap:
    # Если указан rootDiskSize, то этот тип тома также будет
    # использоваться для главного корневого тома.
    ru-1a: fast-ru-1a
    ru-1b: fast-ru-1b
    ru-1c: fast-ru-1c
nodeGroups:
- name: front
  replicas: 2
  instanceClass:
    flavorName: m1.small                        # Обязательный параметр.
    imageName: ubuntu-18-04-cloud-amd64         # Обязательный параметр.
    # Необязательный параметр, Если не указан — используется локальный диск.
    rootDiskSize: 20
    # Необязательный параметр, по умолчанию false. Определяет, требуется ли
    # конфигурационный диск во время процесса начальной загрузки виртуальной
    # машины. Это необходимо, если в сети нет DHCP, который используется в
    # качестве шлюза по умолчанию.
    configDrive: false
    # Обязательный параметр, сеть будет использована как шлюз по-умолчанию.
    mainNetwork: kube
    additionalNetworks:                         # Необязательный параметр.
    - office
    - shared
    # Необязательный параметр, если существуют сети с отключенной защитой
    # портов, необходимо указать их имена.
    networksWithSecurityDisabled:
    - office
    # Необязательный параметр, список сетевых пулов, в которых можно заказать
    # плавающие IP-адреса.
    floatingIPPools:
    - public
    - shared
    # Необязательный параметр, дополнительные группы безопасности.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
  zones:
  - ru-1a
  - ru-1b
sshPublicKey: "<SSH_PUBLIC_KEY>"
tags:
  project: cms
  owner: default
provider:
  ...
```

## StandardWithNoRouter

Создаётся внутренняя сеть кластера без доступа в публичную сеть. Все узлы, включая master-узел, создаются с двумя интерфейсами:
один — в публичную сеть, другой — во внутреннюю сеть. Данная схема размещения должна использоваться, если необходимо, чтобы
все узлы кластера были доступны напрямую.

> **Внимание!**
> В данной конфигурации не поддерживается LoadBalancer. Это связано с тем, что в OpenStack нельзя заказать Floating IP для
сети без роутера, соответственно нельзя заказать балансировщик с Floating IP. Если заказывать internal loadbalancer, у которого
virtual IP создаётся в публичной сети, то он всё равно доступен только с узлов кластера.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vR9Vlk22tZKpHgjOeQO2l-P0hyAZiwxU6NYGaLUsnv-OH0so8UXNnvrkNNiAROMHVI9iBsaZpfkY-kh/pub?w=960&h=720)
<!--- Исходник: https://docs.google.com/drawings/d/1gkuJhyGza0bXB2lcjdsQewWLEUCjqvTkkba-c5LtS_E/edit --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: OpenStackClusterConfiguration
layout: StandardWithNoRouter
standardWithNoRouter:
  internalNetworkCIDR: 192.168.199.0/24         # Обязательный параметр.
  externalNetworkName: ext-net                  # Обязательный параметр.
  # Необязательный параметр, включен ли DHCP в указанной внешней сети
  # (значение по умолчанию true).
  externalNetworkDHCP: false
  # Необязательный параметр, по умолчанию true.
  internalNetworkSecurity: true|false
masterNodeGroup:
  replicas: 3
  instanceClass:
    flavorName: m1.large                        # Обязательный параметр.
    imageName: ubuntu-18-04-cloud-amd64         # Обязательный параметр.
    # Необязательный параметр. Если не указан — используется локальный диск.
    rootDiskSize: 50
    # Необязательный параметр, дополнительные группы безопасности.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
  # Обязательный параметр. Карта типов томов для сертификатов etcd и Kubernetes
  # (всегда используйте самый быстрый диск, предоставляемый поставщиком).
  volumeTypeMap:
    # Если указан rootDiskSize, то этот тип тома также будет
    # использоваться для главного корневого тома.
    nova: ceph-ssd
nodeGroups:
- name: front
  replicas: 2
  instanceClass:
    flavorName: m1.small                         # Обязательный параметр.
    imageName: ubuntu-18-04-cloud-amd64          # Обязательный параметр.
    # Необязательный параметр. Если не указан — используется локальный диск.
    rootDiskSize: 20
    # Необязательный параметр, по умолчанию false. Определяет, требуется ли
    # конфигурационный диск во время процесса начальной загрузки виртуальной
    # машины. Это необходимо, если в сети нет DHCP, который используется в
    # качестве шлюза по умолчанию.
    configDrive: false
    # Обязательный параметр, сеть будет использована как шлюз по-умолчанию.
    mainNetwork: kube
    additionalNetworks:                           # Необязательный параметр.
    - office
    - shared
    # Необязательный параметр. Если существуют сети с отключенной защитой
    # портов, необходимо указать их имена.
    networksWithSecurityDisabled:
    - office
    # Необязательный параметр. Список сетевых пулов, в которых можно заказать
    # плавающие IP-адреса.
    floatingIPPools:
    - public
    - shared
    # Необязательный параметр, дополнительные группы безопасности.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
sshPublicKey: "<SSH_PUBLIC_KEY>"
provider:
  ...
```

## Simple

Master-узел и узлы кластера подключаются к существующей сети. Данная схема размещения может понадобиться, если необходимо
объединить кластер Kubernetes с уже имеющимися виртуальными машинами.

> **Внимание!**
> В данной конфигурации не поддерживается LoadBalancer. Это связано с тем, что в OpenStack нельзя заказать Floating IP для
сети без роутера, соответственно нельзя заказать балансировщик с Floating IP. Если заказывать internal loadbalancer, у которого
virtual IP создаётся в публичной сети, то он всё равно доступен только с узлов кластера.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vTZbaJg7oIvoh2hkEW-DKbqeujhOiJtv_JSvfvDfXE9-mX_p6uggoY1Z9N2EAJ79c7IMfQC9ttQAmaP/pub?w=960&h=720)
<!--- Исходник: https://docs.google.com/drawings/d/1l-vKRNA1NBPIci3Ya8r4dWL5KA9my7_wheFfMR38G10/edit --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: OpenStackClusterConfiguration
layout: Simple
simple:
  externalNetworkName: ext-net                  # Обязательный параметр.
  # Необязательный параметр, по умолчанию true.
  externalNetworkDHCP: false
  # Необязательный параметр, по умолчанию VXLAN, также может быть DirectRouting
  # или DirectRoutingWithPortSecurityEnabled.
  podNetworkMode: VXLAN
masterNodeGroup:
  replicas: 3
  instanceClass:
    flavorName: m1.large                        # Обязательный параметр.
    imageName: ubuntu-18-04-cloud-amd64         # Обязательный параметр.
    # Необязательный параметр. Если не указан — используется локальный диск.
    rootDiskSize: 50
    # Необязательный параметр, дополнительные группы безопасности.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
  # Обязательный параметр. Карта типов томов для сертификатов etcd и Kubernetes
  # (всегда используйте самый быстрый диск, предоставляемый поставщиком).
  volumeTypeMap:
    # Если указан rootDiskSize, то этот тип тома также будет
    # использоваться для главного корневого тома.
    nova: ceph-ssd
nodeGroups:
- name: front
  replicas: 2
  instanceClass:
    flavorName: m1.small                        # Обязательный параметр.
    imageName: ubuntu-18-04-cloud-amd64         # Обязательный параметр.
    # Необязательный параметр. Если не указан — используется локальный диск.
    rootDiskSize: 20
    # Необязательный параметр, по умолчанию false. Определяет, требуется ли
    # конфигурационный диск во время процесса начальной загрузки виртуальной
    # машины. Это необходимо, если в сети нет DHCP, который используется в
    # качестве шлюза по умолчанию.
    configDrive: false
    # Обязательный параметр, сеть будет использована как шлюз по-умолчанию.
    mainNetwork: kube
    additionalNetworks:                         # Необязательный параметр.
    - office
    - shared
    # Необязательный параметр. Если существуют сети с отключенной защитой
    # портов, необходимо указать их имена.
    networksWithSecurityDisabled:
    - office
    # Необязательный параметр. Список сетевых пулов, в которых можно заказать
    # плавающие IP-адреса.
    floatingIPPools:
    - public
    - shared
    # Необязательный параметр, дополнительные группы безопасности.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
sshPublicKey: "<SSH_PUBLIC_KEY>"
provider:
  ...
```

## SimpleWithInternalNetwork

Master-узел и узлы кластера подключаются к существующей сети. Данная схема размещения может понадобиться, если необходимо
объединить кластер Kubernetes с уже имеющимися виртуальными машинами.

> **Внимание!**
> В данной схеме размещения не происходит управление `SecurityGroups`, а подразумевается что они были ранее созданы.
> Для настройки политик безопасности необходимо явно указывать `additionalSecurityGroups` в `OpenStackClusterConfiguration` для masterNodeGroup и других nodeGroups, и `additionalSecurityGroups` при создании `OpenStackInstanceClass` в кластере.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vQOcYZPtHBqMtlNx9PDcMrqI0WEwRssL-oXONnrOoKNaIx1fcEODo9dK2zOoF1wbKeKJlhphFTuefB-/pub?w=960&h=720)
<!--- Исходник: https://docs.google.com/drawings/d/1H9HGOn4abpmZwIhpwwdZSSO9izvyOZakG8HpmmzZZEo/edit --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: OpenStackClusterConfiguration
layout: SimpleWithInternalNetwork
simpleWithInternalNetwork:
  # Обязательный параметр, все узлы кластера должны находиться в одной подсети.
  internalSubnetName: pivot-standard
  # Необязательный параметр, по умолчанию DirectRoutingWithPortSecurityEnabled,
  # также может быть DirectRouting или VXLAN.
  podNetworkMode: DirectRoutingWithPortSecurityEnabled
  # Необязательный параметр. Если задан, будет использоваться для конфигурации
  # балансировщика нагрузки по умолчанию и для главного плавающего IP-адреса.
  externalNetworkName: ext-net
  # Необязательный параметр, по умолчанию true.
  masterWithExternalFloatingIP: false
masterNodeGroup:
  replicas: 3
  instanceClass:
    flavorName: m1.large                        # Обязательный параметр.
    imageName: ubuntu-18-04-cloud-amd64         # Обязательный параметр.
    # Необязательный параметр. Если не указан — используется локальный диск.
    rootDiskSize: 50
    # Необязательный параметр, дополнительные группы безопасности.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
  # Обязательный параметр. Карта типов томов для сертификатов etcd и Kubernetes
  # (всегда используйте самый быстрый диск, предоставляемый поставщиком).
  volumeTypeMap:
    # Если указан rootDiskSize, то этот тип тома также будет
    # использоваться для главного корневого тома.
    nova: ceph-ssd
nodeGroups:
- name: front
  replicas: 2
  instanceClass:
    flavorName: m1.small                        # Обязательный параметр.
    imageName: ubuntu-18-04-cloud-amd64         # Обязательный параметр
    # Необязательный параметр. Если не указан — используется локальный диск.
    rootDiskSize: 20
    # Необязательный параметр, по умолчанию false. Определяет, требуется ли
    # конфигурационный диск во время процесса начальной загрузки виртуальной
    # машины. Это необходимо, если в сети нет DHCP, который используется в
    # качестве шлюза по умолчанию.
    configDrive: false
    # Обязательный параметр. Сеть будет использована как шлюз по-умолчанию.
    mainNetwork: kube
    additionalNetworks:                         # Необязательный параметр.
    - office
    - shared
    # Необязательный параметр. Если существуют сети с отключенной защитой
    # портов, необходимо указать их имена.
    networksWithSecurityDisabled:
    - office
    # Необязательный параметр. Список сетевых пулов, в которых можно заказать
    # плавающие IP-адреса.
    floatingIPPools:
    - public
    - shared
    # Необязательный параметр, дополнительные группы безопасности.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
sshPublicKey: "<SSH_PUBLIC_KEY>"
provider:
  ...
```
