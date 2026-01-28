# [<en>] General cluster parameters.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration
# [<ru>] Общие параметры кластера.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
# [<en>] Address space of the cluster's Pods.
# [<ru>] Адресное пространство подов кластера.
# [<en>] You might consider changing this. Ensure that there are no overlaps with serviceSubnetCIDR and internalNetworkCIDRs.
# [<ru>] Возможно, захотите изменить. Убедитесь, что не будет пересечений с serviceSubnetCIDR и internalNetworkCIDRs.
podSubnetCIDR: 10.111.0.0/16
# [<en>] Address space of the cluster's services.
# [<ru>] Адресное пространство сети сервисов кластера.
# [<en>] You might consider changing this. Ensure that there are no overlaps with podSubnetCIDR and internalNetworkCIDRs.
# [<ru>] Возможно, захотите изменить. Убедитесь, что не будет пересечений с podSubnetCIDR и internalNetworkCIDRs.
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
# [<en>] Cluster domain (used for local routing).
# [<ru>] Домен кластера.
clusterDomain: "cluster.local"
# [<en>] The container runtime type that used on cluster nodes (NodeGroups) by default.
# [<ru>] Тип container runtime, используемый на узлах кластера (в NodeGroup’ах) по умолчанию.
defaultCRI: "ContainerdV2"
# [<en>] Proxy server settings. In the exception list (noProxy parameter), also specify the subnet of the cluster nodes.
# [<ru>] Настройки proxy-сервера. В списке исключений (параметр noProxy) также укажите подсеть узлов кластера.
proxy:
  httpProxy: <HTTP_PROXY_ADDRESS>
  httpsProxy: <HTTPS_PROXY_ADDRESS>
  noProxy: <NO_PROXY_LIST>
---
# [<en>] Settings for the bootstrapping the Deckhouse cluster
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration
# [<ru>] Настройки первичной инициализации кластера Deckhouse.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
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
# [<en>] Deckhouse module settings.
# [<en>] https://deckhouse.io/modules/deckhouse/configuration.html
# [<ru>] Настройки модуля deckhouse.
# [<ru>] https://deckhouse.ru/modules/deckhouse/configuration.html
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
  settings:
    bundle: Default
    releaseChannel: Stable
    logLevel: Info
---
# [<en>] Global Deckhouse settings.
# [<en>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters
# [<ru>] Глобальные настройки Deckhouse.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/reference/api/global.html#%D0%BF%D0%B0%D1%80%D0%B0%D0%BC%D0%B5%D1%82%D1%80%D1%8B
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
  settings:
    modules:
      # [<en>] Template that will be used for system apps domains within the cluster.
      # [<en>] E.g., Grafana for %s.example.com will be available as 'grafana.example.com'.
      # [<en>] The domain MUST NOT match the one specified in the clusterDomain parameter of the ClusterConfiguration resource.
      # [<en>] You can change it to your own or follow the steps in the guide and change it after installation.
      # [<ru>] Шаблон, который будет использоваться для составления адресов системных приложений в кластере.
      # [<ru>] Например, Grafana для %s.example.com будет доступна на домене 'grafana.example.com'.
      # [<ru>] Домен НЕ ДОЛЖЕН совпадать с указанным в параметре clusterDomain ресурса ClusterConfiguration.
      # [<ru>] Можете изменить на свой сразу, либо следовать шагам руководства и сменить его после установки.
      publicDomainTemplate: "%s.example.com"
      # [<en>] The HTTPS implementation used by the Deckhouse modules.
      # [<ru>] Способ реализации протокола HTTPS, используемый модулями Deckhouse.
      https:
        certManager:
          # [<en>] Use self-signed certificates for Deckhouse modules.
          # [<ru>] Использовать самоподписанные сертификаты для модулей Deckhouse.
          clusterIssuerName: selfsigned
---
# [<en>] user-authn module settings.
# [<en>] https://deckhouse.io/modules/user-authn/configuration.html
# [<ru>] Настройки модуля user-authn.
# [<ru>] https://deckhouse.ru/modules/user-authn/configuration.html
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 2
  enabled: true
  settings:
    controlPlaneConfigurator:
      dexCAMode: FromIngressSecret
    # [<ru>] Включение доступа к API-серверу Kubernetes через Ingress.
    # [<ru>] https://deckhouse.ru/modules/user-authn/configuration.html#parameters-publishapi
    # [<en>] Enabling access to the API server through Ingress.
    # [<en>] https://deckhouse.io/modules/user-authn/configuration.html#parameters-publishapi
    publishAPI:
      enabled: true
      https:
        mode: Global
        global:
          kubeconfigGeneratorMasterCA: ""
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cert-manager
spec:
  version: 1
  enabled: true
  settings:
    disableLetsencrypt: true
---
# [<en>] cni-cilium module settings.
# [<en>] https://deckhouse.io/modules/cni-cilium/configuration.html
# [<ru>] Настройки модуля cni-cilium.
# [<ru>] https://deckhouse.ru/modules/cni-cilium/configuration.html
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cni-cilium
spec:
  version: 1
  # [<en>] Enable cni-cilium module
  # [<ru>] Включить модуль cni-cilium
  enabled: true
  settings:
    # [<en>] cni-cilium module settings
    # [<en>] https://deckhouse.io/modules/cni-cilium/configuration.html
    # [<ru>] Настройки модуля cni-cilium
    # [<ru>] https://deckhouse.ru/modules/cni-cilium/configuration.html
    tunnelMode: VXLAN
---
# [<en>] Static cluster settings.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/reference/api/cr.html#staticclusterconfiguration
# [<ru>] Параметры статического кластера.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/reference/api/cr.html#staticclusterconfiguration
apiVersion: deckhouse.io/v1
kind: StaticClusterConfiguration
# [<en>] List of internal cluster networks (e.g., '10.0.4.0/24'), which is
# [<en>] used for linking Kubernetes components (kube-apiserver, kubelet etc.).
# [<en>] Specify, if the cluster nodes have more than one network interface or if you are using the virtualization module.
# [<en>] If only one interface is used on the nodes of the cluster, the StaticClusterConfiguration resource does not need to be created.
# [<ru>] Список внутренних сетей узлов кластера (например, '10.0.4.0/24'), который
# [<ru>] используется для связи компонентов Kubernetes (kube-apiserver, kubelet...) между собой.
# [<ru>] Укажите, если используете модуль virtualization или узлы кластера имеют более одного сетевого интерфейса.
# [<ru>] Если на узлах кластера используется только один интерфейс, ресурс StaticClusterConfiguration можно не создавать.
internalNetworkCIDRs:
- *!CHANGE_internalNetworkCIDRs*
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: Static
  nodeTemplate:
    labels:
      node-role.kubernetes.io/control-plane: ""
      node-role.kubernetes.io/master: ""
    taints: []
---
# [<en>] Stronghold module settings.
# [<en>] https://deckhouse.io/modules/stronghold/configuration.html
# [<ru>] Настройки модуля stronghold.
# [<ru>] https://deckhouse.ru/modules/stronghold/configuration.html
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: stronghold
spec:
  enabled: true
  version: 1
  settings:
    management:
      mode: Automatic
      administrators:
      - type: Group
        name: admins
