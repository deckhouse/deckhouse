---
title: Интеграция со встроенной виртуализацией
permalink: ru/admin/integrations/virtualization/dvp/overview.html
lang: ru
---

Deckhouse Platform интегрируется со [встроенной виртуализацией](../../../configuration/virtualization/) и заказывает в ней узлы кластера. Параметры виртуальных машин задаются в ресурсе [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass), который указывается в группе узлов [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Возможности интеграции:

- Управление ресурсами встроенной виртуализации, в том числе актуализация метаданных виртуальных машин и узлов кластера и удаление из кластера узлов, которых больше нет во встроенной виртуализации.
- Заказ дисков через CSI-драйвер встроенной виртуализации (`csi.dvp.deckhouse.io`), чтобы из кластера можно было заказывать PersistentVolume.
- Заказ базовой инфраструктуры и CloudPermanent-узлов с помощью [Terraform/OpenTofu-провайдера](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-dvp.html#взаимодействия-модуля) `terraform-provider-kubernetes`.
- Заказ CloudEphemeral-узлов через Cluster API (CAPI).
- Автоматическое включение CNI в новых кластерах. По умолчанию используется [`cni-cilium`](/modules/cni-cilium/).

{% alert level="info" %}
Интеграция с виртуализацией включается автоматически для всех [облачных кластеров](../../../../glossary.html#cloud-cluster), развёрнутых во встроенной виртуализации.
Дополнительная настройка не требуется.
{% endalert %}
