---
title: "Cloud provider — Basis Dynamix"
description: "Управление облачными ресурсами в Deckhouse Kubernetes Platform с помощью Базис.DynamiX."
---

Модуль `cloud-provider-dynamix` обеспечивает интеграцию Deckhouse Kubernetes Platform с платформой [Базис.DynamiX](https://basistech.ru/products/basis-dynamix/). Он позволяет модулю [`node-manager`](/modules/node-manager/) использовать ресурсы Dynamix при заказе узлов для [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Функции модуля `cloud-provider-dynamix`:

- Управление ресурсами Dynamix через `cloud-controller-manager`:
  - актуализирует метаданные виртуальных машин и узлов Kubernetes и удаляет из Kubernetes узлы, которых больше нет в Dynamix;
  - создаёт балансировщики нагрузки для Service с типом LoadBalancer. Для этого на Service нужно указать аннотации с именами внутренней и внешней сети.
- Заказ дисков через CSI-драйвер Dynamix (`dynamix.deckhouse.io`), чтобы из кластера можно было заказывать PersistentVolume.
- Заказ CloudEphemeral-узлов через Cluster API (CAPI). Параметры виртуальных машин задаются в ресурсе [DynamixInstanceClass](cr.html#dynamixinstanceclass).
- Регистрация в модуле [`node-manager`](/modules/node-manager/), чтобы DynamixInstanceClass можно было указывать при описании NodeGroup.
- Автоматическое включение CNI для новых кластеров. По умолчанию используется [`cni-cilium`](/modules/cni-cilium/) в режиме `VXLAN`.

{% alert level="info" %}
Модуль находится на стадии Preview.
{% endalert %}
