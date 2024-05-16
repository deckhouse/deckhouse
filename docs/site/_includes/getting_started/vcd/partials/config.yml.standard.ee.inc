# [<en>] General cluster parameters.
# [<en>] https://deckhouse.io/documentation/v1/installing/configuration.html#clusterconfiguration
# [<ru>] Общие параметры кластера.
# [<ru>] https://deckhouse.ru/documentation/v1/installing/configuration.html#clusterconfiguration
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: VCD
  # [<en>] A prefix of objects that are created in the cloud during the installation.
  # [<en>] You might consider changing this.
  # [<ru>] Префикс объектов, создаваемых в облаке при установке.
  # [<ru>] Возможно, захотите изменить.
  prefix: cloud-demo
# [<en>] Address space of the cluster's Pods.
# [<ru>] Адресное пространство Pod’ов кластера.
podSubnetCIDR: 10.111.0.0/16
# [<en>] Address space of the cluster's services.
# [<ru>] Адресное пространство для service’ов кластера.
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: "cluster.local"
---
# [<en>] Settings for the bootstrapping the Deckhouse cluster
# [<en>] https://deckhouse.io/documentation/v1/installing/configuration.html#initconfiguration
# [<ru>] Настройки первичной инициализации кластера Deckhouse.
# [<ru>] https://deckhouse.ru/documentation/v1/installing/configuration.html#initconfiguration
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.deckhouse.ru/deckhouse/ee
  # [<en>] A special string with your token to access Docker registry (generated automatically for your license token).
  # [<ru>] Строка с ключом для доступа к Docker registry (сгенерировано автоматически для вашего токена доступа).
  registryDockerCfg: <YOUR_ACCESS_STRING_IS_HERE>
---
# [<en>] Deckhouse module settings.
# [<en>] https://deckhouse.io/documentation/v1/modules/002-deckhouse/configuration.html
# [<ru>] Настройки модуля deckhouse.
# [<ru>] https://deckhouse.ru/documentation/v1/modules/002-deckhouse/configuration.html
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
# [<en>] https://deckhouse.ru/documentation/v1/deckhouse-configure-global.html#parameters
# [<ru>] Глобальные настройки Deckhouse.
# [<ru>] https://deckhouse.ru/documentation/v1/deckhouse-configure-global.html#%D0%BF%D0%B0%D1%80%D0%B0%D0%BC%D0%B5%D1%82%D1%80%D1%8B
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
---
# [<en>] user-authn module settings.
# [<en>] https://deckhouse.io/documentation/v1/modules/150-user-authn/configuration.html
# [<ru>] Настройки модуля user-authn.
# [<ru>] https://deckhouse.ru/documentation/v1/modules/150-user-authn/configuration.html
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 1
  enabled: true
  settings:
    controlPlaneConfigurator:
      dexCAMode: DoNotNeed
    # [<ru>] Включение доступа к API-серверу Kubernetes через Ingress.
    # [<ru>] https://deckhouse.ru/documentation/v1/modules/150-user-authn/configuration.html#parameters-publishapi
    # [<en>] Enabling access to the API server through Ingress.
    # [<en>] https://deckhouse.io/documentation/v1/modules/150-user-authn/configuration.html#parameters-publishapi
    publishAPI:
      enable: true
      https:
        mode: Global
        global:
          kubeconfigGeneratorMasterCA: ""
---
# [<en>] cni-cilium module settings.
# [<en>] https://deckhouse.io/documentation/v1/modules/021-cni-cilium/configuration.html
# [<ru>] Настройки модуля cni-cilium.
# [<ru>] https://deckhouse.ru/documentation/v1/modules/021-cni-cilium/configuration.html
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cni-cilium
spec:
  enabled: true
---
# [<en>] metallb module settings.
# [<en>] https://deckhouse.io/documentation/v1/modules/380-metallb/configuration.html
# [<ru>] Настройки модуля metallb.
# [<ru>] https://deckhouse.ru/documentation/v1/modules/380-metallb/configuration.html
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: metallb
spec:
  version: 1
  enabled: true
  settings:
      # [<ru>] Настройки metaillb-пула для входящего трафика.
      # [<ru>] https://deckhouse.ru/documentation/v1/modules/380-metallb/configuration.html#parameters-addresspools
      # [<en>] Metallb pool settings for incoming traffic.
      # [<en>] https://deckhouse.io/documentation/v1/modules/380-metallb/configuration.html#parameters-addresspools
    addressPools:
    - addresses:
      - *!CHANGE_SUBNET_METALLB_POOL*.10/32
      name: frontend-pool
      protocol: layer2
    # [<ru>] Размещение спикера на frontend-узлах.
    # [<ru>] https://deckhouse.ru/documentation/v1/modules/380-metallb/configuration.html#parameters-speaker
    # [<en>] Speaker placement on frontend hosts.
    # [<en>] https://deckhouse.io/documentation/v1/modules/380-metallb/configuration.html#parameters-speaker
    speaker:
      nodeSelector:
        node-role.deckhouse.io/frontend: ""
      tolerations:
      - effect: NoExecute
        key: dedicated.deckhouse.io
        operator: Equal
        value: frontend
---
# [<en>] Cloud provider settings.
# [<en>] https://deckhouse.io/documentation/v1/modules/030-cloud-provider-vsphere/cluster_configuration.html
# [<ru>] Настройки облачного провайдера.
# [<ru>] https://deckhouse.io/documentation/v1/modules/030-cloud-provider-vsphere/cluster_configuration.html
apiVersion: deckhouse.io/v1
internalNetworkCIDR: 10.15.11.0/24
kind: VCDClusterConfiguration
layout: Standard
masterNodeGroup:
  instanceClass:
    etcdDiskSizeGb: 10
    # [<en>] List of IP addresses for control-plane node.
    # [<en>] We recommend using .2, .3 .4 addresses, for example: 10.15.11.2.
    # [<ru>] Список IP-адресов для control-plane нод.
    # [<ru>] Рекомендуем использовать .2, .3 .4 адреса, например: 10.15.11.2.
    mainNetworkIPAddresses:
    - !CHANGE_SUBNET*.2
    rootDiskSizeGb: 40
    sizingPolicy: *!CHANGE_SIZING_POLICY*
    storageProfile: *!CHANGE_STORAGE_PROFILE*
    # [<en>] The name of the image, taking into account the vCloudDirector catalog path.
    # [<en>] Example: "catalog/ubuntu-jammy-22.04".
    # [<ru>] Имя образа, созданного с учетом каталога размещения образа в vCloudDirector.
    # [<ru>] Пример: "catatlog/ubuntu-jammy-22.04".
    template: *!CHANGE_TEMPLATE_NAME*
  replicas: 1
nodeGroups:
- instanceClass:
    mainNetworkIPAddresses:
    - !CHANGE_SUBNET*.11
    rootDiskSizeGb: 40
    sizingPolicy: *!CHANGE_SIZING_POLICY*
    storageProfile: *!CHANGE_STORAGE_PROFILE*
    # [<en>] The name of the image, taking into account the vCloudDirector catalog path.
    # [<en>] Example: "catalog/ubuntu-jammy-22.04".
    # [<ru>] Имя образа, созданного с учетом каталога размещения образа в vCloudDirector.
    # [<ru>] Пример: "catatlog/ubuntu-jammy-22.04".
    template: *!CHANGE_TEMPLATE_NAME*
  name: frontend
  replicas: 1
  nodeTemplate:
    labels:
      node-role.deckhouse.io/frontend: ""
    taints:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: frontend
# [<en>] vCloud Director API access parameters
# [<ru>] параметры доступа к API vCloud Director
provider:
  server: *!CHANGE_SERVER*
  username: *!CHANGE_USERNAME*
  password: *!CHANGE_PASSWORD*
  # [<ru>] Укажите true, если vCloud Directorиспользует самоподписанный сертификат,
  # [<ru>] иначе укажите false (или удалите строку с параметром insecure).
  # [<en>] Set to true if vCloud Director has a self-signed certificate,
  # [<en>] otherwise set false (or delete the string below with the insecure parameter).
  insecure: *!CHANGE_INSECURE*
organization: *!CHANGE_ORG*
virtualApplicationName: *!CHANGE_VAPP*
virtualDataCenter: *!CHANGE__DC*
# [<en>] Внутренняя сеть узлов.
# [<ru>] Internal network name.
mainNetwork: *!CHANGE_MAIN_NETWORK*
# [<en>] Public SSH key for accessing cloud nodes.
# [<en>] This key will be added to the user on created nodes (the user name depends on the image used).
# [<ru>] Публичная часть SSH-ключа для доступа к узлам облака.
# [<ru>] Этот ключ будет добавлен пользователю на созданных узлах (имя пользователя зависит от используемого образа).
sshPublicKey: *!CHANGE_SSH_KEY*

