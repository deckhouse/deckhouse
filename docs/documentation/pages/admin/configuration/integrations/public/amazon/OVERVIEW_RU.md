---
title: Интеграция с Amazon Web Services (AWS)
permalink: ru/admin/integrations/public/amazon/overview.html
lang: ru
---

Deckhouse Platform интегрируется с [Amazon AWS](https://aws.amazon.com/) и заказывает в нём узлы кластера. Параметры виртуальных машин задаются в ресурсе [AWSInstanceClass](/modules/cloud-provider-aws/cr.html#awsinstanceclass), который указывается в группе узлов [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Возможности интеграции:

- Управление ресурсами AWS:
  - создание сетевых маршрутов для сети `PodNetwork` на стороне AWS;
  - создание балансировщиков нагрузки для Service с типом LoadBalancer;
  - актуализация метаданных узлов кластера, удаление из кластера узлов, которых больше нет в AWS.
- Заказ дисков через CSI-драйвер EBS (`ebs.csi.aws.com`) и создание StorageClass для типов томов AWS, чтобы из кластера можно было заказывать PersistentVolume.
- Заказ базовой инфраструктуры и CloudPermanent-узлов с помощью [Terraform/OpenTofu-провайдера](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-aws.html#взаимодействия-модуля) `terraform-provider-aws`.
- Заказ CloudEphemeral-узлов через Machine Controller Manager (MCM).
- Автоматическое включение CNI в новых кластерах. По умолчанию используется [`cni-cilium`](/modules/cni-cilium/).
