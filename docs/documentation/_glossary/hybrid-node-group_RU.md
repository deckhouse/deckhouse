---
title: Гибридная группа узлов
description: Группа узлов, в которую входят как Static-узлы, развёрнутые в облаке, так и серверы в собственном ЦОД, bare metal или виртуальные машины.
lang: ru
---

В гибридной [группе узлов](glossary.html#node-group) основную нагрузку могут нести серверы bare metal, а облачные инстансы служат масштабируемым дополнением при пиковых нагрузках. Все узлы группы — [статические](glossary.html#static-node), поэтому модуль [`node-manager`](/modules/node-manager/) работает так же, как для статических узлов. Единственное требование — L3-связность между ЦОД и облаком. Гибридная группа узлов отличается от [гибридного кластера](glossary.html#hybrid-cluster), в котором статические узлы сочетаются с [узлами CloudEphemeral](glossary.html#cloud-ephemeral-node).

Подробнее в разделе [«Гибридные группы узлов и кластеры»](architecture/cluster-and-infrastructure/node-management/hybrid-nodegroups-and-clusters.html).
