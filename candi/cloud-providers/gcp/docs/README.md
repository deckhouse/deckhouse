title: "Cloud provider — GCP: Развертывание"

## Поддерживаемые схемы размещения

Схема размещения описывается объектом GCPClusterConfiguration.

Его поля:

* `layout` — архитектура расположения ресурсов в облаке.
    * Варианты — `Standard` или `WithoutNAT` (описание ниже).
* `standard` — настройки для лейаута `Standard`.
    * `cloudNATAddresses` — список имен публичных статических IP-адресов для `Cloud NAT`. Если параметр не определен, то по умолчанию будет аллоцирован один адрес и в зависимости от нагрузки, количество адресов будет автоматически изменяться (рекомендованный Google режим). Подробнее про `CloudNAT` см. [benefits](https://cloud.google.com/nat/docs/overview#benefits),  [addresses](https://cloud.google.com/nat/docs/ports-and-addresses#addresses).
* `sshKey` — публичный ключ для доступа на ноды под пользователем `user`.
* `subnetworkCIDR` — подсеть, в которой будут работать ноды кластера.
* `peeredVPCs` — список GCP VPC networks, с которыми будет объединена сеть кластера. Сервис-аккаунт должен иметь доступ ко всем перечисленным VPC. Если доступа нет, то пиринг необходимо [настраивать вручную](https://cloud.google.com/vpc/docs/using-vpc-peering#gcloud).
* `labels` — список лейблов, которые будут прикреплены ко всем ресурсам (которые это поддерживают) кластера. Подробнее про лейблы можно прочитать в [официальной документации](https://cloud.google.com/resource-manager/docs/creating-managing-labels).
    * Формат — `key: value`.
* `masterNodeGroup` — спека для описания NG мастера.
    * `replicas` — сколько мастер-узлов создать.
    * `zones` — список зон, в которых допустимо создавать мастер-узлы.
    * `instanceClass` — частичное содержимое полей [GCPInstanceClass](/modules/030-cloud-provider-gcp/docs#gcpinstanceclass-custom-resource).  Параметры, обозначенные **жирным** шрифтом уникальны для `GCPClusterConfiguration`. Допустимые параметры:
        * `machineType`
        * `image`
        * `diskSizeGb`
        * `additionalNetworkTags`
        * `additionalLabels`
        * **`disableExternalIP`** — параметр доступен только для layout `Standard`.
            * `true` —  значение по умолчанию. Узлы не имеют публичных адресов, доступ в интернет осуществляется через `CloudNAT`.
            * `false` — для узлов создаются статические публичные адреса, они же используются для One-to-one NAT.
* `nodeGroups` — массив дополнительных NG для создания статичных узлов (например, для выделенных фронтов или шлюзов). Настройки NG:
    * `name` — имя NG, будет использоваться для генерации имен нод.
    * `replicas` — количество нод.
    * `zones` — список зон, в которых допустимо создавать статичные-узлы.
    * `instanceClass` — частичное содержимое полей [GCPInstanceClass](/modules/030-cloud-provider-gcp/docs#gcpinstanceclass-custom-resource).  Параметры, обозначенные **жирным** шрифтом уникальны для `GCPClusterConfiguration`. Допустимые параметры:
        * `machineType`
        * `image`
        * `diskSizeGb`
        * `additionalNetworkTags`
        * `additionalLabels`
        * **`disableExternalIP`** — параметр доступен только для layout `Standard`.
            * `true` —  значение по умолчанию. Узлы не имеют публичных адресов, доступ в интернет осуществляется через `CloudNAT`.
            * `false` — для узлов создаются статические публичные адреса, они же используются для One-to-one NAT.
* `provider` — параметры подключения к API GCP.
    * `region` — имя региона в котором будут заказываться instances
    * `serviceAccountJSON` — `service account key` в json-формате. [Создание сервис-аккаунта](#создание-сервис-аккаунта)

### Standard
* Для кластера создаётся отдельная VPC с [Cloud NAT](https://cloud.google.com/nat/docs/overview).
* Ноды в кластере не имеют публичных IP адресов.
* Публичные IP адреса можно назначить на master и статические ноды.
    * При этом будет использоваться One-to-one NAT для отображения публичного IP-адреса в IP-адрес ноды (следует помнить, что CloudNAT в этом случае использоваться не будет).
* Если master не имеет публичного IP, то для установки и доступа в кластер, необходим дополнительный инстанс с публичным IP (aka bastion).
* Между VPC кластера и другими VPC можно настроить peering.
![resources](https://docs.google.com/drawings/d/e/2PACX-1vR1oHqbXPJPYxUXwpkRGM6VPpZaNc8WoGH-N0Zqb9GexSc-NQDvsGiXe_Hc-Z1fMQWBRawuoy8FGENt/pub?w=989&amp;h=721)
<!--- Исходник: https://docs.google.com/drawings/d/1VTAoz6-65q7m99KA933e1phWImirxvb9-OLH9DRtWPE/edit --->

```
apiVersion: deckhouse.io/v1alpha1
kind: GCPClusterConfiguration
layout: Standard
standard:
  cloudNATAddresses:                                         # optional, compute address names from this list are used as addresses for Cloud NAT
  - example-address-1
  - example-address-2
subnetworkCIDR: 10.0.0.0/24                                  # required
peeredVPCs:                                                  # optional, list of GCP VPC Networks with which Kubernetes VPC Network will be peered
- default
sshKey: "ssh-rsa ..."                                        # required
labels:
  kube: example
masterNodeGroup:
  replicas: 1
  zones:                                                     # optional
  - europe-west4-b
  instanceClass:
    machineType: n1-standard-4                               # required
    image: projects/ubuntu-os-cloud/global/images/ubuntu-1804-bionic-v20190911    # required
    diskSizeGb: 20                                           # optional, local disk is used if not specified
    disableExternalIP: false                                 # optional, by default master has externalIP
    additionalNetworkTags:                                   # optional
    - tag1
    additionalLabels:                                        # optional
      kube-node: master
nodeGroups:
- name: static
  replicas: 1
  zones:                                                     # optional
  - europe-west4-b
  instanceClass:
    machineType: n1-standard-4                               # required
    image: projects/ubuntu-os-cloud/global/images/ubuntu-1804-bionic-v20190911    # required
    diskSizeGb: 20                                           # optional, local disk is used if not specified
    disableExternalIP: true                                  # optional, by default nodes do not have externalIP
    additionalNetworkTags:                                   # optional
    - tag1
    additionalLabels:                                        # optional
      kube-node: static
provider:
  region: europe-west4                                       # required
  serviceAccountJSON: |                                      # required
    {
      "type": "service_account",
      "project_id": "sandbox",
      "private_key_id": "98sdcj5e8c7asd98j4j3n9csakn",
      "private_key": "-----BEGIN PRIVATE KEY-----",
      "client_id": "342975658279248",
      "auth_uri": "https://accounts.google.com/o/oauth2/auth",
      "token_uri": "https://oauth2.googleapis.com/token",
      "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
      "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/k8s-test%40sandbox.iam.gserviceaccount.com"
    }
```

### WithoutNAT
* Для кластера создаётся отдельная VPC, все ноды кластера имеют публичные IP-адреса.
* Между VPC кластера и другими VPC можно настроить peering.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vTq2Jlx4k8OXt4acHeW6NvqABsZIPSDoOldDiGERYHWHmmKykSjXZ_ADvKecCC1L8Jjq4143uv5GWDR/pub?w=989&amp;h=721)
<!--- Исходник: https://docs.google.com/drawings/d/1uhWbQFiycsFkG9D1vNbJNrb33Ih4YMdCxvOX5maW5XQ/edit --->

```
apiVersion: deckhouse.io/v1alpha1
kind: GCPClusterConfiguration
layout: WithoutNAT
subnetworkCIDR: 10.0.0.0/24                                 # required
peeredVPCs:                                                 # optional, list of GCP VPC Networks with which Kubernetes VPC Network will be peered
- default
labels:
  kube: example
masterNodeGroup:
  replicas: 1
  zones:                                                     # optional
  - europe-west4-b
  instanceClass:
    machineType: n1-standard-4                               # required
    image: projects/ubuntu-os-cloud/global/images/ubuntu-1804-bionic-v20190911    # required
    diskSizeGb: 20                                           # optional, local disk is used if not specified
    additionalNetworkTags:                                   # optional
    - tag1
    additionalLabels:                                        # optional
      kube-node: master
nodeGroups:
- name: static
  replicas: 1
  zones:                                                     # optional
  - europe-west4-b
  instanceClass:
    machineType: n1-standard-4                               # required
    image: projects/ubuntu-os-cloud/global/images/ubuntu-1804-bionic-v20190911    # required
    diskSizeGb: 20                                           # optional, local disk is used if not specified
    additionalNetworkTags:                                   # optional
    - tag1
    additionalLabels:                                        # optional
      kube-node: static
provider:
  region: europe-west4                                       # required
  serviceAccountJSON: |                                      # required
    {
      "type": "service_account",
      "project_id": "sandbox",
      "private_key_id": "98sdcj5e8c7asd98j4j3n9csakn",
      "private_key": "-----BEGIN PRIVATE KEY-----",
      "client_id": "342975658279248",
      "auth_uri": "https://accounts.google.com/o/oauth2/auth",
      "token_uri": "https://oauth2.googleapis.com/token",
      "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
      "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/k8s-test%40sandbox.iam.gserviceaccount.com"
    }
```


## Создание сервис-аккаунта

- [Поддерживаемые схемы размещения](#поддерживаемые-схемы-размещения)
  - [Standard](#standard)
  - [WithoutNAT](#withoutnat)
- [Создание сервис-аккаунта](#создание-сервис-аккаунта)
  - [Google cloud console](#google-cloud-console)
  - [gcloud command-line tool](#gcloud-command-line-tool)

**Внимание!** `service account key` невозможно восстановить, только удалить и создать новый.

### Google cloud console

Переходим по [ссылке](https://console.cloud.google.com/iam-admin/serviceaccounts) , выбираем проект и создаем новый сервис-аккаунт или выбираем существующий.

Список необходимых ролей:
```
Compute Admin
Service Account User
Network Management Admin
```

Роли можно прикрепить на этапе создания сервис-аккаунта, либо изменить список на [странице](https://console.cloud.google.com/iam-admin/iam).

Чтобы получить `service account key` в JSON-формате, на [странице](https://console.cloud.google.com/iam-admin/serviceaccounts) в колонке Actions необходимо кликнуть на три вертитикальные точки и выбрать Create key, тип ключа JSON.

### gcloud command-line tool

Список необходимых ролей:
```
roles/compute.admin
roles/iam.serviceAccountUser
roles/networkmanagement.admin
```

* экспортируем переменные
```shell
export PROJECT=sandbox
export SERVICE_ACCOUNT_NAME=k8s-test
```

* выбираем проект
```shell
gcloud config set project $PROJECT
```

* создаем сервис-аккаунт
```
gcloud iam service-accounts create $SERVICE_ACCOUNT_NAME
```

* прикрепляем роли к сервис-аккаунту
```
for role in roles/compute.admin roles/iam.serviceAccountUser roles/networkmanagement.admin; do gcloud projects add-iam-policy-binding ${PROJECT} --member=serviceAccount:${SERVICE_ACCOUNT_NAME}@${PROJECT}.iam.gserviceaccount.com --role=${role}; done
```

* проверяем список ролей
```
gcloud projects get-iam-policy ${PROJECT} --flatten="bindings[].members" --format='table(bindings.role)' --filter="bindings.members:${SERVICE_ACCOUNT_NAME}@${PROJECT}.iam.gserviceaccount.com"
```

* создаем `service account key`
```
gcloud iam service-accounts keys create --iam-account ${SERVICE_ACCOUNT_NAME}@${PROJECT}.iam.gserviceaccount.com ~/service-account-key-${PROJECT}-${SERVICE_ACCOUNT_NAME}.json
```
