- Организуйте SSH-доступ между машиной, с которой будет производиться установка, и будущим master-узлом кластера.
- Определите 3 секции параметров для будущего кластера, создав новый конфигурационный файл `config.yml`:

{% offtopic title="config.yml для CE-инсталляций" %}
```yaml
# секция с общими параметрами кластера (ClusterConfiguration)
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
# тип секции конфигурации
kind: ClusterConfiguration
# тип инфраструктуры: bare-metal (Static) или облако (Cloud)
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
apiVersion: deckhouse.io/v1alpha1
# тип секции конфигурации
kind: InitConfiguration
# конфигурация Deckhouse
deckhouse:
  # адрес реестра с образом инсталлятора; указано значение по умолчанию для CE-сборки Deckhouse
  imagesRepo: registry.deckhouse.io/deckhouse/ce
  # строка с параметрами подключения к Docker registry
  registryDockerCfg: eyJhdXRocyI6IHsgInJlZ2lzdHJ5LmRlY2tob3VzZS5pbyI6IHt9fX0=
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
    prometheusMadisonIntegrationEnabled: false
    nginxIngressEnabled: false
---
# секция с параметрами bare metal-кластера (StaticClusterConfiguration)
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
# тип секции конфигурации
kind: StaticClusterConfiguration
# адресное пространство для внутренней сети кластера
internalNetworkCIDRs:
- 10.0.4.0/24
```
{% endofftopic %}
{% offtopic title="config.yml для EE-инсталляций" %}
```yaml
# секция с общими параметрами кластера (ClusterConfiguration)
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
# тип секции конфигурации
kind: ClusterConfiguration
# тип инфраструктуры: bare-metal (Static) или облако (Cloud)
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
apiVersion: deckhouse.io/v1alpha1
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
    prometheusMadisonIntegrationEnabled: false
    nginxIngressEnabled: false
---
# секция с параметрами bare metal-кластера (StaticClusterConfiguration)
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
# тип секции конфигурации
kind: StaticClusterConfiguration
# адресное пространство для внутренней сети кластера
internalNetworkCIDRs:
- 10.0.4.0/24
```
{% endofftopic %}
