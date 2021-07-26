Подготовьте конфигурацию для установки **Deckhouse Platform {% if page.revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %}**:
- Сгенерируйте на машине-установщике SSH-ключ для доступа к виртуальным машинам в облаке. В Linux и macOS это можно сделать с помощью консольной утилиты `ssh-keygen`. Публичную часть ключа необходимо включить в файл конфигурации: она будет использована для доступа к узлам облака.

- Выберите layout — архитектуру размещения объектов в облаке *(для каждого провайдера есть несколько таких предопределённых layouts в Deckhouse Platform)*. Для примера с OpenStack возьмем вариант **Standard**. В данной схеме создаётся внутренняя сеть кластера со шлюзом в публичную сеть, узлы не имеют публичных IP-адресов, а для мастера заказывается floating IP.

- Задайте минимальные 3 секции параметров для будущего кластера в файле `config.yml`:

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
    provider: OpenStack
    # префикс для объектов кластера для их отличия (используется, например, при маршрутизации)
    prefix: "mailru-demo"
  # адресное пространство pod’ов кластера
  podSubnetCIDR: 10.111.0.0/16
  # адресное пространство для service’ов кластера
  serviceSubnetCIDR: 10.222.0.0/16
  # устанавливаемая версия Kubernetes
  kubernetesVersion: "1.19"
  # домен кластера (используется для локальной маршрутизации)
  clusterDomain: "cluster.local"
  ---
  # секция первичной инициализации кластера Deckhouse (InitConfiguration)
  # используемая версия API Deckhouse
  apiVersion: deckhouse.io/v1
  # тип секции конфигурации
  kind: InitConfiguration
  # конфигурация Deckhouse
  deckhouse:
    # адрес реестра с образом инсталлятора; указано значение по умолчанию для EE-сборки Deckhouse
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
  # секция с параметрами облачного провайдера
  apiVersion: deckhouse.io/v1
  # тип секции конфигурации
  kind: OpenStackClusterConfiguration
  # layout — архитектура расположения ресурсов в облаке
  layout: Standard
  # параметры группы master-узла
  masterNodeGroup:
    # параметры используемого образа виртуальной машины
    instanceClass:
      # используемый flavor
      flavorName: Standard-2-4-50
      # используемый образ виртуальной машины
      imageName: ubuntu-18-04-cloud-amd64
      # размер HDD
      rootDiskSize: 30
    # количество реплик мастера
    # если будет больше одного мастер-узла, то control-plane на всех master-узлах будет развернут автоматическии
    replicas: 1
    # тип используемого диска
    volumeTypeMap:
      DP1: dp1-high-iops
  # параметры доступа к облаку
  provider:
    authURL: https://infra.mail.ru:35357/v3/
    domainName: users
    password: '***'
    region: RegionOne
    tenantID: '***'
    username: somename@somemail.com
  # публичная часть SSH-ключа для доступа к узлам облака
  sshPublicKey: ssh-rsa <SSH_PUBLIC_KEY>
  standard:
    # назначаемое имя для внешней подсети
    externalNetworkName: ext-net
    # адресное пространство внутренней подсети
    internalNetworkCIDR: 192.168.198.0/24
    # назначаемые DNS-серверы
    internalNetworkDNSServers:
      - 8.8.8.8
      - 8.8.4.4
    # включение политик безопасности во внутренней сети кластера
    internalNetworkSecurity: true
  ```

> Подробнее о каналах обновления Deckhouse Platform (release channels) можно почитать в [документации](/ru/documentation/v1/deckhouse-release-channels.html).
