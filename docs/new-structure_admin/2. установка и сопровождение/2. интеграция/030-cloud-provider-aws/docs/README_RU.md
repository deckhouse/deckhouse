---
title: "Cloud provider — AWS"
---

Взаимодействие с облачными ресурсами провайдера [AWS](https://aws.amazon.com/) осуществляется с помощью модуля `cloud-provider-aws`. Он предоставляет возможность модулю [управления узлами](../../modules/040-node-manager/) использовать ресурсы AWS при заказе узлов для описанной [группы узлов](../../modules/040-node-manager/cr.html#nodegroup).

Функционал модуля `cloud-provider-aws`:
- Управляет ресурсами AWS с помощью модуля `cloud-controller-manager`:
  * Создает сетевые маршруты для сети `PodNetwork` на стороне AWS.
  * Создает LoadBalancer'ы для Service-объектов Kubernetes с типом `LoadBalancer`.
  * Актуализирует метаданные узлов кластера согласно описанным параметрам конфигурации и удаляет из кластера узлы, которых более нет в AWS.
- Заказывает диски в AWS с помощью компонента `CSI storage`.
- Включает необходимый CNI (использует [simple bridge](../../modules/035-cni-simple-bridge/)).
- Регистрируется в модуле [node-manager](../../modules/040-node-manager/), чтобы [AWSInstanceClass'ы](cr.html#awsinstanceclass) можно было использовать при описании [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup).
