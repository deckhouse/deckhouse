# [<en>] General cluster parameters.
# [<en>] https://deckhouse.io/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration
# [<ru>] Общие параметры кластера.
# [<ru>] https://deckhouse.ru/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration
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
# [<en>] You might consider changing this. Ensure that there are no overlaps with serviceSubnetCIDR.
# [<ru>] Возможно, захотите изменить. Убедитесь, что не будет пересечений с serviceSubnetCIDR.
podSubnetCIDR: 10.111.0.0/16
# [<en>] Address space of the cluster's services.
# [<ru>] Адресное пространство для сервисов кластера.
# [<en>] You might consider changing this. Ensure that there are no overlaps with podSubnetCIDR.
# [<ru>] Возможно, захотите изменить. Убедитесь, что не будет пересечений с podSubnetCIDR.
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: "cluster.local"
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
    # [<ru>] Канал обновлений Deckhouse. Канал Early Access достаточно стабилен, его можно использовать в продуктивных окружениях.
    # [<ru>] Если планируется использовать несколько кластеров, то рекомендуется установить на них разные каналы обновлений.
    # [<ru>] Подробнее: https://deckhouse.ru/products/kubernetes-platform/documentation/v1/reference/release-channels.html
    # [<en>] Deckhouse release channel. The Early Access channel is stable enough to be used in productive environments.
    # [<en>] If you plan to use several clusters, it is recommended to use different release channels on them.
    # [<en>] More info: https://deckhouse.io/products/kubernetes-platform/documentation/v1/reference/release-channels.html
    releaseChannel: EarlyAccess
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
# [<en>] https://deckhouse.io/modules/cloud-provider-yandex/cluster_configuration.html
# [<ru>] Настройки облачного провайдера.
# [<ru>] https://deckhouse.io/modules/cloud-provider-yandex/cluster_configuration.html
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
    # [<en>] Yandex Cloud image ID. It is recommended to use the most fresh Ubuntu 24.04 LTS image.
    # [<en>] To get one you can use this one-liner:
    # [<en>] yc compute image list --folder-id standard-images --format json | jq -r '[.[] | select(.family == "ubuntu-2404-lts")] | sort_by(.created_at)[-1].id'
    # [<en>] You might consider changing this.
    # [<ru>] ID образа в Yandex Cloud. Рекомендуется использовать актуальную сборку Ubuntu 24.04 LTS.
    # [<ru>] Для получения можете воспользоваться командой:
    # [<ru>] yc compute image list --folder-id standard-images --format json | jq -r '[.[] | select(.family == "ubuntu-2404-lts")] | sort_by(.created_at)[-1].id'
    # [<ru>] Возможно, захотите изменить.
    imageID: fd85m9q2qspfnsv055rh
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
---
# [<en>] Section containing the parameters of instance class for worker nodes.
# [<en>] https://deckhouse.io/modules/cloud-provider-yandex/cr.html
# [<ru>] Секция, описывающая параметры инстанс-класса для узлов c компонентами, обеспечивающими рабочую нагрузку.
# [<ru>] https://deckhouse.ru/modules/cloud-provider-yandex/cr.html
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: worker
spec:
  # [<en>] You might consider changing this.
  # [<ru>] Возможно, захотите изменить.
  cores: 4
  # [<en>] You might consider changing this.
  # [<ru>] Возможно, захотите изменить.
  memory: 8192
  # [<en>] You might consider changing this.
  # [<ru>] Возможно, захотите изменить.
  diskSizeGB: 30
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
      kind: YandexInstanceClass
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
    - ru-central1-a
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
