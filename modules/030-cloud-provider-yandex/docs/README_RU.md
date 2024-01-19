---
title: "Cloud provider — Yandex Cloud"
---

Взаимодействие с облачными ресурсами провайдера [Yandex Cloud](https://cloud.yandex.ru/) осуществляется с помощью модуля `cloud-provider-yandex`. Он предоставляет возможность модулю [управления узлами](../../modules/040-node-manager/) использовать ресурсы Yandex Cloud при заказе узлов для описанной [группы узлов](../../modules/040-node-manager/cr.html#nodegroup).

Функционал модуля `cloud-provider-yandex`:
- Управляет ресурсами Yandex Cloud с помощью модуля `cloud-controller-manager`:
  * Создает сетевые маршруты для сети `PodNetwork` на стороне Yandex Cloud.
  * Актуализирует метаданные Yandex Cloud Instances и Kubernetes Nodes. Удаляет из Kubernetes узлы, которых уже нет в Yandex Cloud.
- Заказывает диски в Yandex Cloud с помощью компонента `CSI storage`.
- Регистрируется в модуле [node-manager](../../modules/040-node-manager/), чтобы [YandexInstanceClass'ы](cr.html#yandexinstanceclass) можно было использовать при описании [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup).
- Включает необходимый CNI (использует [simple bridge](../../modules/035-cni-simple-bridge/)).

## Интеграция с Yandex Cloud

### Настройка групп безопасности

При создании [облачной сети](https://cloud.yandex.ru/ru/docs/vpc/concepts/network#network), Yandex Cloud создаёт [группу безопасности](https://cloud.yandex.ru/ru/docs/vpc/concepts/security-groups) по умолчанию для всех сетей, в том числе и для сети кластера Deckhouse Kubernetes Platform. Группа безопасности по умолчанию содержит правила разрешающие любой трафик в любом направлении (входящий и исходящий) и применяется для всех подсетей в рамках облачной сети, если на объект (интерфейс ВМ) явно не назначена другая группа безопасности. Вы можете изменить правила группы безопасности по умолчанию, если вам необходимо контролировать трафик в вашем кластере.

{% alert level="danger" %}
Не удаляйте правило, разрешающее любой трафик, до того как закончите настройку всех остальных правил для группы безопасности. Это может нарушить работоспособность кластера.
{% endalert %}

1. Определите облачную сеть, в которой работает кластер Deckhouse Kubernetes Platform.

    Название сети совпадает с полем `prefix` ресурса ClusterConfiguration. Его можно узнать с помощью команды:

    ```bash
    kubectl get secrets -n kube-system d8-cluster-configuration -ojson | jq -r '.data."cluster-configuration.yaml"' | base64 -d | grep prefix | cut -d: -f2
    ```

1. Откройте облачную сеть в Yandex Cloud Console. Перейдите в раздел "Группы безопасности". У вас должна отображаться одна группа безопасности с пометкой `Default`.

    ![Группа безопасности по умолчанию](../../images/030-cloud-provider-yandex/sg-ru-default.png)

1. Создайте правила согласно [инструкции Yandex Cloud](https://cloud.yandex.ru/ru/docs/managed-kubernetes/operations/connect/security-groups#rules-internal).

    ![Правила для группы безопасности](../../images/030-cloud-provider-yandex/sg-ru-rules.png)

1. Удалите правило, разрешающее любой **входящий** трафик (на скриншоте выше оно уже удалено), и сохраните изменения.

{% alert level="warning" %}
Здесь приведены общие рекомендации по настройке группы безопасности. Для более тонкой настройки/ограничения доступа обратитесь в поддержку Deckhouse.
{% endalert %}
