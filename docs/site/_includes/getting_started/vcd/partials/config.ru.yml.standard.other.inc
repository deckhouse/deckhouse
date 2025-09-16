# [<en>] General cluster parameters.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/installing/configuration.html#clusterconfiguration
# [<ru>] Общие параметры кластера.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/installing/configuration.html#clusterconfiguration
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
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/installing/configuration.html#initconfiguration
# [<ru>] Настройки первичной инициализации кластера Deckhouse.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/installing/configuration.html#initconfiguration
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.deckhouse.ru/deckhouse/<REVISION>
  # [<en>] A special string with your token to access Docker registry (generated automatically for your license token).
  # [<ru>] Строка с ключом для доступа к Docker registry (сгенерировано автоматически для вашего токена доступа).
  registryDockerCfg: <YOUR_ACCESS_STRING_IS_HERE>
---
# [<en>] Deckhouse module settings.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/deckhouse/configuration.html
# [<ru>] Настройки модуля deckhouse.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/deckhouse/configuration.html
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
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/user-authn/configuration.html
# [<ru>] Настройки модуля user-authn.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/user-authn/configuration.html
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
# [<en>] cni-cilium module settings.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/cni-cilium/configuration.html
# [<ru>] Настройки модуля cni-cilium.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/cni-cilium/configuration.html
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cni-cilium
spec:
  enabled: true
---
# [<en>] metallb module settings.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/metallb/configuration.html
# [<ru>] Настройки модуля metallb.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/metallb/configuration.html
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: metallb
spec:
  version: 2
  enabled: true
  settings:
    # [<ru>] Настройки metallb-пула для входящего трафика.
    # [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/metallb/configuration.html#parameters-addresspools
    # [<en>] Metallb pool settings for incoming traffic.
    # [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/metallb/configuration.html#parameters-addresspools
    addressPools:
    - addresses:
        - *!CHANGE_SUBNET_METALLB_POOL*.10/32
      name: frontend-pool
      protocol: layer2
---
# [<en>] Load balancer class for Ingress resources.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/metallb/cr.html#metalloadbalancerclass
# [<ru>] Класс балансировщика нагрузки для ресурсов Ingress.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/metallb/cr.html#metalloadbalancerclass
apiVersion: network.deckhouse.io/v1alpha1
kind: MetalLoadBalancerClass
metadata:
  name: ingress
spec:
  # [<en>] IP range from the address pool.
  # [<ru>] Диапазон IP-адресов из пула.
  addressPool:
    - 192.168.207.100-192.168.207.101  # <- замените на свои свободные IP в сети frontend-узлов
  # [<en>] Set false to apply this class only when explicitly requested.
  # [<ru>] False — класс используется только при явном указании.
  isDefault: false
  # [<en>] Limit allocation to nodes with the frontend role.
  # [<ru>] Ограничить выдачу IP-адресов только узлам с ролью frontend.
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  type: L2
---
# [<en>] Cloud provider settings.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/cloud-provider-vsphere/cluster_configuration.html
# [<ru>] Настройки облачного провайдера.
# [<ru>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/cloud-provider-vsphere/cluster_configuration.html
apiVersion: deckhouse.io/v1
internalNetworkCIDR: 10.15.11.0/24
kind: VCDClusterConfiguration
layout: Standard
masterNodeGroup:
  instanceClass:
    etcdDiskSizeGb: 10
    # [<en>] List of IP addresses for control-plane node.
    # [<en>] We recommend using .2, .3 .4 addresses, for example: 10.15.11.2.
    # [<ru>] Список IP-адресов для узлов control-plane.
    # [<ru>] Рекомендуем использовать .2, .3 .4 адреса, например: 10.15.11.2.
    mainNetworkIPAddresses:
    - !CHANGE_SUBNET*.2
    rootDiskSizeGb: 50
    sizingPolicy: *!CHANGE_SIZING_POLICY*
    storageProfile: *!CHANGE_STORAGE_PROFILE*
    # [<en>] The name of the image, taking into account the vCloudDirector catalog path.
    # [<en>] Example: "catalog/ubuntu-jammy-22.04".
    # [<ru>] Имя образа, созданного с учетом каталога размещения образа в vCloudDirector.
    # [<ru>] Пример: "catalog/ubuntu-jammy-22.04".
    template: *!CHANGE_TEMPLATE_NAME*
  replicas: 1
nodeGroups:
- instanceClass:
    mainNetworkIPAddresses:
    - !CHANGE_SUBNET*.11
    rootDiskSizeGb: 50
    sizingPolicy: *!CHANGE_SIZING_POLICY*
    storageProfile: *!CHANGE_STORAGE_PROFILE*
    # [<en>] The name of the image, taking into account the vCloudDirector catalog path.
    # [<en>] Example: "catalog/ubuntu-jammy-22.04".
    # [<ru>] Имя образа, созданного с учетом каталога размещения образа в vCloudDirector.
    # [<ru>] Пример: "catalog/ubuntu-jammy-22.04".
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
  # [<ru>] Укажите true, если vCloud Director использует самоподписанный сертификат,
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
---
# [<en>] Section containing the parameters of instance class for worker nodes.
# [<en>] hhttps://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/cloud-provider-vcd/cr.html
# [<ru>] Секция, описывающая параметры инстанс-класса для узлов c компонентами, обеспечивающими рабочую нагрузку.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/cloud-provider-vcd/cr.html
apiVersion: deckhouse.io/v1
kind: VCDInstanceClass
metadata:
  name: worker
spec:
  rootDiskSizeGb: 50
  sizingPolicy: *!CHANGE_SIZING_POLICY*
  storageProfile: *!CHANGE_STORAGE_PROFILE*
  # [<en>] The name of the image, taking into account the vCloudDirector without catalog path.
  # [<en>] Example: "ubuntu-jammy-22.04".
  # [<ru>] Имя образа, созданного с без учета каталога размещения образа в vCloudDirector.
  # [<ru>] Пример: "ubuntu-jammy-22.04".
  template: *!CHANGE_TEMPLATE_NAME*
---
# [<en>] Section containing the parameters of worker node group.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/node-manager/cr.html#nodegroup
# [<ru>] Секция, описывающая параметры группы узлов c компонентами, обеспечивающими рабочую нагрузку.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/node-manager/cr.html#nodegroup
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  cloudInstances:
    classReference:
      kind: VCDInstanceClass
      name: worker
    maxPerZone: 2
    maxSurgePerZone: 0
    maxUnavailablePerZone: 0
    minPerZone: 1
  nodeType: CloudEphemeral
---
# [<en>] Section containing the parameters of NGINX Ingress controller.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/ingress-nginx/cr.html
# [<ru>] Секция, описывающая параметры NGINX Ingress controller.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/ingress-nginx/cr.html
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  ingressClass: nginx
  # [<en>] The way traffic goes to cluster from the outer network.
  # [<ru>] Способ поступления трафика из внешнего мира.
  inlet: LoadBalancer
  loadBalancer:
    loadBalancerClass: ingress
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
    - key: dedicated.deckhouse.io
      operator: Equal
      value: frontend
---
# [<en>] RBAC and authorization settings.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/user-authz/cr.html#clusterauthorizationrule
# [<ru>] Настройки RBAC и авторизации.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/user-authz/cr.html#clusterauthorizationrule
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
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/user-authn/cr.html#user
# [<ru>] Данные статического пользователя.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/user-authn/cr.html#user
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

