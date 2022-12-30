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
clusterType: Static
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
    # [<en>] enable cni-flannel module
    # [<ru>] включить модуль cni-flannel
    cniFlannelEnabled: true
    # [<en>] cni-flannel module settings
    # [<ru>] настройки модуля cni-flannel
    cniFlannel:
      # [<en>] flannel backend, available values are VXLAN (if your servers have L3 connectivity) and HostGW (for L2 networks)
      # [<ru>] режим работы flannel, допустимые значения VXLAN (если ваши сервера имеют связность L3) или HostGW (для L2-сетей)
      # [<en>] you might consider changing this
      # [<ru>] возможно, захотите изменить
      podNetworkMode: VXLAN
---
# [<en>] section with the parameters of the bare metal cluster (StaticClusterConfiguration)
# [<en>] version of the Deckhouse API
# [<ru>] секция с параметрами bare metal-кластера (StaticClusterConfiguration)
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
# [<en>] type of the configuration section
# [<ru>] тип секции конфигурации
kind: StaticClusterConfiguration
# [<en>] list of internal cluster networks (e.g., '10.0.4.0/24'), which is
# [<en>] used for linking Kubernetes components (kube-apiserver, kubelet etc.)
# [<ru>] список внутренних сетей узлов кластера (например, '10.0.4.0/24'), который
# [<ru>] используется для связи компонентов Kubernetes (kube-apiserver, kubelet...) между собой
internalNetworkCIDRs:
- *!CHANGE_internalNetworkCIDRs*
