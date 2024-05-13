---
title: "Модуль network-policy-engine"
---

<div class="docs__information warning active">
Не используйте модуль, если включен модуль <a href="../021-cni-cilium/">cilium</a>, так как в нем уже есть функционал управления сетевыми политиками.
</div>

Модуль управления сетевыми политиками.

В Deckhouse был выбран консервативный подход к организации сети, при котором используются простые сетевые бекенды (*«чистый»* CNI или flannel в режиме `host-gw`). Этот подход оказался прост и надежен, поэтому показал себя с лучшей стороны.

Имплементация сетевых политик (`NetworkPolicy`) в Deckhouse тоже представляет собой простую и надежную систему, основанную на базе `kube-router` в режиме *Network Policy Controller* (`--run-firewall`). В этом случае `kube-router` транслирует сетевые политики `NetworkPolicy` в правила `iptables`, а они, в свою очередь, работают в любых инсталляциях вне зависимости от облака или используемого CNI.

Модуль `network-policy-engine` разворачивает в namespace `d8-system` DaemonSet с [kube-router](https://github.com/cloudnativelabs/kube-router) в режиме поддержки [network policies](https://kubernetes.io/docs/concepts/services-networking/network-policies/). В результате в Kubernetes-кластере включается полная поддержка Network Policies.

Поддерживаются следующие форматы описания политик:
- *networking.k8s.io/NetworkPolicy API;*
- *network policy V1/GA semantics;*
- *network policy beta semantics.*

[Примеры](https://github.com/ahmetb/kubernetes-network-policy-recipes) использования.
