---
title: Интеграция с облаком Deckhouse Virtualization Platform
permalink: ru/admin/integrations/virtualization/dvp/services.html
lang: ru
---

Deckhouse Kubernetes Platform интегрируется с инфраструктурой DVP и использует ресурсы [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass) для описания характеристик виртуальных машин, создаваемых в составе кластера.

Основные возможности:

- Управление ресурсами DVP через модуль `cloud-controller-manager`;
- Заказ дисков с использованием компонента CSI storage;
- Интеграция с модулем [`node-manager`](/modules/node-manager/) для поддержки DVPInstanceClass при описании [NodeGroup](/modules/node-manager/cr.html#nodegroup).

{% alert level="info" %}
Интеграция с DVP включается автоматически для всех облачных кластеров, развёрнутых в DVP.
Дополнительная настройка не требуется.
{% endalert %}
