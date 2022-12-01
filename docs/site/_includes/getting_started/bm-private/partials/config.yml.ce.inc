# [<en>] General cluster parameters (ClusterConfiguration).
# [<en>] Version of the Deckhouse API.
# [<ru>] Секция с общими параметрами кластера (ClusterConfiguration).
# [<ru>] Используемая версия API Deckhouse Platform.
apiVersion: deckhouse.io/v1
# [<en>] Type of the configuration section.
# [<ru>] Тип секции конфигурации.
kind: ClusterConfiguration
# [<en>] Type of the infrastructure: bare metal (Static) or Cloud (Cloud).
# [<ru>] Тип инфраструктуры: bare metal (Static) или облако (Cloud).
clusterType: Static
# [<en>] Address space of the cluster's Pods.
# [<ru>] Адресное пространство Pod’ов кластера.
podSubnetCIDR: 10.111.0.0/16
# [<en>] Address space of the cluster's services.
# [<ru>] Адресное пространство для service’ов кластера.
serviceSubnetCIDR: 10.222.0.0/16
# [<en>] Kubernetes version to install.
# [<ru>] Устанавливаемая версия Kubernetes.
kubernetesVersion: "1.23"
# [<en>] Cluster domain (used for local routing).
# [<ru>] Домен кластера.
clusterDomain: "cluster.local"
packagesProxy:
  uri: https://example.com
  username: <PROXY-USERNAME>
  password: <PROXY-PASSWORD>
---
# [<en>] Section for bootstrapping the Deckhouse cluster (InitConfiguration).
# [<en>] Version of the Deckhouse API.
# [<ru>] Секция первичной инициализации кластера Deckhouse (InitConfiguration).
# [<ru>] Используемая версия API Deckhouse.
apiVersion: deckhouse.io/v1
# [<en>] Type of the configuration section.
# [<ru>] Тип секции конфигурации.
kind: InitConfiguration
# [<en>] Deckhouse parameters.
# [<ru>] Секция с параметрами Deckhouse.
deckhouse:
  # [<en>] The release channel in use.
  # [<ru>] Используемый канал обновлений.
  releaseChannel: Stable
  configOverrides:
    global:
      modules:
        # [<en>] Template to use for system app domains within the cluster.
        # [<en>] For example, in the case of %s.example.com, Grafana will be mapped to grafana.example.com.
        # [<ru>] Шаблон, который будет использоваться для составления адресов системных приложений в кластере.
        # [<ru>] Например, Grafana для %s.example.com будет доступна на домене grafana.example.com.
        publicDomainTemplate: "%s.example.com"
        # [<en>] Proxy server settings for accessing Deckhouse modules to the Internet.
        # [<ru>] Настройки proxy-сервера для доступа в Интернет модулей Deckhouse.
        proxy:
          httpProxy: <HTTP_PROXY_ADDRESS>
          httpsProxy: <HTTPS_PROXY_ADDRESS>
          noProxy: <NO_PROXY_LIST>
    # [<en>] Enable the cni-flannel module.
    # [<ru>] Включить модуль cni-flannel.
    cniFlannelEnabled: true
    # [<en>] Cni-flannel module settings.
    # [<ru>] Настройки модуля cni-flannel.
    cniFlannel:
      # [<en>] Flannel backend, available values are VXLAN (if your servers have L3 connectivity) and HostGW (for L2 networks).
      # [<ru>] Режим работы flannel, допустимые значения VXLAN (если ваши сервера имеют связность L3) или HostGW (для L2-сетей).
      # [<en>] You might consider changing this.
      # [<ru>] Возможно, захотите изменить.
      podNetworkMode: VXLAN
  # [<en>] Address of the Docker registry where the Deckhouse images are located.
  # [<ru>] Адрес Docker registry с образами Deckhouse.
  imagesRepo: <IMAGES_REPO_URI>
  # [<en>] A special string with your token to access the Docker registry.
  # [<ru>] Строка с ключом для доступа к Docker registry.
  registryDockerCfg: <YOUR_PRIVATE_ACCESS_STRING_IS_HERE>
  # [<en>] Registry access scheme (HTTP or HTTPS).
  # [<ru>] Протокол доступа к registry (HTTP или HTTPS).
  registryScheme: HTTPS
  # [<en>] Root CA certificate to validate the registry’s HTTPS certificate (if self-signed certificates are used).
  # [<ru>] Корневой сертификат, которым можно проверить сертификат registry (если registry использует самоподписанные сертификаты).
  registryCA: <REGISTRY_CA>
---
# [<en>] Section with the parameters of the bare metal cluster (StaticClusterConfiguration).
# [<en>] Version of the Deckhouse API.
# [<ru>] Секция с параметрами bare metal кластера (StaticClusterConfiguration).
# [<ru>] Используемая версия API Deckhouse.
apiVersion: deckhouse.io/v1
# [<en>] Type of the configuration section.
# [<ru>] Тип секции конфигурации.
kind: StaticClusterConfiguration
# [<en>] List of internal cluster networks (e.g., '10.0.4.0/24')
# [<en>] for linking Kubernetes components (kube-apiserver, kubelet etc.).
# [<ru>] Список внутренних сетей узлов кластера (например, '10.0.4.0/24'), который
# [<ru>] используется для связи компонентов Kubernetes (kube-apiserver, kubelet...) между собой.
internalNetworkCIDRs:
- *!CHANGE_internalNetworkCIDRs*
