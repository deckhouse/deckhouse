---
title: Интеграция с Basis Dynamix
permalink: ru/admin/integrations/private/dynamix/overview.html
lang: ru
---

Deckhouse Platform интегрируется с платформой [Basis Dynamix](https://basistech.ru/products/basis-dynamix/) и заказывает в ней узлы кластера. Параметры виртуальных машин задаются в ресурсе [DynamixInstanceClass](/modules/cloud-provider-dynamix/cr.html#dynamixinstanceclass), который указывается в группе узлов [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Возможности интеграции:

- Управление ресурсами Dynamix:
  - актуализация метаданных виртуальных машин и узлов кластера, удаление из кластера узлов, которых больше нет в Dynamix;
  - создание балансировщиков нагрузки для Service с типом LoadBalancer.
- Заказ дисков через CSI-драйвер Dynamix (`dynamix.deckhouse.io`), чтобы из кластера можно было заказывать PersistentVolume.
- Заказ CloudPermanent-узлов с помощью [Terraform/OpenTofu-провайдера](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-dynamix.html#взаимодействия-модуля) `terraform-provider-decort`.
- Заказ CloudEphemeral-узлов через Cluster API (CAPI).
- Автоматическое включение CNI в новых кластерах. По умолчанию используется [`cni-cilium`](/modules/cni-cilium/).

{% alert level="info" %}
Интеграция находится на стадии Preview.
{% endalert %}
