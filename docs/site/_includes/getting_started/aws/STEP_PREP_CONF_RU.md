Подготовьте конфигурацию для установки **Deckhouse Platform {% if page.revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %}**:
- Выберите layout — архитектуру размещения объектов в облаке *(для каждого провайдера есть несколько таких предопределённых layouts в Deckhouse Platform)*.

  Для примера с {{ page.platform_name }} мы возьмем вариант **WithoutNAT**. В данной схеме размещения виртуальные машины будут выходить в интернет через NAT Gateway с общим и единственным source IP. Все узлы, созданные с помощью Deckhouse Platform, опционально могут получить публичный IP (ElasticIP).

  Другие доступные варианты описаны в секции документации [Cloud providers](https://early.deckhouse.io/ru/documentation/v1/kubernetes.html).
- Задайте минимальные 3 секции параметров для будущего кластера в файле `config.yml`:
{%- if page.revision == 'ee' %}
  ```yaml
  # секция с общими параметрами кластера (ClusterConfiguration)
  # используемая версия API Deckhouse Platform
  apiVersion: deckhouse.io/v1
  # тип секции конфигурации
  kind: ClusterConfiguration
  # тип инфраструктуры: bare metal (Static) или облако (Cloud)
  clusterType: Cloud
  # параметры облачного провайдера
  cloud:
    # используемый облачный провайдер
    provider: AWS
    # префикс для объектов кластера для их отличия (используется, например, при маршрутизации)
    prefix: "aws-demo"
  # адресное пространство pod’ов кластера
  podSubnetCIDR: 10.111.0.0/16
  # адресное пространство для service’ов кластера
  serviceSubnetCIDR: 10.222.0.0/16
  # устанавливаемая версия Kubernetes
  kubernetesVersion: "1.19"
  # домен кластера
  clusterDomain: "cluster.local"
  ---
  # секция первичной инициализации кластера Deckhouse Platform (InitConfiguration)
  # используемая версия API Deckhouse Platform
  apiVersion: deckhouse.io/v1
  # тип секции конфигурации
  kind: InitConfiguration
  # секция с параметрами Deckhouse Platform
  deckhouse:
    # адрес реестра с образом инсталлятора; указано значение по умолчанию для EE-сборки Deckhouse Platform
    imagesRepo: registry.deckhouse.io/deckhouse/ee
    # строка с ключом для доступа к Docker registry (сгенерировано автоматически для вашего демонстрационного токена)
    registryDockerCfg: <YOUR_ACCESS_STRING_IS_HERE>
    # используемый канал обновлений
    releaseChannel: Beta
    configOverrides:
      global:
        modules:
          # шаблон, который будет использоваться для составления адресов системных приложений в кластере
          # например, Grafana для %s.somedomain.com будет доступна на домене grafana.somedomain.com
          publicDomainTemplate: "%s.somedomain.com"
  ---
  # секция, описывающая параметры облачного провайдера
  # используемая версия API Deckhouse
  apiVersion: deckhouse.io/v1
  # тип секции конфигурации
  kind: AWSClusterConfiguration
  # layout — архитектура расположения ресурсов в облаке
  layout: WithoutNAT
  # параметры доступа к облаку AWS
  provider:
    providerAccessKeyId: MYACCESSKEY
    providerSecretAccessKey: mYsEcReTkEy
    # регион привязки кластера
    region: eu-central-1
  masterNodeGroup:
    # количество реплик мастера
    # если будет больше одного мастер-узла, то control-plane на всех master-узлах будет развернут автоматическии
    replicas: 1
    # параметры инстанса
    instanceClass:
      # тип используемого инстанса
      instanceType: c5.large
      # id образа виртуальной машины
      ami: ami-0fee04b212b7499e2
  # адресное пространство облака внутри AWS
  vpcNetworkCIDR: "10.241.0.0/16"
  # адресное пространство узлов кластера
  nodeNetworkCIDR: "10.241.32.0/20"
  # публичная часть SSH-ключа для доступа к узлам облака
  sshPublicKey: ssh-rsa <SSH_PUBLIC_KEY>
  ```
{%- else %}
  ```yaml
  # секция с общими параметрами кластера (ClusterConfiguration)
  # используемая версия API Deckhouse
  apiVersion: deckhouse.io/v1
  # тип секции конфигурации
  kind: ClusterConfiguration
  # тип инфраструктуры: bare metal (Static) или облако (Cloud)
  clusterType: Cloud
  # параметры облачного провайдера
  cloud:
    # используемый облачный провайдер
    provider: AWS
    # префикс для объектов кластера для их отличия (используется, например, при маршрутизации)
    prefix: "aws-demo"
  # адресное пространство pod’ов кластера
  podSubnetCIDR: 10.111.0.0/16
  # адресное пространство для service’ов кластера
  serviceSubnetCIDR: 10.222.0.0/16
  # устанавливаемая версия Kubernetes
  kubernetesVersion: "1.19"
  # домен кластера
  clusterDomain: "cluster.local"
  ---
  # секция первичной инициализации кластера Deckhouse (InitConfiguration)
  # используемая версия API Deckhouse
  apiVersion: deckhouse.io/v1
  # тип секции конфигурации
  kind: InitConfiguration
  # секция с параметрами Deckhouse
  deckhouse:
    # используемый канал обновлений
    releaseChannel: Beta
    configOverrides:
      global:
        modules:
          # шаблон, который будет использоваться для составления адресов системных приложений в кластере
          # например, Grafana для %s.somedomain.com будет доступна на домене grafana.somedomain.com
          publicDomainTemplate: "%s.somedomain.com"
  ---
  # секция, описывающая параметры облачного провайдера
  # используемая версия API Deckhouse
  apiVersion: deckhouse.io/v1
  # тип секции конфигурации
  kind: AWSClusterConfiguration
  # layout — архитектура расположения ресурсов в облаке
  layout: WithoutNAT
  # параметры доступа к облаку AWS
  provider:
    providerAccessKeyId: MYACCESSKEY
    providerSecretAccessKey: mYsEcReTkEy
    # регион привязки кластера
    region: eu-central-1
  masterNodeGroup:
    # количество реплик мастера
    # если будет больше одного мастер-узла, то control-plane на всех master-узлах будет развернут автоматическии
    replicas: 1
    # параметры инстанса
    instanceClass:
      # тип используемого инстанса
      instanceType: c5.large
      # id образа виртуальной машины
      ami: ami-0fee04b212b7499e2
  # адресное пространство облака внутри AWS
  vpcNetworkCIDR: "10.241.0.0/16"
  # адресное пространство узлов кластера
  nodeNetworkCIDR: "10.241.32.0/20"
  # публичная часть SSH-ключа для доступа к узлам облака
  sshPublicKey: ssh-rsa <SSH_PUBLIC_KEY>
  ```
{%- endif %}

> Подробнее о каналах обновления Deckhouse Platform (release channels) можно почитать в [документации](/ru/documentation/v1/deckhouse-release-channels.html).
