---
title: Fencing
description: Механизм, который изолирует узел, потерявший связь с кластером. Узел выключается, а при необходимости удаляется его объект Node, чтобы на нём не осталось работающей нагрузки в неопределённом состоянии.
lang: ru
---

Fencing работает в два этапа: сначала узел помечается как подозрительный на уровне gossip-протокола, затем изолируется. Механизм включается для [группы узлов](glossary.html#node-group) параметром `spec.fencing.mode` в [NodeGroup](/modules/node-manager/cr.html#nodegroup). Fencing необходим для [ColdStandby](glossary.html#coldstandby): без него недоступная виртуальная машина остаётся на отказавшем узле и возобновляет работу вместе с ним.

Подробнее в разделе [«Надёжность виртуальных машин»](admin/configuration/high-reliability-and-availability/vm-reliability.html).
