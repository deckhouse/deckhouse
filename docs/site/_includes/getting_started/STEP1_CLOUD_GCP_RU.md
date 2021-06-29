### Подготовка окружения
Чтобы Deckhouse смог управлять ресурсами в облаке Google, необходимо создать сервисный аккаунт. Подробная инструкция по этому действию доступна в [документации провайдера](https://cloud.google.com/iam/docs/service-accounts). Здесь мы представим краткую последовательность необходимых действий:

> Список необходимых ролей:
> - `roles/compute.admin`
> - `roles/iam.serviceAccountUser`
> - `roles/networkmanagement.admin`

- Экспортируйте переменные окружения:
  ```shell
export PROJECT=sandbox
export SERVICE_ACCOUNT_NAME=deckhouse
```
- Выберите project:
  ```shell
gcloud config set project $PROJECT
```
- Создайте service account:
  ```shell
gcloud iam service-accounts create $SERVICE_ACCOUNT_NAME
```
- Выполните проверку ролей service account:
  ```shell
gcloud projects get-iam-policy ${PROJECT} --flatten="bindings[].members" --format='table(bindings.role)' \
    --filter="bindings.members:${SERVICE_ACCOUNT_NAME}@${PROJECT}.iam.gserviceaccount.com"
```
- Создайте service account key:
  ```shell
gcloud iam service-accounts keys create --iam-account ${SERVICE_ACCOUNT_NAME}@${PROJECT}.iam.gserviceaccount.com \
    ~/service-account-key-${PROJECT}-${SERVICE_ACCOUNT_NAME}.json
```

### Подготовка конфигурации
-  Выберите layout — архитектуру размещения объектов в облаке *(для каждого провайдера есть несколько таких предопределённых layouts в Deckhouse)*. Для примера с Google Cloud мы возьмем вариант **Standard**. В данной схеме:
    - Для кластера создаётся отдельная VPC с Cloud NAT.
    - Узлы в кластере не имеют публичных IP-адресов.
    - Публичные IP-адреса можно назначить на master- и статические узлы. При этом будет использоваться One-to-one NAT для отображения публичного IP-адреса в IP-адрес узла (следует помнить, что CloudNAT в этом случае использоваться не будет).
    - Между VPC кластера и другими VPC можно настроить peering.

-  Задайте минимальные 3 секции параметров для будущего кластера в файле `config.yml`.
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
{% endofftopic %}

Примечания:
 -  Полный список поддерживаемых облачных провайдеров и настроек для них доступен в секции документации [Cloud providers](/ru/documentation/v1/kubernetes.html).
 -  Подробнее о каналах обновления Deckhouse (release channels) можно почитать в [документации](/ru/documentation/v1/deckhouse-release-channels.html).
