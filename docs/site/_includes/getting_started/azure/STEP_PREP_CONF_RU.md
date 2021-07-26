Подготовьте конфигурацию для установки **Deckhouse Platform {% if page.revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %}**:
- Сгенерируйте на машине-установщике SSH-ключ для доступа к виртуальным машинам в облаке. В Linux и macOS это можно сделать с помощью консольной утилиты `ssh-keygen`. Публичную часть ключа необходимо включить в файл конфигурации: она будет использована для доступа к узлам облака.
- Выберите layout — архитектуру размещения объектов в облаке *(для каждого провайдера есть несколько таких предопределённых layouts в Deckhouse)*. Для примера выберем вариант **Standard**. В данной схеме размещения:
    - Для кластера создаётся отдельная resource group.
    - По умолчанию каждому инстансу динамически выделяется один внешний IP-адрес, который используется только для доступа в интернет. На каждый IP для SNAT доступно 64000 портов. Поддерживается NAT Gateway (тарификация). Позволяет использовать статические публичные IP для SNAT. Публичные IP-адреса можно назначить на master-узлы и узлы, созданные с Terraform. Если master не имеет публичного IP, то для установки и доступа в кластер необходим дополнительный инстанс с публичным IP (aka bastion). В этом случае также потребуется настроить peering между VNet кластера и VNet bastion'а. Между VNet кластера и другими VNet можно настроить peering.
- Задайте минимальные 3 секции параметров для будущего кластера в файле `config.yml`:
{%- if page.revision == 'ee' %}
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
  provider: Azure
  # префикс для объектов кластера для их отличия (используется, например, при маршрутизации)
  prefix: "azure-demo"
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
kind: AzureClusterConfiguration
# layout — архитектура расположения ресурсов в облаке
layout: Standard
# публичная часть SSH-ключа для доступа к узлам облака
sshPublicKey: ssh-rsa <SSH_PUBLIC_KEY>
# адресное пространство виртуальной сети кластера
vNetCIDR: 10.50.0.0/16
# адресное пространство подсети кластера
subnetCIDR: 10.50.0.0/24
# параметры группы master-узла
masterNodeGroup:
  # количество реплик мастера
  # если будет больше одного мастер-узла, то control-plane на всех master-узлах будет развернут автоматическии
  replicas: 1
  # параметры используемого образа виртуальной машины
  instanceClass:
    # тип виртуальной машины
    machineSize: Standard_F4
    # размер диска
    diskSizeGb: 32
    # используемый образ виртуальной машины
    urn: Canonical:UbuntuServer:18.04-LTS:18.04.202010140
    # включать ли назначение внешнего IP-адреса для кластера
    enableExternalIP: true
# параметры доступа к облаку Azure
provider:
  subscriptionId: "***"
  clientId: "***"
  clientSecret: "***"
  tenantId: "***"
  location: "westeurope"
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
  provider: Azure
  # префикс для объектов кластера для их отличия (используется, например, при маршрутизации)
  prefix: "azure-demo"
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
      modules:
        # шаблон, который будет использоваться для составления адресов системных приложений в кластере
        # например, Grafana для %s.somedomain.com будет доступна на домене grafana.somedomain.com
        publicDomainTemplate: "%s.somedomain.com"
---
# секция с параметрами облачного провайдера
apiVersion: deckhouse.io/v1
# тип секции конфигурации
kind: AzureClusterConfiguration
# layout — архитектура расположения ресурсов в облаке
layout: Standard
# публичная часть SSH-ключа для доступа к узлам облака
sshPublicKey: ssh-rsa <SSH_PUBLIC_KEY>
# адресное пространство виртуальной сети кластера
vNetCIDR: 10.50.0.0/16
# адресное пространство подсети кластера
subnetCIDR: 10.50.0.0/24
# параметры группы master-узла
masterNodeGroup:
  # количество реплик мастера
  # если будет больше одного мастер-узла, то control-plane на всех master-узлах будет развернут автоматическии
  replicas: 1
  # параметры используемого образа виртуальной машины
  instanceClass:
    # тип виртуальной машины
    machineSize: Standard_F4
    # размер диска
    diskSizeGb: 32
    # используемый образ виртуальной машины
    urn: Canonical:UbuntuServer:18.04-LTS:18.04.202010140
    # включать ли назначение внешнего IP-адреса для кластера
    enableExternalIP: true
# параметры доступа к облаку Azure
provider:
  subscriptionId: "***"
  clientId: "***"
  clientSecret: "***"
  tenantId: "***"
  location: "westeurope"
```
{%- endif %}

> Подробнее о каналах обновления Deckhouse Platform (release channels) можно почитать в [документации](/ru/documentation/v1/deckhouse-release-channels.html).
