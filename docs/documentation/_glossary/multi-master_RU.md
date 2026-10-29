---
title: Мультимастерный кластер
description: Режим работы control plane с несколькими master-узлами. kube-apiserver взаимодействует со всеми экземплярами etcd, а при недоступности локального kube-apiserver запросы перенаправляются на другие узлы.
lang: ru
---

Мультимастерный режим обеспечивает высокую доступность [control plane](glossary.html#control-plane) и масштабирование [master-узлов](glossary.html#master-node). Если в кластере больше одного master-узла, [режим HA](glossary.html#ha-mode) включается автоматически. В конфигурации с двумя master-узлами кворум etcd обеспечивает [arbiter-узел](glossary.html#arbiter-node).

Подробнее в разделе [«Масштабирование и изменение master-узлов»](admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html).
