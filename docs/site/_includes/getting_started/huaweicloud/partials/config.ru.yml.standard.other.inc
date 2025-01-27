# [<en>] General cluster parameters.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/installing/configuration.html#clusterconfiguration
# [<ru>] Общие параметры кластера.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/installing/configuration.html#clusterconfiguration
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: HuaweiCloud
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
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/installing/configuration.html#initconfiguration
# [<ru>] Настройки первичной инициализации кластера Deckhouse.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/installing/configuration.html#initconfiguration
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.deckhouse.ru/deckhouse/ee
  # [<en>] A special string with your token to access Docker registry (generated automatically for your license token).
  # [<ru>] Строка с ключом для доступа к Docker registry (сгенерировано автоматически для вашего токена доступа).
  registryDockerCfg: <YOUR_ACCESS_STRING_IS_HERE>
---
# [<en>] Deckhouse module settings.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/002-deckhouse/configuration.html
# [<ru>] Настройки модуля deckhouse.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/002-deckhouse/configuration.html
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
# [<en>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/deckhouse-configure-global.html#parameters
# [<ru>] Глобальные настройки Deckhouse.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/deckhouse-configure-global.html#%D0%BF%D0%B0%D1%80%D0%B0%D0%BC%D0%B5%D1%82%D1%80%D1%8B
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
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/150-user-authn/configuration.html
# [<ru>] Настройки модуля user-authn.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/150-user-authn/configuration.html
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
    # [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/150-user-authn/configuration.html#parameters-publishapi
    # [<en>] Enabling access to the API server through Ingress.
    # [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/150-user-authn/configuration.html#parameters-publishapi
    publishAPI:
      enabled: true
      https:
        mode: Global
        global:
          kubeconfigGeneratorMasterCA: ""
---
# [<en>] Cloud provider settings.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/030-cloud-provider-huaweicloud/cluster_configuration.html
# [<ru>] Настройки облачного провайдера.
# [<ru>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/030-cloud-provider-huaweicloud/cluster_configuration.html
apiVersion: deckhouse.io/v1
kind: HuaweiCloudClusterConfiguration
layout: Standard
standard:
  # [<en>] Addressing for the internal network of the cluster nodes.
  # [<ru>] Адресация для внутренней сети узлов кластера.
  internalNetworkCIDR: 192.168.198.0/24
  # [<en>] A list of recursive DNS addresses of the internal network.
  # [<ru>] Список адресов рекурсивных DNS внутренней сети.
  # [<en>] You might consider changing this.
  # [<ru>] Возможно, захотите изменить.
  internalNetworkDNSServers:
    - 8.8.8.8
    - 8.8.4.4
  # [<en>] A flag that determines whether SecurityGroups and AllowedAddressPairs should be configured on internal network ports.
  # [<ru>] Флаг, который определяет необходимо ли настраивать SecurityGroups и AllowedAddressPairs на портах внутренней сети.
  internalNetworkSecurity: true
  # [<en>] Enable Elastic IP for the master nodes.
  # [<ru>] Включите Elastic IP для мастер узлов.
  enableEIP: true
# [<en>] HuaweiCloud API access parameters
# [<ru>] параметры доступа к HuaweiCloud API
provider:
  # [<en>] The endpoint of the cloud provider.
  # [<ru>] Конечная точка поставщика облачных услуг.
  cloud: "<Endpoint>"
  # [<en>] The Huawei Cloud region where the cluster will be deployed.
  # [<ru>] Регион Huawei Cloud, в котором будет развернут кластер.
  region: "<Region>"
  # [<en>] The Huawei Cloud access key.
  # [<ru>] Ключ доступа к облаку Huawei Cloud.
  accessKey: "<Access key>"
  # [<en>] The Huawei Cloud secret key.
  # [<ru>] Секретный ключ облака Huawei Cloud.
  secretKey: "<Secret key>"
  # [<en>] Trust self-signed SSL certificates.
  # [<ru>] Доверять самоподписанным SSL-сертификатам.
  insecure: false
  # [<en>] The identity authentication URL.
  # [<ru>] URL-адрес аутентификации личности.
  authURL: "<Auth URL>"
  # [<en>] The account name of IAM to scope to.
  # [<ru>] Имя учетной записи IAM для охвата.
  domainName: "<Domain Name>"
  # [<en>] The project ID.
  # [<ru>] Идентификатор проекта.
  projectID: "<Project ID>"
masterNodeGroup:
  replicas: 1
  # [<en>] Disk type array.
  # [<ru>] Массив используемых типов дисков.
  volumeTypeMap:
    # [<en>] <availability zone>: <volume type>
    # [<ru>] <availability zone>: <volume type>
    # [<en>] You might consider changing this.
    # [<ru>] Возможно, захотите изменить.
    DP1: dp1-high-iops
  instanceClass:
    # [<en>] Flavor in use.
    # [<ru>] Используемый flavor.
    # [<en>] You might consider changing this.
    # [<ru>] Возможно, захотите изменить.
    flavorName: Standard-2-8-50
    # [<en>] VM image in use.
    # [<ru>] Используемый образ виртуальной машины.
    # [<en>] You might consider changing this.
    # [<ru>] Возможно, захотите изменить.
    imageName: ubuntu-22-04-cloud-amd64
    # [<en>] Disk size for the root FS.
    # [<ru>] Размер диска для корневой ФС.
    rootDiskSize: 40
    # [<en>] The type of a root disk.
    # [<ru>] Тип root-диска.
    rootDiskType: SSD
# [<en>] Public SSH key for accessing cloud nodes.
# [<en>] This key will be added to the user on created nodes (the user name depends on the image used).
# [<ru>] Публичная часть SSH-ключа для доступа к узлам облака.
# [<ru>] Этот ключ будет добавлен пользователю на созданных узлах (имя пользователя зависит от используемого образа).
sshPublicKey: *!CHANGE_SSH_KEY*
