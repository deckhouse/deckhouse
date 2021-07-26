Подготовьте конфигурацию для установки **Deckhouse Platform {% if page.revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %}**:
- Сгенерируйте на машине-установщике SSH-ключ для доступа к виртуальным машинам в облаке. В Linux и macOS это можно сделать с помощью консольной утилиты `ssh-keygen`. Публичную часть ключа необходимо включить в файл конфигурации: она будет использована для доступа к узлам облака.
- Выберите layout — архитектуру размещения объектов в облаке *(для каждого провайдера есть несколько таких предопределённых layouts в Deckhouse Platform)*. Для примера с Яндекс.Облаком мы возьмем вариант **WithoutNAT**. В данной схеме размещения NAT (любого вида) не используется, а каждому узлу выдаётся публичный IP.
- Задайте минимальные 3 секции параметров для будущего кластера в файле `config.yml`:
{% if page.revision == 'ee' %}
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
    provider: Yandex
    # префикс для объектов кластера для их отличия (используется, например, при маршрутизации)
    prefix: "yandex-demo"
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
  # секция с параметрами Deckhouse
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
  # используемая версия API Deckhouse
  apiVersion: deckhouse.io/v1
  # тип секции конфигурации
  kind: YandexClusterConfiguration
  # публичная часть SSH-ключа для доступа к узлам облака
  sshPublicKey: ssh-rsa <SSH_PUBLIC_KEY>
  # layout — архитектура расположения ресурсов в облаке
  layout: WithoutNAT
  # адресное пространство узлов кластера
  nodeNetworkCIDR: 10.100.0.0/21
  # параметры группы master-узла
  masterNodeGroup:
    # количество реплик мастера
    # если будет больше одного мастер-узла, то control-plane на всех master-узлах будет развернут автоматическии
    replicas: 1
    # количество CPU, RAM, дискового пространства, используемый образ виртуальной машины и политика назначения внешних IP-адресов
    instanceClass:
      cores: 4
      memory: 8192
      imageID: fd8vqk0bcfhn31stn2ts
      diskSizeGB: 40
      externalIPAddresses:
      - Auto
  # идентификатор облака и каталога Yandex.Cloud
  provider:
    cloudID: "***"
    folderID: "***"
    # данные сервисного аккаунта облачного провайдера, имеющего права на создание и управление виртуальными машинами
    # это содержимое файла candi-sa-key.json, который был сгенерирован выше
    serviceAccountJSON: |
      {
        "id": "***",
        "service_account_id": "***",
        "created_at": "2020-08-17T08:56:17Z",
        "key_algorithm": "RSA_2048",
        "public_key": ***,
        "private_key": ***
      }
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
    provider: Yandex
    # префикс для объектов кластера для их отличия (используется, например, при маршрутизации)
    prefix: "yandex-demo"
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
  # секция с параметрами облачного провайдера
  # используемая версия API Deckhouse
  apiVersion: deckhouse.io/v1
  # тип секции конфигурации
  kind: YandexClusterConfiguration
  # публичная часть SSH-ключа для доступа к узлам облака
  sshPublicKey: ssh-rsa <SSH_PUBLIC_KEY>
  # layout — архитектура расположения ресурсов в облаке
  layout: WithoutNAT
  # адресное пространство узлов кластера
  nodeNetworkCIDR: 10.100.0.0/21
  # параметры группы master-узла
  masterNodeGroup:
    # количество реплик мастера
    # если будет больше одного мастер-узла, то control-plane на всех master-узлах будет развернут автоматическии
    replicas: 1
    # количество CPU, RAM, дискового пространства, используемый образ виртуальной машины и политика назначения внешних IP-адресов
    instanceClass:
      cores: 4
      memory: 8192
      imageID: fd8vqk0bcfhn31stn2ts
      diskSizeGB: 40
      externalIPAddresses:
      - Auto
  # идентификатор облака и каталога Yandex.Cloud
  provider:
    cloudID: "***"
    folderID: "***"
    # данные сервисного аккаунта облачного провайдера, имеющего права на создание и управление виртуальными машинами
    # это содержимое файла candi-sa-key.json, который был сгенерирован выше
    serviceAccountJSON: |
      {
        "id": "***",
        "service_account_id": "***",
        "created_at": "2020-08-17T08:56:17Z",
        "key_algorithm": "RSA_2048",
        "public_key": ***,
        "private_key": ***
      }
  ```
{%- endif %}

> Подробнее о каналах обновления Deckhouse Platform (release channels) можно почитать в [документации](/ru/documentation/v1/deckhouse-release-channels.html).
