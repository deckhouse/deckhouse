---
title: "Cloud provider — Yandex Cloud"
---

Взаимодействие с облачными ресурсами провайдера [Yandex Cloud](https://cloud.yandex.ru/) осуществляется с помощью модуля `cloud-provider-yandex`. Он предоставляет возможность модулю [управления узлами](../../modules/040-node-manager/) использовать ресурсы Yandex Cloud при заказе узлов для описанной [группы узлов](../../modules/040-node-manager/cr.html#nodegroup).

Функционал модуля `cloud-provider-yandex`:
- Управляет ресурсами Yandex Cloud с помощью модуля `cloud-controller-manager`:
  * Создаёт сетевые маршруты для сети `PodNetwork` на стороне Yandex Cloud.
  * Актуализирует метаданные Yandex Cloud Instances и Kubernetes Nodes. Удаляет из Kubernetes узлы, которых уже нет в Yandex Cloud.
- Заказывает диски в Yandex Cloud с помощью компонента `CSI storage`.
- Регистрируется в модуле [node-manager](../../modules/040-node-manager/), чтобы [YandexInstanceClass'ы](cr.html#yandexinstanceclass) можно было использовать при описании [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup).
- Включает необходимый CNI (использует [simple bridge](../../modules/035-cni-simple-bridge/)).
