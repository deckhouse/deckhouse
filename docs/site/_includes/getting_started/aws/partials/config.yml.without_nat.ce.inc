# [<en>] General cluster parameters.
# [<en>] https://deckhouse.io/documentation/v1/installing/configuration.html#clusterconfiguration
# [<ru>] Общие параметры кластера.
# [<ru>] https://deckhouse.ru/documentation/v1/installing/configuration.html#clusterconfiguration
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: AWS
  # [<en>] A prefix of objects that are created in the cloud during the installation.
  # [<en>] You might consider changing this.
  # [<ru>] Префикс объектов, создаваемых в облаке при установке.
  # [<ru>] Возможно, захотите изменить.
  prefix: cloud-demo
# [<en>] Address space of the cluster's Pods.
# [<ru>] Адресное пространство подов кластера.
podSubnetCIDR: 10.111.0.0/16
# [<en>] Address space of the cluster's services.
# [<ru>] Адресное пространство сети сервисов кластера.
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
# [<en>] Cluster domain (used for local routing).
# [<ru>] Домен кластера.
clusterDomain: "cluster.local"
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
    # [<ru>] Канал обновлений Deckhouse. Канал Early Access достаточно стабилен, его можно использовать в продуктивных окружениях.
    # [<ru>] Если планируется использовать несколько кластеров, то рекомендуется установить на них разные каналы обновлений.
    # [<ru>] Подробнее: https://deckhouse.ru/documentation/v1/deckhouse-release-channels.html
    # [<en>] Deckhouse release channel. The Early Access channel is stable enough to be used in productive environments.
    # [<en>] If you plan to use several clusters, it is recommended to use different release channels on them.
    # [<en>] More info: https://deckhouse.io/documentation/v1/deckhouse-release-channels.html
    releaseChannel: EarlyAccess
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
  version: 1
  # [<en>] Enable cni-cilium module
  # [<ru>] Включить модуль cni-cilium
  enabled: true
  settings:
    # [<en>] cni-cilium module settings
    # [<en>] https://deckhouse.io/documentation/v1/modules/021-cni-cilium/configuration.html
    # [<ru>] Настройки модуля cni-cilium
    # [<ru>] https://deckhouse.ru/documentation/v1/modules/021-cni-cilium/configuration.html
    tunnelMode: VXLAN
---
# [<en>] Cloud provider settings.
# [<en>] https://deckhouse.io/documentation/v1/modules/030-cloud-provider-aws/cluster_configuration.html
# [<ru>] Настройки облачного провайдера..
# [<ru>] https://deckhouse.io/documentation/v1/modules/030-cloud-provider-aws/cluster_configuration.html
apiVersion: deckhouse.io/v1
kind: AWSClusterConfiguration
layout: WithoutNAT
# [<en>] AWS EC2 access parameters.
# [<ru>] Параметры доступа к AWS EC2.
provider:
  providerAccessKeyId: *!CHANGE_MYACCESSKEY*
  providerSecretAccessKey: *!CHANGE_mYsEcReTkEy*
  # [<en>] Cluster region.
  # [<ru>] Регион привязки кластера.
  # [<en>] You might consider changing this.
  # [<ru>] Возможно, захотите изменить.
  region: eu-central-1
masterNodeGroup:
  replicas: 1
  instanceClass:
    # [<en>] Master node VM disk size.
    # [<ru>] Размер диска для виртуальной машины master-узла.
    # [<en>] You might consider changing this.
    # [<ru>] Возможно, захотите изменить.
    diskSizeGb: 30
    # [<en>] Master node VM disk type to use.
    # [<ru>] Используемый тип диска для виртуальной машины master-узла.
    # [<en>] You might consider changing this.
    # [<ru>] Возможно, захотите изменить.
    diskType: gp3
    # [<en>] Type of the instance.
    # [<ru>] Тип используемого инстанса.
    # [<en>] You might consider changing this.
    # [<ru>] Возможно, захотите изменить.
    instanceType: c5.xlarge
    # [<en>] Amazon Machine Image ID.
    # [<en>] The example uses the Ubuntu Server 22.04 image for the 'eu-central-1' region.
    # [<en>] Change the AMI ID if you use a different region (the 'provider.region' parameter).
    # [<en>] AMI Catalog in the AWS console: EC2 -> AMI Catalog.
    # [<ru>] ID образа виртуальной машины в Amazon.
    # [<ru>] В примере используется образ Ubuntu Server 22.04 для региона 'eu-central-1'.
    # [<ru>] Измените AMI ID, если используете другой регион в параметре 'provider.region'.
    # [<ru>] Каталог AMI в консоли AWS: EC2 -> AMI Catalog.
    # [<en>] You might consider changing this.
    # [<ru>] Возможно, захотите изменить.
    ami: ami-0caef02b518350c8b
# [<en>] Address space of the AWS cloud.
# [<ru>] Адресное пространство облака внутри AWS.
vpcNetworkCIDR: "10.241.0.0/16"
# [<en>] Address space of the cluster's nodes.
# [<ru>] Адресное пространство узлов кластера.
nodeNetworkCIDR: "10.241.32.0/20"
# [<en>] Public SSH key for accessing cloud nodes.
# [<en>] This key will be added to the user on created nodes (the user name depends on the image used).
# [<ru>] Публичная часть SSH-ключа для доступа к узлам облака.
# [<ru>] Этот ключ будет добавлен пользователю на созданных узлах (имя пользователя зависит от используемого образа).
sshPublicKey: *!CHANGE_SSH_KEY*
