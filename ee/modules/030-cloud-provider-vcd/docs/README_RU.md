---
title: "Cloud provider — VMware Cloud Director"
description: "Управление облачными ресурсами в Deckhouse Kubernetes Platform с помощью VMware Cloud Director."
---

Модуль `cloud-provider-vcd` обеспечивает интеграцию Deckhouse Kubernetes Platform с [VMware Cloud Director](https://www.vmware.com/products/cloud-director.html). Он предоставляет возможность модулю [`node-manager`](/modules/node-manager/) использовать ресурсы VCD при заказе узлов для [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Функции модуля `cloud-provider-vcd`:

- Управление ресурсами VCD через `cloud-controller-manager`:
  - актуализирует метаданные виртуальных машин и узлов Kubernetes и удаляет из Kubernetes узлы, которых больше нет в VCD;
  - создаёт балансировщики нагрузки для Service с типом LoadBalancer. Для этого используется VMware NSX Advanced Load Balancer (Avi); поддержка доступна при использовании NSX-T.
- Заказ дисков через CSI-драйвер Named Disk (`named-disk.csi.cloud-director.vmware.com`), чтобы из кластера можно было заказывать PersistentVolume.
- Заказ CloudPermanent-узлов с помощью [Terraform/OpenTofu-провайдера](/products/kubernetes-platform/documentation/v1/architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-vcd.html#взаимодействия-модуля) `terraform-provider-vcd`. Базовая инфраструктура заказывается в зависимости от [схемы размещения](/modules/cloud-provider-vcd/layouts.html).
- Заказ CloudEphemeral-узлов через Cluster API (CAPI). Параметры виртуальных машин задаются в ресурсе [VCDInstanceClass](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass).
- Регистрация в модуле [`node-manager`](/modules/node-manager/), чтобы [VCDInstanceClass](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass) можно было указывать при описании [NodeGroup](/modules/node-manager/cr.html#nodegroup).
- Автоматическое включение CNI для новых кластеров. По умолчанию используется [`cni-cilium`](/modules/cni-cilium/).

{% alert level="info" %}
Для работы с API VCD версии ниже 37.2 модуль использует режим совместимости (legacy-компоненты).
{% endalert %}
