---
title: "Cloud provider — GCP"
---

Взаимодействие с облачными ресурсами провайдера [Google](https://cloud.google.com/) осуществляется с помощью модуля `cloud-provider-gcp`. Он предоставляет возможность модулю [управления узлами](../../modules/040-node-manager/) использовать ресурсы GCP при заказе узлов для описанной [группы узлов](../../modules/040-node-manager/cr.html#nodegroup).

Функционал модуля `cloud-provider-gcp`:
- Управляет ресурсами GCP с помощью модуля `cloud-controller-manager`:
  * Создает сетевые маршруты для сети `PodNetwork` на стороне GCP.
  * Создает LoadBalancer'ы для Service-объектов Kubernetes с типом `LoadBalancer`.
  * Актуализирует метаданные узлов кластера согласно описанным параметрам конфигурации и удаляет из кластера узлы, которых уже нет в GCP.
- Заказывает диски в GCP с помощью компонента `CSI storage`.
- Включает необходимый CNI (использует [simple bridge](../../modules/035-cni-simple-bridge/)).
- Регистрируется в модуле [node-manager](../../modules/040-node-manager/), чтобы [GCPInstanceClass'ы](cr.html#gcpinstanceclass) можно было использовать при описании [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup).
