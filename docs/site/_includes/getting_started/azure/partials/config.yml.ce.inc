# [<en>] general cluster parameters (ClusterConfiguration)
# [<en>] version of the Deckhouse API
# [<ru>] секция с общими параметрами кластера (ClusterConfiguration)
# [<ru>] используемая версия API Deckhouse Platform
apiVersion: deckhouse.io/v1
# [<en>] type of the configuration section
# [<ru>] тип секции конфигурации
kind: ClusterConfiguration
# [<en>] type of the infrastructure: bare metal (Static) or Cloud (Cloud)
# [<ru>] тип инфраструктуры: bare metal (Static) или облако (Cloud)
clusterType: Cloud
# [<en>] cloud provider-related settings
# [<ru>] параметры облачного провайдера
cloud:
  # [<en>] type of the cloud provider
  # [<ru>] используемый облачный провайдер
  provider: Azure
  # [<en>] prefix to differentiate cluster objects (can be used, e.g., in routing)
  # [<ru>] префикс для объектов кластера для их отличия (используется, например, при маршрутизации)
  prefix: "cloud-demo"
# [<en>] address space of the cluster's Pods
# [<ru>] адресное пространство Pod’ов кластера
podSubnetCIDR: 10.111.0.0/16
# [<en>] address space of the cluster's services
# [<ru>] адресное пространство для service’ов кластера
serviceSubnetCIDR: 10.222.0.0/16
# [<en>] Kubernetes version to install
# [<ru>] устанавливаемая версия Kubernetes
kubernetesVersion: "1.23"
# [<en>] cluster domain (used for local routing)
# [<ru>] домен кластера
clusterDomain: "cluster.local"
---
# [<en>] section for bootstrapping the Deckhouse cluster (InitConfiguration)
# [<en>] version of the Deckhouse API
# [<ru>] секция первичной инициализации кластера Deckhouse Platform (InitConfiguration)
# [<ru>] используемая версия API Deckhouse Platform
apiVersion: deckhouse.io/v1
# [<en>] type of the configuration section
# [<ru>] тип секции конфигурации
kind: InitConfiguration
# [<en>] Deckhouse Platform parameters
# [<ru>] секция с параметрами Deckhouse Platform
deckhouse:
  # [<en>] the release channel in use
  # [<ru>] используемый канал обновлений
  releaseChannel: Stable
  configOverrides:
    global:
      modules:
        # [<en>] template that will be used for system apps domains within the cluster
        # [<en>] e.g., Grafana for %s.example.com will be available as grafana.example.com
        # [<ru>] шаблон, который будет использоваться для составления адресов системных приложений в кластере
        # [<ru>] например, Grafana для %s.example.com будет доступна на домене grafana.example.com
        publicDomainTemplate: "%s.example.com"
---
# [<en>] section containing the parameters of the cloud provider
# [<en>] version of the Deckhouse API
# [<ru>] секция, описывающая параметры облачного провайдера
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
# [<en>] type of the configuration section
# [<ru>] тип секции конфигурации
kind: AzureClusterConfiguration
# [<en>] pre-defined layout from Deckhouse
# [<ru>] layout — архитектура расположения ресурсов в облаке
layout: <layout>
# [<en>] Azure access parameters
# [<ru>] параметры доступа к облаку Azure
provider:
  # the ID of the subscription
  subscriptionId: *!CHANGE_SUBSCRIPTION_ID*
  # the client ID
  clientId: *!CHANGE_CLIENT_ID*
  # the client's secret
  clientSecret: *!CHANGE_CLIENT_SECRET*
  # the ID of the tenant
  tenantId: *!CHANGE_TENANT_ID*
  # [<en>] cluster region
  # [<ru>] регион привязки кластера
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  location: westeurope
# [<en>] parameters of the master node group
# [<ru>] параметры группы master-узлов
masterNodeGroup:
  # [<en>] number of replicas
  # [<en>] if more than 1 master node exists, control-plane will be automatically deployed on all master nodes
  # [<ru>] количество реплик мастера
  # [<ru>] если будет больше одного master-узла, то control-plane на всех master-узлах будет развернут автоматическии
  replicas: 1
  # [<en>] parameters of the VM image
  # [<ru>] параметры инстанса
  instanceClass:
    # [<en>] type of the instance
    # [<ru>] тип используемого инстанса
    instanceType: Standard_F4
    # [<en>] disk size
    # [<ru>] размер диска
    diskSizeGb: 32
    # [<en>] VM image in use
    # [<ru>] используемый образ виртуальной машины
    urn: Canonical:UbuntuServer:18.04-LTS:18.04.202010140
    # [<en>] enabling assigning external IP addresses to the cluster
    # [<ru>] включать ли назначение внешнего IP-адреса для кластера
    enableExternalIP: true
# [<en>] public SSH key for accessing cloud nodes
# [<ru>] публичная часть SSH-ключа для доступа к узлам облака
sshPublicKey: <SSH_PUBLIC_KEY>
# [<en>] address space of the cluster virtual network
# [<ru>] адресное пространство виртуальной сети кластера
vNetCIDR: 10.50.0.0/16
# [<en>] a subnet from the `vNetCIDR` address space for cluster nodes
# [<ru>] адресное пространство подсети кластера из пространства `vNetCIDR`
subnetCIDR: 10.50.0.0/24
