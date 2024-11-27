---
title: "Cloud provider — VMware vSphere: FAQ"
---

## Как поднять гибридный кластер?

Гибридный кластер представляет собой объединенные в один кластер bare-metal-узлы и узлы vSphere. Для создания такого кластера
необходимо наличие L2-сети между всеми узлами кластера.

Чтобы поднять гибридный кластер, необходимо:

1. Удалить flannel из kube-system: `kubectl -n kube-system delete ds flannel-ds`.
2. Включить модуль и прописать ему необходимые для работы [параметры](configuration.html#параметры).

> **Важно!** Cloud-controller-manager синхронизирует состояние между vSphere и Kubernetes, удаляя из Kubernetes те узлы, которых нет в vSphere. В гибридном кластере такое поведение не всегда соответствует потребности, поэтому, если узел Kubernetes запущен не с параметром `--cloud-provider=external`, он автоматически игнорируется (Deckhouse прописывает `static://` на узлы в `.spec.providerID`, а cloud-controller-manager такие узлы игнорирует).
