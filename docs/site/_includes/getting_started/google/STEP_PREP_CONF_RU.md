Подготовьте конфигурацию для установки **Deckhouse Platform {% if page.revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %}**:
- Выберите layout — архитектуру размещения объектов в облаке *(для каждого провайдера есть несколько таких предопределённых layouts в Deckhouse Platform)*. Для примера с Google Cloud мы возьмем вариант **Standard**. В данной схеме:
    - Для кластера создаётся отдельная VPC с Cloud NAT.
    - Узлы в кластере не имеют публичных IP-адресов.
    - Публичные IP-адреса можно назначить на master- и статические узлы. При этом будет использоваться One-to-one NAT для отображения публичного IP-адреса в IP-адрес узла (следует помнить, что CloudNAT в этом случае использоваться не будет).
    - Между VPC кластера и другими VPC можно настроить peering.

- Задайте минимальные 3 секции параметров для будущего кластера в файле `config.yml`.
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
    provider: GCP
    # префикс для объектов кластера для их отличия (используется, например, при маршрутизации)
    prefix: "google-demo"
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
        # имя кластера; используется, например, в лейблах алертов Prometheus
        clusterName: somecluster
        # имя проекта для кластера; используется для тех же целей
        project: someproject
        modules:
          # шаблон, который будет использоваться для составления адресов системных приложений в кластере
          # например, Grafana для %s.somedomain.com будет доступна на домене grafana.somedomain.com
          publicDomainTemplate: "%s.somedomain.com"
  ---
  # секция с параметрами облачного провайдера
  # используемая версия API Deckhouse
  apiVersion: deckhouse.io/v1
  # тип секции конфигурации
  kind: GCPClusterConfiguration
  # layout — архитектура расположения ресурсов в облаке
  layout: Standard
  # адресное пространство узлов кластера
  subnetworkCIDR: 10.0.0.0/24
  # публичная часть SSH-ключа для доступа к узлам облака
  sshPublicKey: ssh-rsa <SSH_PUBLIC_KEY>
  # метка кластера, используется для идентификации в качестве префикса
  labels:
    kube: example
  # параметры группы master-узла
  masterNodeGroup:
    # количество реплик мастера
    # если будет больше одного мастер-узла, то control-plane на всех master-узлах будет развернут автоматическии
    replicas: 1
    # параметры используемого образа виртуальной машины
    instanceClass:
      # тип виртуальной машины
      machineType: n1-standard-4
      # используемый образ
      image: projects/ubuntu-os-cloud/global/images/ubuntu-1804-bionic-v20190911
      # отключать ли назначение внешнего IP-адреса для кластера
      disableExternalIP: false
  # идентификатор облака и каталога Yandex.Cloud
  provider:
    # регион привязки кластера
    region: europe-west3
    serviceAccountJSON: |
      {
        "type": "service_account",
        "project_id": "somproject-sandbox",
        "private_key_id": "***",
        "private_key": "***",
        "client_email": "mail@somemail.com",
        "client_id": "<client_id>",
        "auth_uri": "https://accounts.google.com/o/oauth2/auth",
        "token_uri": "https://oauth2.googleapis.com/token",
        "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
        "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/somproject-sandbox.gserviceaccount.com"
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
    provider: GCP
    # префикс для объектов кластера для их отличия (используется, например, при маршрутизации)
    prefix: "google-demo"
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
    # используемый канал обновлений
    releaseChannel: Beta
    configOverrides:
      global:
        # имя кластера; используется, например, в лейблах алертов Prometheus
        clusterName: somecluster
        # имя проекта для кластера; используется для тех же целей
        project: someproject
        modules:
          # шаблон, который будет использоваться для составления адресов системных приложений в кластере
          # например, Grafana для %s.somedomain.com будет доступна на домене grafana.somedomain.com
          publicDomainTemplate: "%s.somedomain.com"
  ---
  # секция с параметрами облачного провайдера
  # используемая версия API Deckhouse
  apiVersion: deckhouse.io/v1
  # тип секции конфигурации
  kind: GCPClusterConfiguration
  # layout — архитектура расположения ресурсов в облаке
  layout: Standard
  # адресное пространство узлов кластера
  subnetworkCIDR: 10.0.0.0/24
  # публичная часть SSH-ключа для доступа к узлам облака
  sshPublicKey: ssh-rsa <SSH_PUBLIC_KEY>
  # метка кластера, используется для идентификации в качестве префикса
  labels:
    kube: example
  # параметры группы master-узла
  masterNodeGroup:
    # количество реплик мастера
    # если будет больше одного мастер-узла, то control-plane на всех master-узлах будет развернут автоматическии
    replicas: 1
    # параметры используемого образа виртуальной машины
    instanceClass:
      # тип виртуальной машины
      machineType: n1-standard-4
      # используемый образ
      image: projects/ubuntu-os-cloud/global/images/ubuntu-1804-bionic-v20190911
      # отключать ли назначение внешнего IP-адреса для кластера
      disableExternalIP: false
  # идентификатор облака и каталога Yandex.Cloud
  provider:
    # регион привязки кластера
    region: europe-west3
    serviceAccountJSON: |
      {
        "type": "service_account",
        "project_id": "somproject-sandbox",
        "private_key_id": "***",
        "private_key": "***",
        "client_email": "mail@somemail.com",
        "client_id": "<client_id>",
        "auth_uri": "https://accounts.google.com/o/oauth2/auth",
        "token_uri": "https://oauth2.googleapis.com/token",
        "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
        "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/somproject-sandbox.gserviceaccount.com"
      }
  ```
{%- endif %}

> Подробнее о каналах обновления Deckhouse Platform (release channels) можно почитать в [документации](/ru/documentation/v1/deckhouse-release-channels.html).
