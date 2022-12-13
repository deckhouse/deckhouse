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
  provider: OpenStack
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
# [<ru>] секция первичной инициализации кластера Deckhouse (InitConfiguration)
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
# [<en>] type of the configuration section
# [<ru>] тип секции конфигурации
kind: InitConfiguration
# [<en>] Deckhouse parameters
# [<ru>] секция с параметрами Deckhouse
deckhouse:
  # [<en>] address of the Docker registry where the Deckhouse images are located
  # [<ru>] адрес Docker registry с образами Deckhouse
  imagesRepo: registry.deckhouse.io/deckhouse/ee
  # [<en>] a special string with your token to access Docker registry (generated automatically for your license token)
  # [<ru>] строка с ключом для доступа к Docker registry (сгенерировано автоматически для вашего токена доступа)
  registryDockerCfg: <YOUR_ACCESS_STRING_IS_HERE>
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
kind: OpenStackClusterConfiguration
# [<en>] pre-defined layout from Deckhouse
# [<ru>] layout — архитектура расположения ресурсов в облаке
layout: <layout>
# [<en>] cloud access parameters
# [<ru>] параметры доступа к облаку
provider:
  authURL: *!CHANGE_API_URL*
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  domainName: users
  password: *!CHANGE_PASSWORD*
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  region: RegionOne
  tenantID: *!CHANGE_TENANT_ID*
  username: *!CHANGE_USERNAME*
masterNodeGroup:
  # [<en>] number of replicas
  # [<en>] if more than 1 master node exists, control-plane will be automatically deployed on all master nodes
  # [<ru>] количество реплик мастера
  # [<ru>] если будет больше одного master-узла, то control-plane на всех master-узлах будет развернут автоматическии
  replicas: 1
  # [<en>] disk type
  # [<ru>] тип используемого диска
  volumeTypeMap:
    # [<en>] <availability zone>: <volume type>
    # [<ru>] <availability zone>: <volume type>
    # [<en>] you might consider changing this
    # [<ru>] возможно, захотите изменить
    DP1: dp1-high-iops
  # [<en>] Parameters of the VM image
  # [<ru>] параметры инстанса
  instanceClass:
    # [<en>] flavor in use
    # [<ru>] используемый flavor
    # [<en>] you might consider changing this
    # [<ru>] возможно, захотите изменить
    flavorName: Standard-2-4-50
    # [<en>] VM image in use
    # [<ru>] используемый образ виртуальной машины
    # [<en>] you might consider changing this
    # [<ru>] возможно, захотите изменить
    imageName: ubuntu-18-04-cloud-amd64
    # [<en>] disk size for the root FS
    # [<ru>] размер диска для корневой ФС
    rootDiskSize: 30
standard:
  # [<en>] the name assigned to the external subnetwork
  # [<ru>] назначаемое имя для внешней подсети
  externalNetworkName: ext-net
  # [<en>] address space for the internal subnetwork
  # [<ru>] адресное пространство внутренней подсети
  internalNetworkCIDR: 192.168.198.0/24
  # [<en>] assigned DNS servers
  # [<ru>] назначаемые DNS-серверы
  internalNetworkDNSServers:
    - 8.8.8.8
    - 8.8.4.4
  # [<en>] enabling security policies in the cluster's internal network
  # [<ru>] включение политик безопасности во внутренней сети кластера
  internalNetworkSecurity: true
# [<en>] address space of the cluster's nodes
# [<ru>] адресное пространство узлов кластера
sshPublicKey: <SSH_PUBLIC_KEY>
