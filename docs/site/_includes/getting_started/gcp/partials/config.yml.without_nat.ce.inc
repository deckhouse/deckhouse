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
  provider: GCP
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
kind: GCPClusterConfiguration
# [<en>] pre-defined layout from Deckhouse
# [<ru>] layout — архитектура расположения ресурсов в облаке
layout: WithoutNAT
# [<en>] GCP access parameters
# [<ru>] параметры доступа к облаку GCP
provider:
  # [<en>] Example of serviceAccountJSON:
  # [<ru>] Пример заполнения serviceAccountJSON:
  # serviceAccountJSON: |
  #    {
  #      "type": "service_account",
  #      "project_id": "somproject-sandbox",
  #      "private_key_id": "***",
  #      "private_key": "***",
  #      "client_email": "mail@somemail.com",
  #      "client_id": "<client_id>",
  #      "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  #      "token_uri": "https://oauth2.googleapis.com/token",
  #      "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
  #      "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/somproject-sandbox.gserviceaccount.com"
  #    }
  serviceAccountJSON: *!CHANGE_ServiceAccountJSON*
  # [<en>] cluster region
  # [<ru>] регион привязки кластера
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  region: europe-west3
# [<en>] list of labels to attach to cluster resources.
# [<ru>] список меток ресурсов кластера, используются для идентификации в качестве префиксов ресурсов
labels:
  kube: example
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
    machineType: n1-standard-4
    # [<en>] Image id
    # [<ru>] используемый образ
    # [<en>] you might consider changing this
    # [<ru>] возможно, захотите изменить
    image: projects/ubuntu-os-cloud/global/images/ubuntu-1804-bionic-v20190911
    # [<en>] disable public IP assignement for the cluster
    # [<ru>] отключить назначение внешнего IP-адреса для кластера
    disableExternalIP: false
# [<en>] a subnet to use for cluster nodes
# [<ru>] адресное пространство узлов кластера
subnetworkCIDR: 10.0.0.0/24
# [<en>] public SSH key for accessing cloud nodes
# [<ru>] публичная часть SSH-ключа для доступа к узлам облака
sshKey: <SSH_PUBLIC_KEY>
