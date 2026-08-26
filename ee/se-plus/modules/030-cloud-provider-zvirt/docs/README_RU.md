---
title: "Cloud provider — zVirt"
description: "Управление облачными ресурсами в Deckhouse Kubernetes Platform с помощью zVirt."
---

Модуль `cloud-provider-zvirt` обеспечивает интеграцию Deckhouse Kubernetes Platform с [zVirt](https://www.zvirt.ru/). Он предоставляет возможность модулю [`node-manager`](/modules/node-manager/) использовать ресурсы zVirt при заказе узлов для [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Функции модуля `cloud-provider-zvirt`:

- Управление ресурсами zVirt через `cloud-controller-manager`: актуализирует метаданные виртуальных машин и узлов Kubernetes и удаляет из Kubernetes узлы, которых больше нет в zVirt.
- Заказ дисков через CSI-драйвер zVirt (`csi.ovirt.org`), чтобы из кластера можно было заказывать PersistentVolume.
- Заказ CloudPermanent-узлов с помощью [Terraform/OpenTofu-провайдера](/products/kubernetes-platform/documentation/v1/architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-zvirt.html#взаимодействия-модуля) `terraform-provider-ovirt/ovirt`.
- Заказ CloudEphemeral-узлов через Cluster API (CAPI). Параметры виртуальных машин задаются в ресурсе [ZvirtInstanceClass](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass).
- Регистрация в модуле [`node-manager`](/modules/node-manager/), чтобы [ZvirtInstanceClass](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass) можно было указывать при описании [NodeGroup](/modules/node-manager/cr.html#nodegroup).
- Автоматическое включение CNI для новых кластеров. По умолчанию используется [`cni-cilium`](/modules/cni-cilium/).
