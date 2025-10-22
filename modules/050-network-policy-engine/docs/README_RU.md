---
title: "Модуль network-policy-engine"
description: Управление сетевыми политиками в кластере Deckhouse Kubernetes Platform.
---

{% alert level="warning" %}
Не используйте модуль, если включен модуль <a href="../cni-cilium/">cilium</a>, так как в нем уже есть функционал управления сетевыми политиками.
{% endalert %}

Модуль управления сетевыми политиками.

В Deckhouse выбран консервативный подход к организации сети, при котором используются простые сетевые бэкенды (*«чистый»* CNI или flannel в режиме `host-gw`). Этот подход прост, надежен и показал себя с лучшей стороны.

Имплементация сетевых политик (`NetworkPolicy`) в Deckhouse также является простой и надежной системой, основанной на базе `kube-router` в режиме *Network Policy Controller* (`--run-firewall`). В этом случае `kube-router` транслирует сетевые политики `NetworkPolicy` в правила `iptables`, которые работают в любых инсталляциях вне зависимости от облака или используемого CNI.

Модуль `network-policy-engine` разворачивает в namespace `d8-system` DaemonSet с [kube-router](https://github.com/cloudnativelabs/kube-router) в режиме поддержки [network policies](https://kubernetes.io/docs/concepts/services-networking/network-policies/). В результате в Kubernetes-кластере включается полная поддержка Network Policies.

Поддерживаются следующие форматы описания политик:

- *networking.k8s.io/NetworkPolicy API;*
- *network policy V1/GA semantics;*
- *network policy beta semantics.*

[Примеры](https://github.com/ahmetb/kubernetes-network-policy-recipes) использования.
