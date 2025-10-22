---
title: "Cloud provider — GCP: схемы размещения"
description: "Описание схем размещения и взаимодействия ресурсов в GCP при работе облачного провайдера Deckhouse."
---

Поддерживаются две схемы размещения. Ниже подробнее о каждой их них.

## Standard

* Для кластера создается отдельная VPC с [Cloud NAT](https://cloud.google.com/nat/docs/overview).
* Узлы в кластере не имеют публичных IP-адресов.
* Публичные IP-адреса можно назначить на статические и master-узлы:
  * При этом будет использоваться One-to-One NAT для отображения публичного IP-адреса в IP-адрес узла (следует помнить, что CloudNAT в этом случае использоваться не будет).
* Если master не имеет публичного IP-адреса, для установки и доступа в кластер необходим дополнительный инстанс с публичным IP-адресом (например, bastion-хост).
* Между VPC кластера и другими VPC можно настроить пиринговое соединение.

![resources](images/gcp-standard.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10164&t=IvETjbByf1MSQzcm-0 --->

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

## WithoutNAT

* Для кластера создается отдельная VPC, все узлы кластера имеют публичные IP-адреса.
* Между VPC кластера и другими VPC можно настроить пиринговое соединение.

![resources](images/gcp-withoutnat.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10296&t=IvETjbByf1MSQzcm-0 --->

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
