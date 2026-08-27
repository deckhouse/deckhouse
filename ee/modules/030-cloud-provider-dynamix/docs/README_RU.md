---
title: "Cloud provider — Basis Dynamix"
description: "Управление облачными ресурсами в Deckhouse Kubernetes Platform с помощью Базис.DynamiX."
---

Модуль `cloud-provider-dynamix` обеспечивает интеграцию Deckhouse Kubernetes Platform с платформой [Базис.DynamiX](https://basistech.ru/products/basis-dynamix/). Он предоставляет возможность модулю [`node-manager`](/modules/node-manager/) использовать ресурсы Dynamix при заказе узлов для [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Функции модуля `cloud-provider-dynamix`:

- Управление ресурсами Dynamix через `cloud-controller-manager`:
  - актуализирует метаданные виртуальных машин и узлов Kubernetes и удаляет из Kubernetes узлы, которых больше нет в Dynamix;
  - создаёт балансировщики нагрузки для Service с типом LoadBalancer. Для этого на Service нужно указать аннотации с именами внутренней и внешней сети.
- Заказ дисков через CSI-драйвер Dynamix (`dynamix.deckhouse.io`), чтобы из кластера можно было заказывать PersistentVolume.
- Заказ CloudPermanent-узлов с помощью [Terraform/OpenTofu-провайдера](/products/kubernetes-platform/documentation/v1/architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-dynamix.html#взаимодействия-модуля) `terraform-provider-decort`.
- Заказ CloudEphemeral-узлов через Cluster API (CAPI). Параметры виртуальных машин задаются в ресурсе [DynamixInstanceClass](/modules/cloud-provider-dynamix/cr.html#dynamixinstanceclass).
- Регистрация в модуле [`node-manager`](/modules/node-manager/), чтобы [DynamixInstanceClass](/modules/cloud-provider-dynamix/cr.html#dynamixinstanceclass) можно было указывать при описании [NodeGroup](/modules/node-manager/cr.html#nodegroup).
- Автоматическое включение CNI для новых кластеров. По умолчанию используется [`cni-cilium`](/modules/cni-cilium/).

{% alert level="info" %}
Модуль находится на стадии Preview.
{% endalert %}
