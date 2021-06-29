### Подготовка окружения
Чтобы Deckhouse смог управлять ресурсами в облаке, необходимо создать сервисный аккаунт в облачном провайдере и выдать ему права на редактирование. Подробная инструкция по созданию сервисного аккаунта в Яндекс.Облаке доступна в [документации провайдера](https://cloud.yandex.com/en/docs/resource-manager/operations/cloud/set-access-bindings). Здесь мы представим краткую последовательность необходимых действий:

- Создайте пользователя с именем `candi`. В ответ вернутся параметры пользователя:
  ```yaml
yc iam service-account create --name candi
id: <userId>
folder_id: <folderId>
created_at: "YYYY-MM-DDTHH:MM:SSZ"
name: candi
```
- Назначьте роль `editor` вновь созданному пользователю для своего облака:
  ```yaml
yc resource-manager folder add-access-binding <cloudname> --role editor --subject serviceAccount:<userId>
```
- Создайте JSON-файл с параметрами авторизации пользователя в облаке. В дальнейшем с помощью этих данных будем авторизовываться в облаке:
  ```yaml
yc iam key create --service-account-name candi --output candi-sa-key.json
```

### Подготовка конфигурации
- Сгенерируйте на машине-установщике SSH-ключ для доступа к виртуальным машинам в облаке. В Linux и macOS это можно сделать с помощью консольной утилиты `ssh-keygen`. Публичную часть ключа необходимо включить в файл конфигурации: она будет использована для доступа к узлам облака.
- Выберите layout — архитектуру размещения объектов в облаке *(для каждого провайдера есть несколько таких предопределённых layouts в Deckhouse)*. Для примера с Яндекс.Облаком мы возьмем вариант **WithoutNAT**. В данной схеме размещения NAT (любого вида) не используется, а каждому узлу выдаётся публичный IP.
- Задайте минимальные 3 секции параметров для будущего кластера в файле `config.yml`:

{% offtopic title="config.yml для CE" %}
```yaml
# секция с общими параметрами кластера (ClusterConfiguration)
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
# тип секции конфигурации
kind: ClusterConfiguration
# тип инфраструктуры: bare-metal (Static) или облако (Cloud)
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
apiVersion: deckhouse.io/v1alpha1
# тип секции конфигурации
kind: InitConfiguration
# секция с параметрами Deckhouse
deckhouse:
  # адрес реестра с образом инсталлятора; указано значение по умолчанию для CE-сборки Deckhouse
  # подробнее см. в описании следующего шага
  imagesRepo: registry.deckhouse.io/deckhouse/сe
  # строка с параметрами подключения к Docker registry
  registryDockerCfg: eyJhdXRocyI6IHsgInJlZ2lzdHJ5LmRlY2tob3VzZS5pbyI6IHt9fX0=
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
    prometheusMadisonIntegrationEnabled: false
    nginxIngressEnabled: false
---
# секция с параметрами облачного провайдера
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
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
{% endofftopic %}
{% offtopic title="config.yml для EE" %}
```yaml
# секция с общими параметрами кластера (ClusterConfiguration)
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
# тип секции конфигурации
kind: ClusterConfiguration
# тип инфраструктуры: bare-metal (Static) или облако (Cloud)
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
apiVersion: deckhouse.io/v1alpha1
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
      # имя кластера; используется, например, в лейблах алертов Prometheus
      clusterName: somecluster
      # имя проекта для кластера; используется для тех же целей
      project: someproject
      modules:
        # шаблон, который будет использоваться для составления адресов системных приложений в кластере
        # например, Grafana для %s.somedomain.com будет доступна на домене grafana.somedomain.com
        publicDomainTemplate: "%s.somedomain.com"
    prometheusMadisonIntegrationEnabled: false
    nginxIngressEnabled: false
---
# секция с параметрами облачного провайдера
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
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
{% endofftopic %}

Примечания:
- Полный список поддерживаемых облачных провайдеров и настроек для них доступен в секции документации [Cloud providers](/ru/documentation/v1/kubernetes.html).
- Подробнее о каналах обновления Deckhouse (release channels) можно почитать в [документации](/ru/documentation/v1/deckhouse-release-channels.html).
