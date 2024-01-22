---
title: "Cloud provider — VMware vSphere: FAQ"
---

## Как поднять гибридный кластер?

Гибридный кластер представляет собой объединенные в один кластер bare-metal-узлы и узлы vSphere. Для создания кластера
необходимы L2-сети между всеми узлами кластера.

Чтобы поднять гибридный кластер, необходимо:

1. Удалить flannel из kube-system: `kubectl -n kube-system delete ds flannel-ds`.
2. Включить модуль и прописать необходимые для работы [параметры](configuration.html#параметры).

> **Важно!** Cloud-controller-manager синхронизирует состояние между vSphere и Kubernetes, удаляя из Kubernetes узлы, которых нет в vSphere. В гибридном кластере это требуется не всегда. Поэтому, если узел Kubernetes запускается без параметра `--cloud-provider=external`, он автоматически игнорируется. Deckhouse прописывает `static://` на узлы в `.spec.providerID`, поэтому cloud-controller-manager игнорирует такие узлы.