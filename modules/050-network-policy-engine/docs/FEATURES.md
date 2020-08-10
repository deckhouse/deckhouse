---
title: "Управление сетевыми политиками"
---

В Deckhouse был выбран консервативный подход к организации сети, при котором используются простые сетевые бекенды (*“чистый”* CNI или flannel в режиме `host-gw`). Этот подход оказался прост и надежен, поэтому показал себя с лучшей стороны.

Имплементация сетевых политик (`NetworkPolicy`) в Deckhouse тоже представляет простую и надежную систему, основанную на базе `kube-router` в режиме *Network Policy Controller* (`--run-firewall`). В этом случае `kube-router` транслирует сетевые политики `NetworkPolicy` в правила `iptables`, а они, в свою очередь, работают в любых инсталляциях, вне зависимости от облака или используемого CNI.

`Kube-router` поддерживает следующие форматы описания политик:
- *networking.k8s.io/NetworkPolicy API*
- *network policy V1/GA semantics*
- *network policy beta semantics*

Подробная [документация]({{site.baseurl}}/modules/050-network-policy-engine/) и [примеры](https://github.com/ahmetb/kubernetes-network-policy-recipes). 
