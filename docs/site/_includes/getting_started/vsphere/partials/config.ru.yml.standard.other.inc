# [<en>] General cluster parameters.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration
# [<ru>] Общие параметры кластера.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: vSphere
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
  imagesRepo: registry.deckhouse.ru/deckhouse/<REVISION>
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
# [<en>] Cloud provider settings.
# [<en>] https://deckhouse.io/modules/cloud-provider-vsphere/cluster_configuration.html
# [<ru>] Настройки облачного провайдера.
# [<ru>] https://deckhouse.io/modules/cloud-provider-vsphere/cluster_configuration.html
apiVersion: deckhouse.io/v1
kind: VsphereClusterConfiguration
layout: Standard
# [<en>] vCenter API access parameters
# [<ru>] параметры доступа к API vCenter
provider:
  server: *!CHANGE_SERVER*
  username: *!CHANGE_USERNAME*
  password: *!CHANGE_PASSWORD*
  # [<ru>] Укажите true, если vCenter использует самоподписанный сертификат,
  # [<ru>] иначе укажите false (или удалите строку с параметром insecure).
  # [<en>] Set to true if vCenter has a self-signed certificate,
  # [<en>] otherwise set false (or delete the string below with the insecure parameter).
  insecure: *!CHANGE_INSECURE*
# [<ru>] Укажите false, если vSphere DRS выключен,
# [<ru>] иначе укажите true (или удалите строку с параметром useNestedResourcePool).
# [<en>] Set to false if vSphere DRS is turned off,
# [<en>] otherwise set true (or delete the string below with the useNestedResourcePool parameter).
useNestedResourcePool: *!CHANGE_INSECURE*
# [<en>] Path to a Folder in which VirtualMachines will be created.
# [<en>] The folder itself will be created by the Deckhouse Installer.
# [<ru>] Путь до Folder, в котором будут создаваться виртуальные машины.
# [<ru>] Сам Folder будет создан Deckhouse Installer'ом.
vmFolderPath: *!CHANGE_FOLDER*
# [<en>] Region and zone tag category names.
# [<ru>] Имена категорий тегов регионов и зон.
regionTagCategory: k8s-region
zoneTagCategory: k8s-zone
# [<en>] Region and zone tag names in which cluster will operate.
# [<ru>] Имена тегов зон и регионов, в которых будет работать кластер.
region: *!CHANGE_REGION_TAG_NAME*
zones:
- *!CHANGE_ZONE_TAG_NAME*
# [<en>] Public SSH key for accessing cloud nodes.
# [<en>] This key will be added to the user on created nodes (the user name depends on the image used).
# [<ru>] Публичная часть SSH-ключа для доступа к узлам облака.
# [<ru>] Этот ключ будет добавлен пользователю на созданных узлах (имя пользователя зависит от используемого образа).
sshPublicKey: *!CHANGE_SSH_KEY*
# [<en>] Name of the External Network which has access to the Internet.
# [<en>] IP addresses from the External Network sets as ExternalIP of Node object.
# [<en>] Optional parameter.
# [<ru>] Имя External Network, у которой есть доступ в интернет.
# [<ru>] IP-адреса из External Network проставляются как ExternalIP объекта Node.
# [<ru>] Опциональный параметр.
externalNetworkNames:
- *!CHANGE_NETWORK_NAME*
# [<en>] Name of the Internal Network that will be used for traffic between nodes.
# [<en>] IP addresses from the Internal Network sets as InternalIP of Node object.
# [<en>] Optional parameter.
# [<ru>] Имя Internal Network, которая будет использоваться для трафика между узлами, в данном примере идентично externalNetworkNames.
# [<ru>] IP-адреса из Internal Network проставляются как InternalIP объекта Node.
# [<ru>] Опциональный параметр.
internalNetworkNames:
- *!CHANGE_NETWORK_NAME*
# [<en>] Address space of the cluster's nodes.
# [<ru>] Адресное пространство узлов кластера.
internalNetworkCIDR: 10.90.0.0/24
masterNodeGroup:
  replicas: 1
  instanceClass:
    numCPUs: 4
    memory: 8192
    rootDiskSize: 50
    # [<en>] The name of the image, taking into account the vCenter folder path.
    # [<en>] Example: "folder/ubuntu-jammy-22.04".
    # [<ru>] Имя образа, созданного с учетом папки размещения образа в vCenter.
    # [<ru>] Пример: "folder/ubuntu-jammy-22.04".
    template: *!CHANGE_TEMPLATE_NAME*
    datastore: *!CHANGE_DATASTORE_NAME*
    # [<en>] Main network connected to node.
    # [<ru>] Основная сеть подключенная к узлу.
    mainNetwork: *!CHANGE_NETWORK_NAME*
    # [<en>] Additional networks connected to node.
    # [<en>] Optional parameter.
    # [<ru>] Дополнительные сети, подключенные к узлу.
    # [<ru>] Опциональный параметр.
    additionalNetworks:
    - *!CHANGE_NETWORK_NAME*
---
# [<en>] Section containing the parameters of instance class for worker nodes.
# [<en>] https://deckhouse.io/modules/cloud-provider-vsphere/cr.html
# [<ru>] Секция, описывающая параметры инстанс-класса для узлов c компонентами, обеспечивающими рабочую нагрузку.
# [<ru>] https://deckhouse.ru/modules/cloud-provider-vsphere/cr.html
apiVersion: deckhouse.io/v1
kind: VsphereInstanceClass
metadata:
  name: worker
spec:
  numCPUs: 8
  memory: 16384
  # [<en>] VM disk size.
  # [<ru>] Размер диска для виртуальной машины.
  # [<en>] You might consider changing this.
  # [<ru>] Возможно, захотите изменить.
  rootDiskSize: 70
  template: *!CHANGE_TEMPLATE_NAME*
  mainNetwork: *!CHANGE_NETWORK_NAME*
  # [<en>] Specify the path to a Datastore (relative to the datacenter) in which VirtualMachines will be cloned. Example: clStorage/LUN-3.
  # [<ru>] Укажите путь до Datastore (относительно дата-центра), на котором будут созданы склонированные виртуальные машины. Пример: clStorage/LUN-3.
  datastore: *!CHANGE_PATH_TO_DATASTORE*
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
      kind: VsphereInstanceClass
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
    - *!CHANGE_ZONE_TAG_NAME*
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
  # [<en>] The way traffic goes to cluster from the outer network.
  # [<ru>] Способ поступления трафика из внешнего мира.
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
    realIPHeader: X-Forwarded-For
  nodeSelector:
    node-role.kubernetes.io/control-plane: ""
  tolerations:
  - operator: Exists
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
