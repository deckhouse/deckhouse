# [<en>] General cluster parameters.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration
# [<ru>] Общие параметры кластера.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: DVP
  # [<en>] A prefix of objects that are created in the cloud during the installation.
  # [<en>] You might consider changing this.
  # [<ru>] Префикс объектов, создаваемых в облаке при установке.
  # [<ru>] Возможно, захотите изменить.
  prefix: cloud-demo
# [<en>] Address space of the cluster's Pods.
# [<en>] You might consider changing this. Must not overlap with the address space of DVP Pods and serviceSubnetCIDR.
# [<ru>] Адресное пространство подов кластера.
# [<ru>] Возможно, захотите изменить. Не должно пересекаться с адресным пространством подов DVP и serviceSubnetCIDR.
podSubnetCIDR: 10.112.0.0/16
# [<en>] Address space of the cluster's services.
# [<en>] You might consider changing this. Must not overlap with the address space of DVP services and podSubnetCIDR.
# [<ru>] Адресное пространство для сервисов кластера.
# [<ru>] Возможно, захотите изменить. Не должно пересекаться с адресным пространством сервисов DVP и podSubnetCIDR.
serviceSubnetCIDR: 10.223.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: "cluster.local"
---
# [<en>] Settings for bootstrapping the Deckhouse cluster.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration
# [<ru>] Настройки первичной инициализации кластера Deckhouse.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.deckhouse.io/deckhouse/ce
  registryDockerCfg: eyJhdXRocyI6IHsgInJlZ2lzdHJ5LmRlY2tob3VzZS5ydSI6IHt9fX0K
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
    releaseChannel: EarlyAccess
    logLevel: Info
---
# [<en>] Global Deckhouse settings.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters
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
      # [<en>] Template that will be used for system app domains within the cluster.
      # [<en>] For example, Grafana for %s.example.com will be available as 'grafana.example.com'.
      # [<en>] The domain MUST NOT match the one specified in the clusterDomain parameter of the ClusterConfiguration resource.
      # [<en>] You can change it to your own or follow the steps in the guide and change it after installation.
      # [<ru>] Шаблон, который будет использоваться для составления адресов системных приложений в кластере.
      # [<ru>] Например, Grafana для %s.example.com будет доступна на домене 'grafana.example.com'.
      # [<ru>] Домен НЕ ДОЛЖЕН совпадать с указанным в параметре clusterDomain ресурса ClusterConfiguration.
      # [<ru>] Можете изменить на свой сразу, либо следовать шагам руководства и сменить его после установки.
      publicDomainTemplate: "%s.example.com"
---
# [<en>] User-authn module settings.
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
# [<en>] https://deckhouse.io/modules/030-cloud-provider-dvp/cluster_configuration.html
# [<ru>] Настройки облачного провайдера.
# [<ru>] https://deckhouse.ru/modules/030-cloud-provider-dvp/cluster_configuration.html
apiVersion: deckhouse.io/v1
kind: DVPClusterConfiguration
layout: Standard
masterNodeGroup:
  replicas: 1
  instanceClass:
    # [<en>] Virtual machine settings for the created master node.
    # [<ru>] Настройки виртуальной машины для созданного master-узла.
    virtualMachine:
      # [<en>] CPU settings for the virtual machine.
      # [<ru>] Настройки процессора для виртуальной машины.
      cpu:
        # [<en>] Number of CPU cores for the virtual machine.
        # [<ru>] Количество ядер процессора для виртуальной машины.
        cores: 4
        # [<en>] Guaranteed share of CPU that will be allocated to the virtual machine.
        # [<ru>] Процент гарантированной доли CPU, которая будет выделена для виртуальной машины.
        coreFraction: 100%
      # [<en>] Specifies the memory settings for the virtual machine.
      # [<ru>] Определяет параметры памяти для виртуальной машины.
      memory:
        # [<en>] Amount of memory resources allowed for the virtual machine.
        # [<ru>] Количество ресурсов памяти, разрешенных для виртуальной машины.
        size: 8Gi
      # [<en>] The name of the VirtualMachineClass.
      # [<ru>] Имя VirtualMachineClass.
      virtualMachineClassName: *!CHANGE_VIRTUAL_MACHINE_CLASS_NAME*
      # [<en>] Static IP addresses (or Auto for automatic IP address assignment) to be assigned to the network interfaces of the virtual machines. The number of addresses must match the number of replicas being created — each IP address will be assigned to a specific virtual machine replica.
      # [<ru>] Статические IP-адреса (или Auto для автоматического назначения IP-адреса), назначаемые сетевым интерфейсам виртуальных машин. Количество адресов должно соответствовать числу создаваемых реплик — каждый IP-адрес будет закреплён за конкретной репликой.
      ipAddresses:
        - Auto
    # [<en>] Specifies settings for the root disk of the virtual machine.
    # [<ru>] Указывает настройки для корневого диска виртуальной машины.
    rootDisk:
      # [<en>] Root disk size.
      # [<ru>] Размер корневого диска.
      size: 50Gi
      # [<en>] Name of the existing StorageClass will be used to create the virtual machine's root disk.
      # [<ru>] Имя существующего StorageClass будет использоваться для создания корневого диска виртуальной машины.
      storageClass: *!CHANGE_STORAGE_CLASS*
      # [<en>] Image parameters that will be used to create the virtual machine's root disk.
      # [<ru>] Параметры образа, который будет использоваться для создания корневого диска виртуальной машины.
      image:
        # [<en>] Kind of the image source.
        # [<en>] You might consider changing this.
        # [<ru>] Тип источника образа.
        # [<ru>] Возможно, захотите изменить.
        kind: ClusterVirtualImage
        # [<en>] Name of the image that will be used to create the root disk.
        # [<en>] The installation requires Linux OS images with cloud-init pre-installed.
        # [<ru>] Имя образа, который будет использоваться для создания корневого диска.
        # [<ru>] Для установки требуются образы ОС Linux с предустановленным cloud-init.
        name: *!CHANGE_NAME*
    # [<en>] Settings for the etcd data disk.
    # [<ru>] Настройки для диска данных etcd.
    etcdDisk:
      # [<en>] Etcd disk size.
      # [<ru>] Размер диска etcd.
      size: 15Gi
      # [<en>] Name of the existing StorageClass will be used to create the etcd data disk.
      # [<ru>] Имя существующего StorageClass будет использоваться для создания диска данных etcd.
      storageClass: *!CHANGE_STORAGE_CLASS*
# [<en>] Public SSH key for accessing cloud nodes.
# [<en>] This key will be added to the user on created nodes (the user name depends on the image used).
# [<ru>] Публичная часть SSH-ключа для доступа к узлам облака.
# [<ru>] Этот ключ будет добавлен пользователю на созданных узлах (имя пользователя зависит от используемого образа).
sshPublicKey: *!CHANGE_SSH_KEY*
# [<en>] Contains settings to connect to the Deckhouse Kubernetes Platform API.
# [<ru>] Содержит настройки для подключения к API Deckhouse Kubernetes Platform.
provider:
  # [<en>] Kubeconfig to connect to Deckhouse Kubernetes Platform API, Base64-encoded.
  # [<ru>] Kubeconfig для подключения к API Deckhouse Kubernetes Platform в кодировке Base64.
  kubeconfigDataBase64: *!CHANGE_KUBERNETES_CONFIG_BASE64*
  # [<en>] Namespace in which DKP cluster resources will be created.
  # [<en>] You might consider changing this.
  # [<ru>] Пространство имён, в котором будут созданы ресурсы кластера DKP.
  # [<ru>] Возможно, захотите изменить.
  namespace: demo
---
# [<en>] Section containing parameters of instance class for worker nodes.
# [<en>] https://deckhouse.io/modules/030-cloud-provider-dvp/cr.html
# [<ru>] Секция, описывающая параметры инстанс-класса для узлов c компонентами, обеспечивающими рабочую нагрузку.
# [<ru>] https://deckhouse.ru/modules/030-cloud-provider-dvp/cr.html
apiVersion: deckhouse.io/v1alpha1
kind: DVPInstanceClass
metadata:
  name: worker
spec:
  # [<en>] Virtual machine settings for the created master node.
  # [<ru>] Настройки виртуальной машины для созданного master-узла.
  virtualMachine:
    # [<en>] CPU settings for the virtual machine.
    # [<ru>] Настройки процессора для виртуальной машины.
    cpu:
      # [<en>] Number of CPU cores for the virtual machine.
      # [<ru>] Количество ядер процессора для виртуальной машины.
      cores: 4
      # [<en>] Guaranteed share of CPU that will be allocated to the virtual machine.
      # [<ru>] Процент гарантированной доли CPU, которая будет выделена для виртуальной машины.
      coreFraction: 100%
    # [<en>] Specifies the memory settings for the virtual machine.
    # [<ru>] Определяет параметры памяти для виртуальной машины.
    memory:
      # [<en>] Amount of memory resources allowed for the virtual machine.
      # [<ru>] Количество ресурсов памяти, разрешенных для виртуальной машины.
      size: 8Gi
    # [<en>] The name of the VirtualMachineClass.
    # [<ru>] Имя VirtualMachineClass.
    virtualMachineClassName: *!CHANGE_VIRTUAL_MACHINE_CLASS_NAME*
    # [<en>] Defines a bootloader for the virtual machine.
    # [<en>] You might consider changing this.
    # [<ru>] Определяет загрузчик для виртуальной машины.
    # [<ru>] Возможно, захотите изменить.
    bootloader: EFI
  # [<en>] Specifies settings for the root disk of the virtual machine.
  # [<ru>] Указывает настройки для корневого диска виртуальной машины.
  rootDisk:
    # [<en>] Root disk size.
    # [<ru>] Размер корневого диска.
    size: 50Gi
    # [<en>] Name of the existing StorageClass will be used to create the virtual machine's root disk.
    # [<ru>] Имя существующего StorageClass будет использоваться для создания корневого диска виртуальной машины.
    storageClass: *!CHANGE_STORAGE_CLASS*
    # [<en>] Image parameters that will be used to create the virtual machine's root disk.
    # [<ru>] Параметры образа, который будет использоваться для создания корневого диска виртуальной машины.
    image:
      # [<en>] Kind of the image source.
      # [<en>] You might consider changing this.
      # [<ru>] Тип источника образа.
      # [<ru>] Возможно, захотите изменить.
      kind: ClusterVirtualImage
      # [<en>] Name of the image that will be used to create the root disk.
      # [<en>] The installation requires Linux OS images with cloud-init pre-installed.
      # [<ru>] Имя образа, который будет использоваться для создания корневого диска.
      # [<ru>] Для установки требуются образы ОС Linux с предустановленным cloud-init.
      name: *!CHANGE_NAME*
---
# [<en>] Section containing parameters of worker node group.
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
      kind: DVPInstanceClass
      name: worker
    # [<en>] The maximum number of instances for the group in each zone (used by the autoscaler).
    # [<en>] You might consider changing this.
    # [<ru>] Максимальное количество инстансов в каждой зоне (используется при масштабировании).
    # [<ru>] Возможно, захотите изменить.
    maxPerZone: 1
    # [<en>] The minimum number of instances for the group in each zone. To launch more nodes, increase maxPerZone or add more zones.
    # [<ru>] Минимальное количество инстансов в каждой зоне. Чтобы запустить больше узлов, увеличьте maxPerZone или добавьте зоны.
    minPerZone: 1
  nodeType: CloudEphemeral
---
# [<en>] Section containing parameters of the NGINX Ingress controller.
# [<en>] https://deckhouse.io/modules/402-ingress-nginx/cr.html
# [<ru>] Секция, описывающая параметры NGINX Ingress controller.
# [<ru>] https://deckhouse.ru/modules/402-ingress-nginx/cr.html
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  # [<en>] Describes on which nodes the Ingress Controller will be located. The label "node.deckhouse.io/group: <NAME_GROUP_NAME>" is set automatically.
  # [<ru>] Описывает, на каких узлах будет находиться Ingress-контроллер. Лейбл "node.deckhouse.io/group: <NODE_GROUP_NAME>" устанавливается автоматически.
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
  # [<en>] echo "<GENERATED_PASSWORD>" | htpasswd -BinC 10 "" | cut -d: -f2 | base64 -w0
  # [<ru>] Это хэш пароля <GENERATED_PASSWORD>, сгенерированного при загрузке страницы "Быстрого Старта".
  # [<ru>] Сгенерируйте свой или используйте этот, но только для тестирования
  # [<ru>] echo "<GENERATED_PASSWORD>" | htpasswd -BinC 10 "" | cut -d: -f2 | base64 -w0
  # [<en>] You might consider changing this.
  # [<ru>] Возможно, захотите изменить.
  password: <GENERATED_PASSWORD_HASH>
