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
podSubnetCIDR: 10.115.0.0/16
# [<en>] Address space of the cluster's services.
# [<ru>] Адресное пространство сети сервисов кластера.
# [<en>] You might consider changing this. Ensure that there are no overlaps with podSubnetCIDR and internalNetworkCIDRs.
# [<ru>] Возможно, захотите изменить. Убедитесь, что не будет пересечений с podSubnetCIDR и internalNetworkCIDRs.
serviceSubnetCIDR: 10.225.0.0/16
kubernetesVersion: "Automatic"
# [<en>] Cluster domain (used for local routing).
# [<ru>] Домен кластера.
clusterDomain: "internal.cluster.local"
# [<en>] The container runtime type that used on cluster nodes (NodeGroups) by default.
# [<ru>] Тип container runtime, используемый на узлах кластера (в NodeGroup’ах) по умолчанию.
defaultCRI: "ContainerdV2"
---
# [<en>] Settings for the bootstrapping the Deckhouse cluster
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration
# [<ru>] Настройки первичной инициализации кластера Deckhouse.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  # [<en>] Address of the Docker registry where the Deckhouse images are located
  # [<ru>] Адрес Docker registry с образами Deckhouse
  imagesRepo: registry.deckhouse.io/deckhouse/ee
  # [<en>] A special string with your token to access Docker registry (generated automatically for your license token)
  # [<ru>] Строка с ключом для доступа к Docker registry (сгенерировано автоматически для вашего токена доступа)
  registryDockerCfg: <YOUR_ACCESS_STRING_IS_HERE>
---
# [<en>] Deckhouse module settings.
# [<en>] https://deckhouse.io/products/virtualization-platform/documentation/v1/modules/deckhouse/configuration.html
# [<ru>] Настройки модуля deckhouse.
# [<ru>] https://deckhouse.ru/products/virtualization-platform/documentation/v1/modules/deckhouse/configuration.html
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
# [<en>] https://deckhouse.io/products/virtualization-platform/documentation/v1/reference/api/global.html#parameters
# [<ru>] Глобальные настройки Deckhouse.
# [<ru>] https://deckhouse.ru/products/virtualization-platform/documentation/v1/reference/api/global.html#%D0%BF%D0%B0%D1%80%D0%B0%D0%BC%D0%B5%D1%82%D1%80%D1%8B
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 2
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
    # [<en>] Default StorageClass for the cluster.
    # [<ru>] StorageClass по умолчанию для кластера.
    defaultClusterStorageClass: nfs-storage-class
---
# [<en>] user-authn module settings.
# [<en>] https://deckhouse.io/products/virtualization-platform/documentation/v1/modules/user-authn/configuration.html
# [<ru>] Настройки модуля user-authn.
# [<ru>] https://deckhouse.ru/products/virtualization-platform/documentation/v1/modules/user-authn/configuration.html
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 2
  enabled: true
  settings:
    controlPlaneConfigurator:
      dexCAMode: DoNotNeed
    # [<ru>] Включение доступа к API-серверу Kubernetes через Ingress.
    # [<ru>] https://deckhouse.ru/products/virtualization-platform/documentation/v1/modules/user-authn/configuration.html#parameters-publishapi
    # [<en>] Enabling access to the API server through Ingress.
    # [<en>] https://deckhouse.io/products/virtualization-platform/documentation/v1/modules/user-authn/configuration.html#parameters-publishapi
    publishAPI:
      enabled: true
      https:
        mode: Global
        global:
          kubeconfigGeneratorMasterCA: ""
---
# [<en>] cni-cilium module settings.
# [<en>] https://deckhouse.io/modules/cni-cilium/
# [<ru>] Настройки модуля cni-cilium.
# [<ru>] https://deckhouse.io/modules/cni-cilium/
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
    tunnelMode: VXLAN
---
# [<en>] admission-policy-engine module settings
# [<en>] https://deckhouse.io/modules/admission-policy-engine/
# [<ru>] Настройки модуля admission-policy-engine
# [<ru>] https://deckhouse.io/modules/admission-policy-engine/
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: admission-policy-engine
spec:
  enabled: true
  version: 1
---
# [<en>] multitenancy-manager module settings
# [<en>] https://deckhouse.io/modules/multitenancy-manager/configuration.html
# [<ru>] Настройки модуля multitenancy-manager
# [<en>] https://deckhouse.io/modules/multitenancy-manager/configuration.html
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: multitenancy-manager
spec:
  enabled: true
  version: 1
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
- <INTERNAL_NETWORK_CIDRS>
---
# [<en>] csi-nfs module settings.
# [<en>] https://deckhouse.io/modules/csi-nfs/configuration.html
# [<ru>] Настройки модуля csi-nfs.
# [<ru>] https://deckhouse.ru/modules/csi-nfs/configuration.html
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-nfs
spec:
  enabled: true
  version: 1
---
# [<en>] virtualization module settings.
# [<en>] https://deckhouse.io/modules/virtualization/configuration.html
# [<ru>] Настройки модуля virtualization.
# [<ru>] https://deckhouse.ru/modules/virtualization/configuration.html
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  enabled: true
  version: 1
  settings:
    dvcr:
      storage:
        persistentVolumeClaim:
          size: 50G
        type: PersistentVolumeClaim
    # [<en>] Subnets from which IP addresses will be assigned to virtual machines.
    # [<en>] These subnets MUST NOT overlap with node subnets, service subnets, or pod subnets.
    # [<en>] Replace with your own subnets if needed.
    # [<ru>] Подсети, из которых будут назначаться IP-адреса виртуальным машинам.
    # [<ru>] Эти подсети НЕ ДОЛЖНЫ пересекаться с подсетями узлов, сервисов или подов.
    # [<ru>] Замените на свои подсети при необходимости.
    virtualMachineCIDRs:
    - 10.66.10.0/24
    - 10.66.20.0/24
    - 10.66.30.0/24
---
# [<en>] NFS StorageClass configuration.
# [<en>] https://deckhouse.io/modules/csi-nfs/stable/cr.html#nfsstorageclass
# [<ru>] Конфигурация NFS StorageClass.
# [<ru>] https://deckhouse.ru/modules/csi-nfs/stable/cr.html#nfsstorageclass
apiVersion: storage.deckhouse.io/v1alpha1
kind: NFSStorageClass
metadata:
  name: nfs-storage-class
spec:
  connection:
    host: <NFS_HOST>
    share: <NFS_SHARE>
    nfsVersion: "4.1"
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
---
# [<en>] Section containing the parameters of Ingress NGINX Controller.
# [<en>] https://deckhouse.io/products/virtualization-platform/documentation/v1/modules/ingress-nginx/cr.html
# [<ru>] Секция, описывающая параметры Ingress NGINX Controller.
# [<ru>] https://deckhouse.ru/products/virtualization-platform/documentation/v1/modules/ingress-nginx/cr.html
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  ingressClass: nginx
  # [<en>] The way traffic goes to cluster from the outer network.
  # [<ru>] Способ поступления трафика из внешнего мира.
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
  # [<en>] Describes on which nodes the Ingress Controller will be located.
  # [<ru>] Описывает, на каких узлах будет находиться Ingress-контроллер.
  # [<en>] You might consider changing this.
  # [<en>] More examples here.
  # [<en>] https://deckhouse.io/products/virtualization-platform/documentation/v1/modules/ingress-nginx/examples.html
  # [<ru>] Возможно, захотите изменить.
  # [<ru>] Больше примеров здесь
  # [<ru>] https://deckhouse.ru/products/virtualization-platform/documentation/v1/modules/ingress-nginx/examples.html
  nodeSelector:
    node-role.kubernetes.io/control-plane: ""
  tolerations:
  - effect: NoSchedule
    key: node-role.kubernetes.io/control-plane
    operator: Exists
