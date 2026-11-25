---
title: Схемы размещения Google Cloud Platform
permalink: ru/admin/integrations/public/gcp/layout.html
lang: ru
---

## Схемы размещения

Данный раздел описывает возможные [схемы размещения](../../../../glossary.html#layout)
узлов кластера в инфраструктуре Google Cloud Platform (GCP). От
выбора схемы (layout) зависят принципы сетевого взаимодействия, наличие публичных
IP-адресов, маршрутизация исходящего трафика и способ подключения к узлам.

Deckhouse Platform поддерживает две схемы размещения ресурсов в облаке GCP.

### Standard

В этой схеме для кластера создаются отдельные VPC и Cloud NAT.

* Для кластера создается отдельная VPC с [Cloud NAT](https://cloud.google.com/nat/docs/overview).
* Узлы в кластере не имеют публичных IP-адресов.
* Публичные IP-адреса можно назначить на статические и
  [master-узлы](../../../../glossary.html#master-node):
  * при этом будет использоваться One-to-One NAT для отображения публичного IP-адреса в IP-адрес узла (следует помнить, что CloudNAT в этом случае использоваться не будет).
* Если master-узел не имеет публичного IP-адреса, для
  установки и доступа в кластер необходим дополнительный
  инстанс с публичным IP-адресом (например, [bastion-хост](../../../../glossary.html#bastion-host)).
* Между VPC кластера и другими VPC можно настроить пиринговое соединение.

![resources](../../../../images/cloud-provider-gcp/gcp-standard.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10164&t=Qb5yyWumzPiTBtfL-0 --->

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
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20260128  # Обязательный параметр.
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
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20260128  # Обязательный параметр.
    diskSizeGb: 20                  # Необязательный параметр. Если не указан — используется локальный диск.
    disableExternalIP: true         # Необязательный параметр, по умолчанию узлы не имеют externalIP.
    additionalNetworkTags:          # Необязательный параметр.
    - tag1
    additionalLabels:               # Необязательный параметр.
      kube-node: static
provider:
  region: europe-west4              # Обязательный параметр.
  serviceAccountJSON: "<SERVICE_ACCOUNT_JSON>"  # Обязательный параметр.
```

Замените `<SERVICE_ACCOUNT_JSON>` на JSON-ключ сервисного аккаунта GCP.

### WithoutNAT

В этой схеме для кластера создаётся отдельная VPC без Cloud NAT.

* Для кластера создается отдельная VPC, все узлы кластера имеют публичные IP-адреса.
* Между VPC кластера и другими VPC можно настроить пиринговое соединение.

![resources](../../../../images/cloud-provider-gcp/gcp-withoutnat.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10296&t=Qb5yyWumzPiTBtfL-0 --->

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
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20260128  # Обязательный параметр.
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
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20260128  # Обязательный параметр.
    diskSizeGb: 20                  # Необязательный параметр. Если не указан — используется локальный диск.
    additionalNetworkTags:          # Необязательный параметр.
    - tag1
    additionalLabels:               # Необязательный параметр.
      kube-node: static
provider:
  region: europe-west4              # Обязательный параметр.
  serviceAccountJSON: "<SERVICE_ACCOUNT_JSON>"  # Обязательный параметр.
```

Замените `<SERVICE_ACCOUNT_JSON>` на JSON-ключ сервисного аккаунта GCP.

[Настройка интеграции](configuration.html).

[Настройка сети](network.html).
