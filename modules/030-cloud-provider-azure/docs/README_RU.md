---
title: "Cloud provider — Azure"
---

Взаимодействие с облачными ресурсами провайдера [Azure](https://portal.azure.com/) осуществляется с помощью модуля `cloud-provider-azure`. Он предоставляет возможность модулю [управления узлами](../../modules/040-node-manager/) использовать ресурсы Azure при заказе узлов для описанной [группы узлов](../../modules/040-node-manager/cr.html#nodegroup).

Функционал модуля `cloud-provider-azure`:
- Управляет ресурсами Azure с помощью модуля `cloud-controller-manager`:
  * Создает сетевые маршруты для сети `PodNetwork` на стороне Azure.
  * Создает LoadBalancer'ы для Service-объектов Kubernetes с типом `LoadBalancer`.
  * Актуализирует метаданные узлов кластера согласно описанным параметрам конфигурации и удаляет из кластера узлы, которых уже нет в Azure.
- Заказывает диски в Azure с помощью компонента `CSI storage`.
- Включает необходимый CNI (использует [simple bridge](../../modules/035-cni-simple-bridge/)).
- Регистрируется в модуле [`node-manager`](../../modules/040-node-manager/), чтобы [AzureInstanceClass'ы](cr.html#azureinstanceclass) можно было использовать при описании [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup).

> **Внимание!** При использовании балансировщиков нагрузки исходящий трафик также идет через них. Если ни у одного балансировщика нет правил для UDP, весь исходящий UDP-трафик блокируется, вследствие чего не работают такие утилиты, как `ntpdate` и `chrony`. Для решения проблемы необходимо самостоятельно добавить load balancing rule с любым UDP-портом к уже существующему балансировщику либо в кластере создать сервис с типом LoadBalancer с любым UDP-портом.
