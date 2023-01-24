# [<en>] General cluster parameters.
# [<en>] https://deckhouse.io/documentation/v1/installing/configuration.html#clusterconfiguration
# [<ru>] Секция с общими параметрами кластера.
# [<ru>] https://deckhouse.ru/documentation/v1/installing/configuration.html#clusterconfiguration
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
# [<en>] Address space of the cluster's Pods.
# [<ru>] Адресное пространство Pod’ов кластера.
podSubnetCIDR: 10.111.0.0/16
# [<en>] Address space of the cluster's services.
# [<ru>] Адресное пространство для service’ов кластера.
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.23"
clusterDomain: "cluster.local"
# [<en>] Proxy server settings.
# [<ru>] Настройки proxy-сервера.
proxy:
  httpProxy: <HTTP_PROXY_ADDRESS>
  httpsProxy: <HTTPS_PROXY_ADDRESS>
  noProxy: <NO_PROXY_LIST>
---
# [<en>] Section for bootstrapping the Deckhouse cluster.
# [<en>] https://deckhouse.io/documentation/v1/installing/configuration.html#initconfiguration
# [<ru>] Секция первичной инициализации кластера Deckhouse.
# [<ru>] https://deckhouse.ru/documentation/v1/installing/configuration.html#initconfiguration
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  releaseChannel: Stable
  configOverrides:
    global:
      modules:
        # [<en>] Template to use for system app domains within the cluster.
        # [<en>] For example, in the case of %s.example.com, Grafana will be mapped to grafana.example.com.
        # [<ru>] Шаблон, который будет использоваться для составления адресов системных приложений в кластере.
        # [<ru>] Например, Grafana для %s.example.com будет доступна на домене grafana.example.com.
        publicDomainTemplate: "%s.example.com"
    # [<en>] Enable the cni-flannel module.
    # [<ru>] Включить модуль cni-flannel.
    # [<en>] You might consider changing this.
    # [<ru>] Возможно, захотите изменить.
    cniFlannelEnabled: true
    # [<en>] Cni-flannel module settings.
    # [<ru>] Настройки модуля cni-flannel.
    # [<en>] You might consider changing this.
    # [<ru>] Возможно, захотите изменить.
    cniFlannel:
      # [<en>] Flannel backend, available values are VXLAN (if your servers have L3 connectivity) and HostGW (for L2 networks).
      # [<ru>] Режим работы flannel, допустимые значения VXLAN (если ваши сервера имеют связность L3) или HostGW (для L2-сетей).
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
# [<en>] https://deckhouse.io/documentation/v1/installing/configuration.html#staticclusterconfiguration
# [<ru>] Секция с параметрами bare metal кластера (StaticClusterConfiguration).
# [<ru>] https://deckhouse.ru/documentation/v1/installing/configuration.html#staticclusterconfiguration
apiVersion: deckhouse.io/v1
kind: StaticClusterConfiguration
# [<en>] list of internal cluster networks (e.g., '10.0.4.0/24'), which is
# [<en>] used for linking Kubernetes components (kube-apiserver, kubelet etc.)
# [<en>] if every node in cluster has only one network interface
# [<en>] StaticClusterConfiguration resource can be skipped.
# [<ru>] список внутренних сетей узлов кластера (например, '10.0.4.0/24'), который
# [<ru>] используется для связи компонентов Kubernetes (kube-apiserver, kubelet...) между собой
# [<ru>] Если каждый узел в кластере имеет только один сетевой интерфейс,
# [<ru>] ресурс StaticClusterConfiguration можно не создавать.
internalNetworkCIDRs:
- *!CHANGE_internalNetworkCIDRs*
