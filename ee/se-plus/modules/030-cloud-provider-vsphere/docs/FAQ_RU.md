---
title: "Cloud provider — VMware vSphere: FAQ"
---

## Как поднять гибридный кластер?

Гибридный кластер представляет собой объединенные в один кластер bare-metal-узлы и узлы vSphere. Для создания такого кластера необходимо наличие L2-сети между всеми узлами кластера.

{% alert level="info" %}
В Deckhouse Kubernetes Platform есть возможность задавать префикс для имени CloudEphemeral-узлов, добавляемых в гибридный кластер c master-узлами типа Static.
Для этого используйте параметр [`instancePrefix`](../node-manager/configuration.html#parameters-instanceprefix) модуля `node-manager`. Префикс, указанный в параметре, будет добавляться к имени всех добавляемых в кластер узлов типа CloudEphemeral. Задать префикс для определенной NodeGroup нельзя.
{% endalert %}

Чтобы поднять гибридный кластер, необходимо:

1. Удалить flannel из kube-system: `d8 k -n kube-system delete ds flannel-ds`.
2. Включить модуль и прописать ему необходимые для работы [параметры](configuration.html#параметры).

> **Важно!** Cloud-controller-manager синхронизирует состояние между vSphere и Kubernetes, удаляя из Kubernetes те узлы, которых нет в vSphere. В гибридном кластере такое поведение не всегда соответствует потребности, поэтому, если узел Kubernetes запущен не с параметром `--cloud-provider=external`, он автоматически игнорируется (Deckhouse прописывает `static://` на узлы в `.spec.providerID`, а cloud-controller-manager такие узлы игнорирует).
