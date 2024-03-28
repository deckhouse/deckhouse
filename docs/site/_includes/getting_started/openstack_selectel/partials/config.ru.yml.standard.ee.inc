# [<en>] General cluster parameters.
# [<en>] https://deckhouse.io/documentation/v1/installing/configuration.html#clusterconfiguration
# [<ru>] Общие параметры кластера.
# [<ru>] https://deckhouse.ru/documentation/v1/installing/configuration.html#clusterconfiguration
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: OpenStack
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
# [<en>] Module settings for working with OpenStack.
# [<en>] https://deckhouse.io/documentation/v1/modules/030-cloud-provider-openstack/configuration.html
# [<ru>] Настройки модуля для работы с OpenStack.
# [<ru>] https://deckhouse.ru/documentation/v1/modules/030-cloud-provider-openstack/configuration.html
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-openstack
spec:
  version: 1
  enabled: true
  settings:
    # [<en>] Default Storage Class.
    # [<ru>] Storage Class по умолчанию.
    storageClass:
      default: fast.ru-3a
---
# [<en>] Cloud provider settings.
# [<en>] https://deckhouse.io/documentation/v1/modules/030-cloud-provider-openstack/cluster_configuration.html
# [<ru>] Настройки облачного провайдера.
# [<ru>] https://deckhouse.io/documentation/v1/modules/030-cloud-provider-openstack/cluster_configuration.html
apiVersion: deckhouse.io/v1
kind: OpenStackClusterConfiguration
layout: Standard
standard:
  # [<en>] Clusters in Selectel VPC should be installed with a bastion host because Security Groups are not supported.
  # [<en>] Otherwise ports on master nodes will be exposed to WAN.
  # [<ru>] Кластера в Selectel VPC должны устанавливаться с bastion-хостом в целях безопасности, из-за отсутствия поддержки Security Groups.
  # [<ru>] В противном случае в WAN будут открыты порты на master-узлах.
  bastion:
    zone: ru-3a
    volumeType: fast.ru-3a
    instanceClass:
      flavorName: c2m2-d0
      imageName: ubuntu-22-04
      rootDiskSize: 20
  # [<en>] Network name for external communication. Default is external-network
  # [<ru>] Имя сети для внешнего взаимодействия. Значение по умолчанию: external-network
  externalNetworkName: external-network
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
  # [<en>] SecurityGroups are not supported by Selectel VPC.
  # [<ru>] SecurityGroups не поддерживаются в Selectel VPC.
  internalNetworkSecurity: false
# [<en>] OpenStack API access parameters
# [<ru>] параметры доступа к OpenStack API
provider:
  authURL: 'https://api.selvpc.ru/identity/v3'
  # [<en>] OS_USER_DOMAIN_NAME variable from the openrc file.
  # [<ru>] Переменная OS_USER_DOMAIN_NAME из openrc-файла.
  domainName: *!CHANGE_DOMAINNAME*
  password: *!CHANGE_PASSWORD*
  # [<en>] You might consider changing this.
  # [<ru>] Возможно, захотите изменить.
  region: ru-3
  # [<en>] You might need to specify tenantName instead of tenantID.
  # [<en>] https://deckhouse.io/documentation/v1/modules/030-cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration-provider-tenantname
  # [<ru>] Возможно вам нужно указать tenantName вместо tenantID.
  # [<ru>] https://deckhouse.ru/documentation/v1/modules/030-cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration-provider-tenantname
  # tenantName: PROJECT_NAME
  tenantID: *!CHANGE_PROJECT_ID*
  # [<en>] OS_USERNAME variable from the openrc file.
  # [<ru>] Переменная OS_USERNAME из openrc-файла.
  username: *!CHANGE_USERNAME*
masterNodeGroup:
  replicas: 1
  # [<en>] Disk type array.
  # [<ru>] Массив используемых типов дисков.
  volumeTypeMap:
    # [<en>] <availability zone>: <volume type>
    # [<ru>] <availability zone>: <volume type>
    # [<en>] You might consider changing this.
    # [<ru>] Возможно, захотите изменить.
    ru-3a: "fast.ru-3a"
  instanceClass:
    # [<en>] Flavor in use.
    # [<ru>] Используемый flavor.
    # [<en>] You might consider changing this.
    # [<ru>] Возможно, захотите изменить.
    flavorName: m1.large
    # [<en>] VM image in use.
    # [<ru>] Используемый образ виртуальной машины.
    # [<en>] You might consider changing this.
    # [<ru>] Возможно, захотите изменить.
    # [<en>] Don't use images provided by Selectel. Upload and use cloud images provided, by distribution maintainers.
    # [<ru>] Не используйте образы, предоставленные Selectel. Загружайте и используйте образы для облачных сред, предоставленные разработчиками дистрибутива.
    imageName: ubuntu-22.04
    # [<en>] Disk size for the root FS.
    # [<ru>] Размер диска для корневой ФС.
    rootDiskSize: 40
# [<en>] Public SSH key for accessing cloud nodes.
# [<en>] This key will be added to the user on created nodes (the user name depends on the image used).
# [<ru>] Публичная часть SSH-ключа для доступа к узлам облака.
# [<ru>] Этот ключ будет добавлен пользователю на созданных узлах (имя пользователя зависит от используемого образа).
sshPublicKey: *!CHANGE_SSH_KEY*
