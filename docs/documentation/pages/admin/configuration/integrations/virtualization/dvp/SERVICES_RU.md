---
title: Интеграция с облаком DVP
permalink: ru/admin/integrations/virtualization/dvp/services.html
lang: ru
---

Deckhouse Kubernetes Platform интегрируется с инфраструктурой DVP и использует ресурсы [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass) для описания характеристик виртуальных машин, создаваемых в составе кластера.

Основные возможности:

- Управление ресурсами DVP через модуль `cloud-controller-manager`;
- Заказ дисков с использованием компонента CSI storage;
- Интеграция с модулем [`node-manager`](/modules/node-manager/) для поддержки DVPInstanceClass при описании [NodeGroup](/modules/node-manager/cr.html#nodegroup).

{% alert level="info" %}
Модуль автоматически включается для всех облачных кластеров, развернутых в DVP.
Модуль не имеет настроек.
{% endalert %}
