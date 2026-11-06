---
title: Сетевая политика
description: Правила, которые ограничивают сетевое взаимодействие подов друг с другом, с внешними системами и с узлами кластера.
lang: ru
---

Реализация сетевых политик в Deckhouse Platform зависит от включённого CNI-модуля. С [`cni-cilium`](/modules/cni-cilium/) политики применяет Cilium в трёх форматах: стандартный NetworkPolicy на уровнях L3 и L4, сетевые политики Cilium с правилами L3–L7 в пределах неймспейса и CiliumClusterwideNetworkPolicy для всего кластера, который также защищает узлы как [host firewall](glossary.html#host-firewall). С [`cni-flannel`](/modules/cni-flannel/) или другим CNI без поддержки политик стандартный NetworkPolicy на уровнях L3 и L4 применяет модуль [`network-policy-engine`](/modules/network-policy-engine/) через правила `iptables` и `ipset` на каждом узле.

Подробнее в разделе [«Сетевые политики»](admin/configuration/network/policy/configuration.html).
