# [<en>] General cluster parameters.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration
# [<ru>] Общие параметры кластера.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration
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
# [<en>] You might consider changing this. Ensure that there are no overlaps with serviceSubnetCIDR.
# [<ru>] Возможно, захотите изменить. Убедитесь, что не будет пересечений с serviceSubnetCIDR.
podSubnetCIDR: 10.111.0.0/16
# [<en>] Address space of the cluster's services.
# [<ru>] Адресное пространство для service’ов кластера.
# [<en>] You might consider changing this. Ensure that there are no overlaps with podSubnetCIDR.
# [<ru>] Возможно, захотите изменить. Убедитесь, что не будет пересечений с podSubnetCIDR.
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: "cluster.local"
---
# [<en>] Settings for the bootstrapping the Deckhouse cluster
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration
# [<ru>] Настройки первичной инициализации кластера Deckhouse.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.deckhouse.io/deckhouse/<REVISION>
  # [<en>] A special string with your token to access Docker registry (generated automatically for your license token).
  # [<ru>] Строка с ключом для доступа к Docker registry (сгенерировано автоматически для вашего токена доступа).
  registryDockerCfg: <YOUR_ACCESS_STRING_IS_HERE>
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
      dexCAMode: DoNotNeed
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
# [<en>] Module settings for working with OpenStack.
# [<en>] https://deckhouse.io/modules/cloud-provider-openstack/configuration.html
# [<ru>] Настройки модуля для работы с OpenStack.
# [<ru>] https://deckhouse.ru/modules/cloud-provider-openstack/configuration.html
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
# [<en>] https://deckhouse.io/modules/cloud-provider-openstack/cluster_configuration.html
# [<ru>] Настройки облачного провайдера.
# [<ru>] https://deckhouse.io/modules/cloud-provider-openstack/cluster_configuration.html
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
  internalNetworkSecurity: true
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
  # [<en>] https://deckhouse.io/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration-provider-tenantname
  # [<ru>] Возможно вам нужно указать tenantName вместо tenantID.
  # [<ru>] https://deckhouse.ru/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration-provider-tenantname
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
    # [<en>] Don't use images provided by Selectel. Upload and use cloud images, provided by distribution maintainers.
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
---
# [<en>] Section containing the parameters of instance class for worker nodes.
# [<en>] https://deckhouse.io/modules/cloud-provider-openstack/cr.html
# [<ru>] Секция, описывающая параметры инстанс-класса для узлов c компонентами, обеспечивающими рабочую нагрузку.
# [<ru>] https://deckhouse.ru/modules/cloud-provider-openstack/cr.html
apiVersion: deckhouse.io/v1
kind: OpenStackInstanceClass
metadata:
  name: worker
spec:
  # [<en>] Flavor in use for this instance class.
  # [<ru>] Используемый flavor для данного инстанс-класса.
  # [<en>] You might consider changing this.
  # [<ru>] Возможно, захотите изменить.
  flavorName: m1.large
  rootDiskSize: 30
  # [<en>] VM image in use.
  # [<ru>] Используемый образ виртуальной машины.
  # [<en>] You might consider changing this.
  # [<ru>] Возможно, захотите изменить.
  imageName: ubuntu-22.04
---
# [<en>] Section containing the parameters of worker node group.
# [<en>] https://deckhouse.io/modules/node-manager/cr.html#nodegroup
# [<ru>] Секция, описывающая параметры группы узлов c компонентами, обеспечивающими рабочую нагрузку.
# [<ru>] https://deckhouse.ru/modules/node-manager/cr.html#nodegroup
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  cloudInstances:
    classReference:
      kind: OpenStackInstanceClass
      name: worker
    # [<en>] The maximum number of instances for the group in each zone (used by the autoscaler).
    # [<en>] You might consider changing this.
    # [<ru>] Максимальное количество инстансов в каждой зоне (используется при масштабировании).
    # [<ru>] Возможно, захотите изменить.
    maxPerZone: 1
    # [<en>] The minimum number of instances for the group in each zone. To launch more nodes, increase maxPerZone or add more zones.
    # [<ru>] Минимальное количество инстансов в каждой зоне. Чтобы запустить больше узлов, увеличьте maxPerZone или добавьте зоны.
    minPerZone: 1
    # [<en>] List of availability zones to create instances in.
    # [<ru>] Список зон, в которых создаются инстансы.
    # [<en>] You might consider changing this.
    # [<ru>] Возможно, захотите изменить.
    zones:
    - ru-3a
  disruptions:
    approvalMode: Automatic
  nodeType: CloudEphemeral
---
# [<en>] Section containing the parameters of Ingress NGINX Controller.
# [<en>] https://deckhouse.io/modules/ingress-nginx/cr.html
# [<ru>] Секция, описывающая параметры Ingress NGINX Controller.
# [<ru>] https://deckhouse.ru/modules/ingress-nginx/cr.html
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  # [<en>] Describes on which nodes the Ingress Controller will be located. Label node.deckhouse.io/group: <NAME_GROUP_NAME> is set automatically.
  # [<ru>] Описывает, на каких узлах будет находиться Ingress-контроллер. Лейбл node.deckhouse.io/group: <NODE_GROUP_NAME> устанавливается автоматически.
  nodeSelector:
    node.deckhouse.io/group: worker
---
# [<en>] RBAC and authorization settings.
# [<en>] https://deckhouse.io/modules/user-authz/cr.html#clusterauthorizationrule
# [<ru>] Настройки RBAC и авторизации.
# [<ru>] https://deckhouse.ru/modules/user-authz/cr.html#clusterauthorizationrule
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin
spec:
  subjects:
  - kind: User
    name: admin@deckhouse.io
  accessLevel: SuperAdmin
  portForwarding: true
---
# [<en>] Parameters of the static user.
# [<en>] https://deckhouse.io/modules/user-authn/cr.html#user
# [<ru>] Данные статического пользователя.
# [<ru>] https://deckhouse.ru/modules/user-authn/cr.html#user
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  # [<en>] User e-mail.
  # [<ru>] E-mail пользователя.
  email: admin@deckhouse.io
  # [<en>] This is a hash of the password <GENERATED_PASSWORD>, generated when loading the page of the Getting Started.
  # [<en>] Generate your own or use it at your own risk (for testing purposes)
  # [<en>] echo -n '<GENERATED_PASSWORD>' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo
  # [<ru>] Это хэш пароля <GENERATED_PASSWORD>, сгенерированного при загрузке страницы "Быстрого Старта".
  # [<ru>] Сгенерируйте свой или используйте этот, но только для тестирования
  # [<ru>] echo -n '<GENERATED_PASSWORD>' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo
  # [<en>] You might consider changing this.
  # [<ru>] Возможно, захотите изменить.
  password: <GENERATED_PASSWORD_HASH>
