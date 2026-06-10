---
title: "Cloud provider — Azure"
description: "Управление облачными ресурсами в Deckhouse Kubernetes Platform с помощью Microsoft Azure."
---

Взаимодействие с облачными ресурсами провайдера [Azure](https://portal.azure.com/) осуществляется с помощью модуля `cloud-provider-azure`. Он предоставляет возможность модулю [управления узлами](/modules/node-manager/) использовать ресурсы Azure при заказе узлов для описанной [группы узлов](/modules/node-manager/cr.html#nodegroup).

Функции модуля `cloud-provider-azure`:

- Управляет ресурсами Azure с помощью модуля `cloud-controller-manager`:
  - Создает сетевые маршруты для сети `PodNetwork` на стороне Azure.
  - Создает LoadBalancer'ы для Service-объектов Kubernetes с типом `LoadBalancer`.
  - Актуализирует метаданные узлов кластера согласно описанным параметрам конфигурации и удаляет из кластера узлы, которых уже нет в Azure.
- Заказывает диски в Azure с помощью компонента `CSI storage`.
- Включает необходимый CNI (использует [`cni-cilium`](/modules/cni-cilium/)).
- Регистрируется в модуле [`node-manager`](/modules/node-manager/), чтобы [AzureInstanceClass'ы](cr.html#azureinstanceclass) можно было использовать при описании [NodeGroup](/modules/node-manager/cr.html#nodegroup).

{% alert level="warning" %}
Начиная с DKP 1.77, для новых кластеров в Azure по умолчанию используется CNI `cilium`. В существующих кластерах текущая конфигурация CNI сохраняется.

Для новых кластеров на всех узлах требуется ядро Linux версии 5.8 или новее. Также убедитесь, что правила межсетевого экрана разрешают межузловой UDP-трафик, необходимый для работы Cilium VXLAN.

Подробнее в разделах [«Требования к установке»](/products/kubernetes-platform/documentation/v1/installing/), [«Сетевое взаимодействие компонентов платформы»](/products/kubernetes-platform/documentation/v1/reference/network_interaction.html) и [документации модуля `cni-cilium`](/modules/cni-cilium/).
{% endalert %}

{% alert level="warning" %}
Для корректной работы утилит, таких как `ntpdate` и `chrony`, при использовании балансировщиков нагрузки важно убедиться, что у балансировщика есть соответствующие правила для обработки UDP-трафика. В случае блокировки исходящего UDP-трафика, можно добавить новое правило к существующему балансировщику или создать новый сервис с типом LoadBalancer и UDP-портом, чтобы обеспечить правильную маршрутизацию UDP-запросов.
{% endalert %}
