---
title: Arbiter-узел
description: Узел, на котором размещается только etcd, без остальных компонентов control plane, для обеспечения кворума etcd в режиме HA с двумя master-узлами.
lang: ru
---

[Режим HA](glossary.html#ha-mode) с двумя [master-узлами](glossary.html#master-node) и arbiter-узлом обеспечивает требования высокой доступности при ограниченных ресурсах. Arbiter-узел должен иметь лейбл `node.deckhouse.io/etcd-arbiter: ""` и taint, который не допускает на него пользовательскую нагрузку. Ему нужны не менее 2 ядер CPU, 4 ГБ оперативной памяти и 8 ГБ дискового пространства для etcd, а также такие же сетевые задержки, как у master-узлов.

Подробнее в разделе [«Управление режимом HA»](admin/configuration/high-reliability-and-availability/enable.html).
