# [<en>] General cluster parameters.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/installing/configuration.html#clusterconfiguration
# [<ru>] Секция с общими параметрами кластера.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/installing/configuration.html#clusterconfiguration
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: Dynamix
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
# [<en>] Section for bootstrapping the Deckhouse cluster.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/installing/configuration.html#initconfiguration
# [<ru>] Секция первичной инициализации кластера Deckhouse.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/installing/configuration.html#initconfiguration
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.deckhouse.io/deckhouse/<REVISION>
  # [<en>] A special string with your token to access Docker registry (generated automatically for your license token).
  # [<ru>] Строка с ключом для доступа к Docker registry (сгенерировано автоматически для вашего токена доступа).
  registryDockerCfg: <YOUR_ACCESS_STRING_IS_HERE>
---
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
    # [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/user-authn/configuration.html#parameters-publishapi
    # [<en>] Enabling access to the API server through Ingress.
    # [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/user-authn/configuration.html#parameters-publishapi
    publishAPI:
      enabled: true
      https:
        mode: Global
        global:
          kubeconfigGeneratorMasterCA: ""
---
# [<en>] Cloud provider settings.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/cloud-provider-openstack/cluster_configuration.html
# [<ru>] Настройки облачного провайдера..
# [<ru>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/cloud-provider-openstack/cluster_configuration.html
apiVersion: deckhouse.io/v1
kind: DynamixClusterConfiguration
layout: Standard
# [<en>] The name of the location to install the cluster.
# [<ru>] Имя места установки кластера.
location: *!CHANGE_LOCATION!*
# [<en>] Account name.
# [<ru>] Имя аккаунта.
account: *!CHANGE_ACCOUNT!*
# [<en>] Dynamix API access parameters
# [<ru>] Параметры доступа к Dynamix API
provider:
  # [<en>] The URL to the Dynamix API endpoint.
  # [<ru>] URL-адрес API Dynamix
  controllerUrl: *!CHANGE_CONTROLLER_URL*
  # [<en>] The URL to the SSO Dynamix API endpoint.
  # [<ru>] URL-адрес SSO Dynamix API
  oAuth2Url: *!CHANGE_OAUTH2_URL*
  # [<en>] The application ID.
  # [<ru>] Идентификатор приложения.
  appId: *!CHANGE_APP_ID*
  # [<en>] The application secret.
  # [<ru>] Секрет приложения.
  appSecret: *!CHANGE_APP_SECRET*
  # [<en>] You might consider changing this.
  # [<ru>] Возможно, захотите изменить.
  insecure: false
masterNodeGroup:
  replicas: 1
  instanceClass:
    # [<en>] Number of allocated CPUs for the virtual machine.
    # [<ru>] Количество выделеных CPU для виртуальной машины.
    numCPUs: 4
    # [<en>] The amount of allocated RAM for the virtual machine.
    # [<ru>] Количество выделеной оперативной памяти для виртуальной машины.
    memory: 8192
    # [<en>] Disk size for the root FS.
    # [<ru>] Размер диска для корневой ФС.
    rootDiskSizeGb: 50
    # [<en>] OS image name to use for VM deployment.
    # [<ru>] Имя образа ОС, используемое для развертывания ВМ.
    imageName: *!CHANGE_IMAGE_NAME*
    # [<en>] Storage name.
    # [<ru>] Имя хранилища.
    storageEndpoint: *!CHANGE_STORAGE_ENDPOINT*
    # [<en>] Pool name.
    # [<ru>] Имя пула хранения.
    pool: *!CHANGE_POOL*
    # [<en>] External network name.
    # [<ru>] Имя внешней сети.
    externalNetwork: *!CHANGE_EXTERNAL_NETWORK*
# [<en>] Public SSH key for accessing cloud nodes.
# [<en>] This key will be added to the user on created nodes (the user name depends on the image used).
# [<ru>] Публичная часть SSH-ключа для доступа к узлам облака.
# [<ru>] Этот ключ будет добавлен пользователю на созданных узлах (имя пользователя зависит от используемого образа).
sshPublicKey: *!CHANGE_SSH_KEY*
