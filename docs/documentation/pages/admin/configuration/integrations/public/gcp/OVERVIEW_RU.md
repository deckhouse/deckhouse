---
title: Интеграция с Google Cloud Platform (GCP)
permalink: ru/admin/integrations/public/gcp/overview.html
lang: ru
---

Deckhouse Platform интегрируется с [Google Cloud Platform](https://cloud.google.com/) (GCP) и заказывает в нём узлы кластера. Параметры виртуальных машин задаются в ресурсе [GCPInstanceClass](/modules/cloud-provider-gcp/cr.html#gcpinstanceclass), который указывается в группе узлов [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Возможности интеграции:

- Управление ресурсами GCP:
  - создание сетевых маршрутов для сети `PodNetwork` на стороне GCP;
  - создание балансировщиков нагрузки для Service с типом LoadBalancer;
  - актуализация метаданных узлов кластера, удаление из кластера узлов, которых больше нет в GCP.
- Заказ дисков через CSI-драйвер Persistent Disk (`pd.csi.storage.gke.io`) и создание StorageClass для типов дисков GCP, чтобы из кластера можно было заказывать PersistentVolume.
- Заказ базовой инфраструктуры и CloudPermanent-узлов с помощью [Terraform/OpenTofu-провайдера](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-gcp.html#взаимодействия-модуля) `terraform-provider-google`.
- Заказ CloudEphemeral-узлов через Machine Controller Manager (MCM).
- Автоматическое включение CNI в новых кластерах. По умолчанию используется [`cni-cilium`](/modules/cni-cilium/).
