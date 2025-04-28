---
title: Конфигурация и схема размещения
permalink: ru/admin/integrations/public/gcp/сonfiguration-and-layout-scheme.html
lang: ru
---

## Схемы размещения

DKP поддерживает две схемы размещения ресурсов в облаке GCP.

### Standard

* Для кластера создается отдельная VPC с [Cloud NAT](https://cloud.google.com/nat/docs/overview).
* Узлы в кластере не имеют публичных IP-адресов.
* Публичные IP-адреса можно назначить на статические и master-узлы:
  * при этом будет использоваться One-to-One NAT для отображения публичного IP-адреса в IP-адрес узла (следует помнить, что CloudNAT в этом случае использоваться не будет).
* Если master-узел не имеет публичного IP-адреса, для установки и доступа в кластер необходим дополнительный инстанс с публичным IP-адресом (например, bastion-хост).
* Между VPC кластера и другими VPC можно настроить пиринговое соединение.

![resources](../../../../images/cloud-provider-gcp/gcp-standard.png)
<!--- Исходник: https://docs.google.com/drawings/d/1VTAoz6-65q7m99KA933e1phWImirxvb9-OLH9DRtWPE/edit --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: GCPClusterConfiguration
layout: Standard
standard:
  # Необязательный параметр. Адреса из этого списка будут использованы для
  # адресации Cloud NAT.
  cloudNATAddresses:
  - example-address-1
  - example-address-2
subnetworkCIDR: 10.0.0.0/24         # Обязательный параметр.
# Необязательный параметр, список GCP VPC Networks, с которыми Kubernetes VPC
# Network будет соединяться через пиринговое соединение.
peeredVPCs:
- default
sshKey: "<SSH_PUBLIC_KEY>"  # Обязательный параметр.
labels:
  kube: example
masterNodeGroup:
  replicas: 1
  zones:                            # Необязательный параметр.
  - europe-west4-b
  instanceClass:
    machineType: n1-standard-4      # Обязательный параметр.
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20250313  # Обязательный параметр.
    diskSizeGb: 20                  # Необязательный параметр. Если не указан — используется локальный диск.
    disableExternalIP: false        # Необязательный параметр, по умолчанию master-узел имеет externalIP.
    additionalNetworkTags:          # Необязательный параметр.
    - tag1
    additionalLabels:               # Необязательный параметр.
      kube-node: master
nodeGroups:
- name: static
  replicas: 1
  zones:                            # Необязательный параметр.
  - europe-west4-b
  instanceClass:
    machineType: n1-standard-4      # Обязательный параметр.
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20250313  # Обязательный параметр.
    diskSizeGb: 20                  # Необязательный параметр. Если не указан — используется локальный диск.
    disableExternalIP: true         # Необязательный параметр, по умолчанию узлы не имеют externalIP.
    additionalNetworkTags:          # Необязательный параметр.
    - tag1
    additionalLabels:               # Необязательный параметр.
      kube-node: static
provider:
  region: europe-west4              # Обязательный параметр.
  serviceAccountJSON: |             # Обязательный параметр.
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

* Для кластера создается отдельная VPC, все узлы кластера имеют публичные IP-адреса.
* Между VPC кластера и другими VPC можно настроить пиринговое соединение.

![resources](../../../../images/cloud-provider-gcp/gcp-withoutnat.png)
<!--- Исходник: https://docs.google.com/drawings/d/1uhWbQFiycsFkG9D1vNbJNrb33Ih4YMdCxvOX5maW5XQ/edit --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: GCPClusterConfiguration
layout: WithoutNAT
subnetworkCIDR: 10.0.0.0/24         # Обязательный параметр.
# Необязательный параметр, список GCP VPC Networks, с которыми Kubernetes VPC
# Network будет соединяться через пиринговое соединение.
peeredVPCs:
- default
labels:
  kube: example
masterNodeGroup:
  replicas: 1
  zones:                            # Необязательный параметр.
  - europe-west4-b
  instanceClass:
    machineType: n1-standard-4      # Обязательный параметр.
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20250313  # Обязательный параметр.
    diskSizeGb: 20                  # Необязательный параметр, Если не указан — используется локальный диск.
    additionalNetworkTags:          # Необязательный параметр.
    - tag1
    additionalLabels:               # Необязательный параметр.
      kube-node: master
nodeGroups:
- name: static
  replicas: 1
  zones:                            # Необязательный параметр.
  - europe-west4-b
  instanceClass:
    machineType: n1-standard-4      # Обязательный параметр.
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20250313  # Обязательный параметр.
    diskSizeGb: 20                  # Необязательный параметр. Если не указан — используется локальный диск.
    additionalNetworkTags:          # Необязательный параметр.
    - tag1
    additionalLabels:               # Необязательный параметр.
      kube-node: static
provider:
  region: europe-west4              # Обязательный параметр.
  serviceAccountJSON: |             # Обязательный параметр.
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

## Конфигурация

Интеграции с GCP осуществляется с помощью ресурса GCPClusterConfiguration, который описывает конфигурацию облачного кластера в GCP и используется облачным провайдером, если управляющий слой (control plane) кластера размещён в облаке. Отвечающий за интеграцию модуль DKP настраивается автоматически, исходя из выбранной схемы размещения.

Выполните следующую команду, чтобы изменить конфигурацию в работающем кластере:

```shell
kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller edit provider-cluster-configuration
```

> После изменения параметров узлов необходимо выполнить команду [dhctl converge](../../deckhouse-faq.html#изменение-конфигурации), чтобы изменения вступили в силу.

Пример конфигурации:

```yaml
apiVersion: deckhouse.io/v1
kind: GCPClusterConfiguration
layout: WithoutNAT
sshKey: "<SSH_PUBLIC_KEY>"
subnetworkCIDR: 10.36.0.0/24
masterNodeGroup:
  replicas: 1
  zones:
  - europe-west3-b
  instanceClass:
    machineType: n1-standard-4
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20240523a
    diskSizeGb: 50
nodeGroups:
- name: static
  replicas: 1
  zones:
  - europe-west3-b
  instanceClass:
    machineType: n1-standard-4
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20240523a
    diskSizeGb: 50
    additionalNetworkTags:
    - tag1
    additionalLabels:
      kube-node: static
provider:
  region: europe-west3
  serviceAccountJSON: "<SERVICE_ACCOUNT_JSON>"
```

Количество и параметры процесса заказа машин в облаке настраиваются в custom resource [`NodeGroup`](../../../configuration/platform-scaling/node-management.html#конфигурация-группы-узлов), в котором также указывается название используемого для этой группы узлов инстанс-класса (параметр `cloudInstances.classReference` NodeGroup). Инстанс-класс для cloud-провайдера GCP — это custom resource [`GCPInstanceClass`](cr.html#gcpinstanceclass), в котором указываются конкретные параметры самих машин.

Также для работы втоматически создаются StorageClass'ы, покрывающие все варианты дисков в GCP:

| Тип | Репликация | Имя StorageClass |
|---|---|---|
| standard | none | pd-standard-not-replicated |
| standard | regional | pd-standard-replicated |
| balanced | none | pd-balanced-not-replicated |
| balanced | regional | pd-balanced-replicated |
| ssd | none | pd-ssd-not-replicated |
| ssd | regional | pd-ssd-replicated |

Можно отфильтровать ненужные StorageClass'ы, для этого нужно указать их в параметре `exclude`.

### Настройка политик безопасности на узлах

На виртуальных машинах кластера в GCP может возникнуть необходимость ограничить или расширить входящий и исходящий трафик по различным причинам. Некоторые из них могут включать:

- Разрешение подключения к узлам кластера с виртуальных машин из другой подсети.
- Разрешение подключения к портам статического узла для работы приложения.
- Ограничение доступа к внешним ресурсам или другим виртуальным машинам в облаке по требованию службы безопасности.

Для всего этого необходимо применять дополнительные network tags.

### Установка дополнительных network tags на статических и master-узлах

Данный параметр можно задать либо при создании кластера или в уже существующем кластере. В обоих случаях дополнительные network tags указываются в `GCPClusterConfiguration`:

- для master-узлов — в секции `masterNodeGroup` в поле `additionalNetworkTags`;
- для статических узлов — в секции `nodeGroups` в конфигурации, описывающей соответствующую nodeGroup, в поле `additionalNetworkTags`.

Поле `additionalNetworkTags` содержит массив строк с именами network tags.

### Установка дополнительных network tags на эфемерных узлах

Необходимо указать параметр `additionalNetworkTags` для всех [`GCPInstanceClass`](cr.html#gcpinstanceclass) в кластере, которым нужны дополнительные network tags.

### Добавление CloudStatic узлов в кластер

К виртуальным машинам, которые вы хотите добавить к кластеру в качестве узлов, добавьте `Network Tag`, аналогичный префиксу кластера.

Префикс кластера можно узнать, воспользовавшись следующей командой:

```shell
kubectl -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' \
  | base64 -d | grep prefix
```
