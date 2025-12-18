# [<en>] General cluster parameters.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration
# [<ru>] Общие параметры кластера.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: Huaweicloud
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
  imagesRepo: registry.deckhouse.ru/deckhouse/ee
  # [<en>] A special string with your token to access Docker registry (generated automatically for your license token).
  # [<ru>] Строка с ключом для доступа к Docker registry (сгенерировано автоматически для вашего токена доступа).
  registryDockerCfg: <YOUR_ACCESS_STRING_IS_HERE>
---
# [<en>] Deckhouse module settings.
# [<en>] https://deckhouse.io/modules/002-deckhouse/configuration.html
# [<ru>] Настройки модуля deckhouse.
# [<ru>] https://deckhouse.ru/modules/002-deckhouse/configuration.html
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
# [<en>] https://deckhouse.io/modules/150-user-authn/configuration.html
# [<ru>] Настройки модуля user-authn.
# [<ru>] https://deckhouse.ru/modules/150-user-authn/configuration.html
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
    # [<ru>] https://deckhouse.ru/modules/150-user-authn/configuration.html#parameters-publishapi
    # [<en>] Enabling access to the API server through Ingress.
    # [<en>] https://deckhouse.io/modules/150-user-authn/configuration.html#parameters-publishapi
    publishAPI:
      enabled: true
      https:
        mode: Global
        global:
          kubeconfigGeneratorMasterCA: ""
---
# [<en>] Cloud provider settings.
# [<en>] https://deckhouse.io/modules/030-cloud-provider-huaweicloud/cluster_configuration.html
# [<ru>] Настройки облачного провайдера.
# [<ru>] https://deckhouse.io/modules/030-cloud-provider-huaweicloud/cluster_configuration.html
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
  # [<ru>] Включите Elastic IP для master-узлов.
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
    rootDiskSize: 50
# [<en>] Public SSH key for accessing cloud nodes.
# [<en>] This key will be added to the user on created nodes (the user name depends on the image used).
# [<ru>] Публичная часть SSH-ключа для доступа к узлам облака.
# [<ru>] Этот ключ будет добавлен пользователю на созданных узлах (имя пользователя зависит от используемого образа).
sshPublicKey: *!CHANGE_SSH_KEY*
---
# [<en>] Section containing the parameters of instance class for worker nodes.
# [<en>] https://deckhouse.io/modules/030-cloud-provider-huaweicloud/cr.html
# [<ru>] Секция, описывающая параметры инстанс-класса для узлов c компонентами, обеспечивающими рабочую нагрузку.
# [<ru>] https://deckhouse.ru/modules/030-cloud-provider-huaweicloud/cr.html
apiVersion: deckhouse.io/v1
kind: HuaweiCloudInstanceClass
metadata:
  name: worker
spec:
  # [<en>] Flavor in use for this instance class.
  # [<ru>] Используемый flavor для данного инстанс-класса.
  # [<en>] You might consider changing this.
  # [<ru>] Возможно, захотите изменить.
  flavorName: Standard-2-4-50
  # [<en>] The size of a root disk in gigabytes.
  # [<ru>] Размер root-диска. Значение указывается в гигабайтах.
  rootDiskSize: 40
  # [<en>] VM image in use.
  # [<ru>] Используемый образ виртуальной машины.
  # [<en>] You might consider changing this.
  # [<ru>] Возможно, захотите изменить.
  imageName: ubuntu-22-04-cloud-amd64
---
# [<en>] Section containing the parameters of worker node group.
# [<en>] https://deckhouse.io/modules/040-node-manager/cr.html#nodegroup
# [<ru>] Секция, описывающая параметры группы узлов c компонентами, обеспечивающими рабочую нагрузку.
# [<ru>] https://deckhouse.ru/modules/040-node-manager/cr.html#nodegroup
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  cloudInstances:
    classReference:
      kind: HuaweiCloudInstanceClass
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
    - DP1
  disruptions:
    approvalMode: Automatic
  nodeType: CloudEphemeral
---
# [<en>] Section containing the parameters of Ingress NGINX Controller.
# [<en>] https://deckhouse.io/modules/402-ingress-nginx/cr.html
# [<ru>] Секция, описывающая параметры Ingress NGINX Controller.
# [<ru>] https://deckhouse.ru/modules/402-ingress-nginx/cr.html
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
# [<en>] https://deckhouse.io/modules/140-user-authz/cr.html#clusterauthorizationrule
# [<ru>] Настройки RBAC и авторизации.
# [<ru>] https://deckhouse.ru/modules/140-user-authz/cr.html#clusterauthorizationrule
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
# [<en>] https://deckhouse.io/modules/150-user-authn/cr.html#user
# [<ru>] Данные статического пользователя.
# [<ru>] https://deckhouse.ru/modules/150-user-authn/cr.html#user
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
