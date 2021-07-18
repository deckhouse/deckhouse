Организуйте SSH-доступ между машиной, с которой будет производиться установка **Deckhouse Platform {% if page.revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %}**, и будущим master-узлом кластера.

Задайте минимальные 3 секции параметров для будущего кластера в файле `config.yml`:

{%- if page.revision == 'ee' %}
```yaml
# секция с общими параметрами кластера (ClusterConfiguration)
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
# тип секции конфигурации
kind: ClusterConfiguration
# тип инфраструктуры: bare metal (Static) или облако (Cloud)
clusterType: Static
# адресное пространство pod’ов кластера
podSubnetCIDR: 10.111.0.0/16
# адресное пространство для service’ов кластера
serviceSubnetCIDR: 10.222.0.0/16
# устанавливаемая версия Kubernetes
kubernetesVersion: "1.19"
# домен кластера, используется для локальной маршрутизации
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
      clusterName: main
      # имя проекта; используется для тех же целей
      project: someproject
      modules:
        # шаблон, который будет использоваться для составления адресов системных приложений в кластере
        # например, Grafana для %s.somedomain.com будет доступна на домене grafana.somedomain.com
        publicDomainTemplate: "%s.somedomain.com"
    cniFlannelEnabled: true
    cniFlannel:
      podNetworkMode: VXLAN
---
# секция с параметрами bare metal-кластера (StaticClusterConfiguration)
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
# тип секции конфигурации
kind: StaticClusterConfiguration
# адресное пространство для внутренней сети кластера
internalNetworkCIDRs:
- 10.0.4.0/24
```
{%- else %}
```yaml
# секция с общими параметрами кластера (ClusterConfiguration)
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
# тип секции конфигурации
kind: ClusterConfiguration
# тип инфраструктуры: bare metal (Static) или облако (Cloud)
clusterType: Static
# адресное пространство pod’ов кластера
podSubnetCIDR: 10.111.0.0/16
# адресное пространство для service’ов кластера
serviceSubnetCIDR: 10.222.0.0/16
# устанавливаемая версия Kubernetes
kubernetesVersion: "1.19"
# домен кластера, используется для локальной маршрутизации
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
      clusterName: main
      # имя проекта; используется для тех же целей
      project: someproject
      modules:
        # шаблон, который будет использоваться для составления адресов системных приложений в кластере
        # например, Grafana для %s.somedomain.com будет доступна на домене grafana.somedomain.com
        publicDomainTemplate: "%s.somedomain.com"
    cniFlannelEnabled: true
    cniFlannel:
      podNetworkMode: VXLAN
---
# секция с параметрами bare metal-кластера (StaticClusterConfiguration)
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
# тип секции конфигурации
kind: StaticClusterConfiguration
# адресное пространство для внутренней сети кластера
internalNetworkCIDRs:
- 10.0.4.0/24
```
{%- endif %}

> Подробнее о каналах обновления Deckhouse Platform (release channels) можно почитать в [документации](/ru/documentation/v1/deckhouse-release-channels.html).
