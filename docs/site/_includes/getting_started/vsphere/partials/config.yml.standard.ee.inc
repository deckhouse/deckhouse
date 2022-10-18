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
  provider: vSphere
  # [<en>] prefix to differentiate cluster objects (can be used, e.g., in routing)
  # [<ru>] префикс для объектов кластера для их отличия (используется, например, при маршрутизации)
  prefix: "cloud-demo"
# [<en>] address space of the cluster's pods
# [<ru>] адресное пространство pod’ов кластера
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
kind: VsphereClusterConfiguration
# [<en>] pre-defined layout from Deckhouse
# [<ru>] layout — архитектура расположения ресурсов в облаке
layout: Standard
# [<en>] vCenter access parameters
# [<ru>] параметры доступа к vCenter
provider:
  server: *!CHANGE_SERVER*
  username: *!CHANGE_USERNAME*
  password: *!CHANGE_PASSWORD*
  # [<ru>] Укажите true, если vCenter использует самоподписанный сертификат,
  # [<ru>] иначе укажите false (или удалите строку с параметром insecure).
  # [<en>] Set to true if vCenter has a self-signed certificate,
  # [<en>] otherwise set false (or delete the string below with the insecure parameter).
  insecure: *!CHANGE_INSECURE*
# [<en>] path to a Folder in which VirtualMachines will be create
# [<en>] the folder itself will be created by the Deckhouse Installer
# [<ru>] путь до Folder, в котором будут создаваться виртуальные машины
# [<ru>] сам Folder будет создан Deckhouse Installer'ом
vmFolderPath: *!CHANGE_FOLDER*
# [<en>] region and zone tag category names
# [<ru>] имена категорий тэгов регионов и зон
regionTagCategory: k8s-region
zoneTagCategory: k8s-zone
# [<en>] region and zone tag names in which cluster will operate
# [<ru>] имена тэгов зон и регионов, в которых будет работать кластер
region: *!CHANGE_REGION_TAG_NAME*
zones:
- *!CHANGE_ZONE_TAG_NAME*
# [<en>] public SSH key for accessing cloud nodes
# [<ru>] публичная часть SSH-ключа для доступа к узлам облака
sshPublicKey: <SSH_PUBLIC_KEY>
# [<en>] name of External Network which has access to the Internet
# [<en>] ip addresses from External Network sets as ExternalIP of Node object
# [<en>] optional parameter
# [<ru>] имя External Network, у которой есть доступ в Интернет
# [<ru>] ip адреса из External Network проставляются как ExternalIP объекта Node
# [<ru>] опциональный параметр
externalNetworkNames:
- *!CHANGE_NETWORK_NAME*
# [<en>] name of Internal Network that will be used for traffic between nodes
# [<en>] ip addresses from Internal Network sets as InternalIP of Node object
# [<en>] optional parameter
# [<ru>] имя Internal Network, которая будет использоваться для трафика между узлами, в данном примере идентично externalNetworkNames
# [<ru>] ip адреса из Internal Network проставляются как InternalIP объекта Node
# [<ru>] опциональный параметр
internalNetworkNames:
- *!CHANGE_NETWORK_NAME*
# [<en>] address space of the cluster's nodes
# [<ru>] адресное пространство узлов кластера
internalNetworkCIDR: 10.90.0.0/24
masterNodeGroup:
  # [<en>] number of replicas
  # [<en>] if more than 1 master node exists, control-plane will be automatically deployed on all master nodes
  # [<ru>] количество реплик мастера
  # [<ru>] если будет больше одного master-узла, то control-plane на всех master-узлах будет развернут автоматическии
  replicas: 1
  # [<en>] Parameters of the VM image
  # [<ru>] Параметры инстанса
  instanceClass:
    numCPUs: 4
    memory: 8192
    rootDiskSize: 50
    # [<en>] The name of the image created in step 4 at the "Building a VM image" stage,
    # [<en>] taking into account the vCenter folder path. Example: "folder/my-ubuntu-packer-image".
    # [<ru>] Имя образа, созданного в шаге 4 на этапе "Сборка образа виртуальных машин",
    # [<ru>] с учетом папки размещения образа в vCenter. Пример: "folder/my-ubuntu-packer-image".
    template: *!CHANGE_TEMPLATE_NAME*
    datastore: *!CHANGE_DATASTORE_NAME*
    # [<en>] main network connected to node
    # [<ru>] основная сеть подключенная к узлу
    mainNetwork: *!CHANGE_NETWORK_NAME*
    # [<en>] additional networks connected to node
    # [<en>] optional parameter
    # [<ru>] дополнительные сети, подключенные к узлу
    # [<ru>] опциональный параметр
    additionalNetworks:
    - *!CHANGE_NETWORK_NAME*
