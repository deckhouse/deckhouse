---
title: "Cloud provider — GCP"
---

Взаимодействие с облачными ресурсами провайдера [Google](https://cloud.google.com/) осуществляется с помощью модуля `cloud-provider-gcp`. Он предоставляет возможность модулю [управления узлами](../../modules/040-node-manager/) использовать ресурсы GCP при заказе узлов для описанной [группы узлов](../../modules/040-node-manager/cr.html#nodegroup).

Функционал модуля `cloud-provider-gcp`:
- Управляет ресурсами GCP с помощью модуля `cloud-controller-manager`:
  * Создаёт сетевые маршруты для сети `PodNetwork` на стороне GCP.
  * Создаёт LoadBalancer'ы для Service-объектов Kubernetes с типом `LoadBalancer`.
  * Актуализирует метаданные узлов кластера согласно описанным параметрам конфигурации. Удаляет из кластера узлы, которых уже нет в GCP.
- Заказывает диски в GCP с помощью компонента `CSI storage`.
- Включает необходимый CNI (использует [simple bridge](../../modules/035-cni-simple-bridge/)).
- Регистрируется в модуле [node-manager](../../modules/040-node-manager/), чтобы [GCPInstanceClass'ы](cr.html#gcpinstanceclass) можно было использовать при описании [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup).

***Внимание!*** Начиная с версии Kubernetes 1.23, для корректной работы балансировщиков нагрузки на узлы необходимо добавить [аннотацию](../021-kube-proxy/docs/README_RU.md), разрешающую kube-proxy принимать подключения  на внешние IP-адреса. Это необходимо, поскольку healthcheck балансировщиков нагрузки использует внешний адрес балансировщика.
