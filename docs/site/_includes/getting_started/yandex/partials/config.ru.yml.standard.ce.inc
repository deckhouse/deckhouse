# [<en>] General cluster parameters.
# [<en>] https://deckhouse.io/documentation/v1/installing/configuration.html#clusterconfiguration
# [<ru>] Общие параметры кластера.
# [<ru>] https://deckhouse.ru/documentation/v1/installing/configuration.html#clusterconfiguration
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: Yandex
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
  imagesRepo: registry.deckhouse.ru/deckhouse/ce
  registryDockerCfg: eyJhdXRocyI6IHsgInJlZ2lzdHJ5LmRlY2tob3VzZS5ydSI6IHt9fX0K
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
  version: 2
  enabled: true
  settings:
    controlPlaneConfigurator:
      dexCAMode: DoNotNeed
    # [<ru>] Включение доступа к API-серверу Kubernetes через Ingress.
    # [<ru>] https://deckhouse.ru/documentation/v1/modules/150-user-authn/configuration.html#parameters-publishapi
    # [<en>] Enabling access to the API server through Ingress.
    # [<en>] https://deckhouse.io/documentation/v1/modules/150-user-authn/configuration.html#parameters-publishapi
    publishAPI:
      enabled: true
      https:
        mode: Global
        global:
          kubeconfigGeneratorMasterCA: ""
---
# [<en>] Cloud provider settings.
# [<en>] https://deckhouse.io/documentation/v1/modules/030-cloud-provider-yandex/cluster_configuration.html
# [<ru>] Настройки облачного провайдера.
# [<ru>] https://deckhouse.io/documentation/v1/modules/030-cloud-provider-yandex/cluster_configuration.html
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
# [<en>] Yandex Cloud account parameters.
# [<ru>] Параметры доступа к облаку Yandex Cloud.
provider:
  # [<en>] The cloud ID.
  # [<ru>] ID облака.
  cloudID: *!CHANGE_CloudID*
  # [<en>] The folder ID.
  # [<ru>] ID каталога.
  folderID: *!CHANGE_FolderID*
  # [<en>] A JSON key generated by `yc iam key create` on the previous step.
  # [<en>] Example of serviceAccountJSON:
  # [<ru>] JSON-ключ, сгенерированный с помощью `yc iam key create` на предыдущем шаге.
  # [<ru>] Пример заполнения serviceAccountJSON:
  # serviceAccountJSON: |
  #    {
  #      "id": "...",
  #      "service_account_id": "...",
  #      "created_at": "2022-08-04T05:38:34.756137618Z",
  #      "key_algorithm": "RSA_2048",
  #      "public_key": "-----BEGIN PUBLIC KEY-----...-----END PUBLIC KEY-----\n",
  #      "private_key": "-----BEGIN PRIVATE KEY-----...-----END PRIVATE KEY-----\n"
  #    }
  serviceAccountJSON: *!CHANGE_ServiceAccountJSON*
masterNodeGroup:
  replicas: 1
  instanceClass:
    cores: 4
    memory: 8192
    # [<en>] Yandex Cloud image ID. It is recommended to use the most fresh Ubuntu 22.04 LTS image.
    # [<en>] To get one you can use this one-liner:
    # [<en>] yc compute image list --folder-id standard-images --format json | jq -r '[.[] | select(.family == "ubuntu-2204-lts")] | sort_by(.created_at)[-1].id'
    # [<en>] You might consider changing this.
    # [<ru>] ID образа в Yandex Cloud. Рекомендуется использовать актуальную сборку Ubuntu 22.04 LTS.
    # [<ru>] Для получения можете воспользоваться командой:
    # [<ru>] yc compute image list --folder-id standard-images --format json | jq -r '[.[] | select(.family == "ubuntu-2204-lts")] | sort_by(.created_at)[-1].id'
    # [<ru>] Возможно, захотите изменить.
    imageID: fd864gbboths76r8gm5f
    externalIPAddresses:
    - "Auto"
# [<en>] This subnet will be split into three equal parts; they will serve as a basis for subnets in three Yandex Cloud zones.
# [<ru>] Данная подсеть будет разделена на три равных части и использована для создания подсетей в трёх зонах Yandex Cloud.
nodeNetworkCIDR: "10.241.32.0/20"
# [<en>] Public SSH key for accessing cloud nodes.
# [<en>] This key will be added to the user on created nodes (the user name depends on the image used).
# [<ru>] Публичная часть SSH-ключа для доступа к узлам облака.
# [<ru>] Этот ключ будет добавлен пользователю на созданных узлах (имя пользователя зависит от используемого образа).
sshPublicKey: *!CHANGE_SSH_KEY*
