---
title: "Cloud provider — Yandex Cloud"
description: "Управление облачными ресурсами в Deckhouse Kubernetes Platform с помощью Yandex Cloud."
---

Взаимодействие с облачными ресурсами провайдера [Yandex Cloud](https://cloud.yandex.ru/) осуществляется с помощью модуля `cloud-provider-yandex`. Он предоставляет возможность модулю [управления узлами](../../modules/node-manager/) использовать ресурсы Yandex Cloud при заказе узлов для описанной [группы узлов](../../modules/node-manager/cr.html#nodegroup).

Функции модуля `cloud-provider-yandex`:

- Управление ресурсами Yandex Cloud с помощью модуля `cloud-controller-manager`:
  - Создание сетевых маршрутов для сети `PodNetwork` на стороне Yandex Cloud.
  - Актуализация метаданных Yandex Cloud Instances и Kubernetes Nodes. Удаление из Kubernetes узлов, которых уже нет в Yandex Cloud.
- Заказ дисков в Yandex Cloud с помощью компонента `CSI storage`.
- Регистрация в модуле [node-manager](../../modules/node-manager/) для использования [YandexInstanceClass'ы](cr.html#yandexinstanceclass) можно было использовать при описании [NodeGroup](../../modules/node-manager/cr.html#nodegroup).
- Включение необходимого CNI (который использует [simple bridge](../../modules/cni-simple-bridge/)).
