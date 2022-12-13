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
  prefix: cloud-demo
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
clusterDomain: cluster.local
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
layout: Standard
# [<en>] public SSH key for accessing cloud nodes
# [<ru>] публичная часть SSH-ключа для доступа к узлам облака
sshPublicKey: <SSH_PUBLIC_KEY>
# [<en>] address space of the Azure cloud
# [<ru>] адресное пространство облака внутри Azure
vNetCIDR: 10.241.0.0/16
# [<en>] address space of the cluster's nodes
# [<ru>] адресное пространство узлов кластера
subnetCIDR: 10.241.0.0/24
# [<en>] parameters of the master node group
# [<ru>] параметры группы master-узлов
masterNodeGroup:
  # [<en>] number of replicas
  # [<en>] if more than 1 master node exists, control-plane will be automatically deployed on all master nodes
  # [<ru>] количество реплик мастера
  # [<ru>] если будет больше одного master-узла, то control-plane на всех master-узлах будет развернут автоматическии
  replicas: 1
  # [<en>] Parameters of the VM image
  # [<ru>] параметры инстанса
  instanceClass:
    # [<en>] type of the instance
    # [<ru>] тип используемого инстанса
    # [<en>] you might consider changing this
    # [<ru>] возможно, захотите изменить
    machineSize: Standard_D4ds_v4
    # [<en>] Azure image URN
    # [<ru>] URN образа виртуальной машины в Azure
    # [<en>] you might consider changing this
    # [<ru>] возможно, захотите изменить
    urn: Canonical:UbuntuServer:18.04-LTS:18.04.202010140
    # [<en>] order static public IP
    # [<ru>] заказать статический публичный IP
    enableExternalIP: true
provider:
  # [<en>] subscription ID
  # [<ru>] идентификатор подписки
  subscriptionId: *!CHANGE_ME*
  # [<en>] application (client) ID
  # [<ru>] идентификатор приложения
  clientId: *!CHANGE_ME*
  # [<en>] application (client) secret
  # [<ru>] секрет приложения
  clientSecret: *!CHANGE_ME*
  # [<en>] directory (tenant) ID
  # [<ru>] идентификатор каталога (клиента)
  tenantId: *!CHANGE_ME*
  # [<en>] cluster region
  # [<ru>] регион привязки кластера
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  location: westeurope
