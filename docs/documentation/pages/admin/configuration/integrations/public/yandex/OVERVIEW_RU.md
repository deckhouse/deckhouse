---
title: Интеграция с Yandex Cloud
permalink: ru/admin/integrations/public/yandex/overview.html
lang: ru
---

Deckhouse Platform интегрируется с [Yandex Cloud](https://cloud.yandex.ru/) и заказывает в нём узлы кластера. Параметры виртуальных машин задаются в ресурсе [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass), который указывается в группе узлов [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Возможности интеграции:

- Управление ресурсами Yandex Cloud:
  - создание сетевых маршрутов для сети `PodNetwork` на стороне Yandex Cloud;
  - создание Network Load Balancer и целевых групп для Service с типом LoadBalancer;
  - актуализация метаданных инстансов и узлов кластера, удаление из кластера узлов, которых больше нет в Yandex Cloud.
- Заказ дисков через CSI-драйвер Yandex (`yandex.csi.flant.com`) и создание StorageClass для типов дисков Yandex Cloud, чтобы из кластера можно было заказывать PersistentVolume.
- Заказ базовой инфраструктуры и CloudPermanent-узлов с помощью [Terraform/OpenTofu-провайдера](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-yandex.html#взаимодействия-модуля) `terraform-provider-yandex`.
- Заказ CloudEphemeral-узлов через Machine Controller Manager (MCM) или Cluster API (CAPI).
- Автоматическое включение CNI в новых кластерах. Начиная с Deckhouse Platform 1.76 по умолчанию используется [`cni-cilium`](/modules/cni-cilium/).
