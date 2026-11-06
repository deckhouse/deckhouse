---
title: Disruptive-обновление
description: Обновление узла, которое требует прерывания его работы и может привести к кратковременному простою системных компонентов, например обновление containerd или kubelet через несколько версий. Режим подтверждения задаётся параметром [disruptions.approvalMode](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions-approvalmode) в [NodeGroup](/modules/node-manager/cr.html#nodegroup).
lang: ru
---

Обычные обновления узлов всегда применяются автоматически и не приводят к остановке или перезагрузке узла. Секцией `disruptions` управляются только disruptive-обновления, например обновление ядра или containerd и значительная смена версии kubelet. Одновременно обновляется только один узел группы.

В [группе узлов](glossary.html#node-group) доступны три режима подтверждения. В режиме `Manual` отображается [алерт](glossary.html#alert) `NodeRequiresDisruptionApprovalForUpdate`, а обновление начинается после добавления на узел аннотации `update.node.deckhouse.io/disruption-approved=`. Узел при этом не освобождается от нагрузки автоматически. В режиме `Automatic` узел освобождается от нагрузки и обновляется без подтверждения. В режиме `RollingUpdate` создаётся новый узел с обновлёнными настройками, а старый удаляется. Этот режим применим только к облачным узлам.

Для групп [master-узлов](glossary.html#master-node) используйте режим `Manual`.

Подробнее в разделе [«Основы управления узлами»](admin/configuration/platform-scaling/node/node-management.html).
