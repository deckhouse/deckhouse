---
title: Группа узлов
description: Набор узлов кластера с общими параметрами, описанный в [NodeGroup](/modules/node-manager/cr.html#nodegroup). NodeGroup задаёт тип узлов, их количество и конфигурацию, а Deckhouse Platform приводит узлы в соответствие с ней.
lang: ru
---

Группы узлов логически разделяют инфраструктуру кластера. Типичные группы: `master` для [control plane](glossary.html#control-plane), `front` для [frontend-узлов](glossary.html#frontend-node), `monitoring` для [узлов мониторинга](glossary.html#monitoring-node), `system` для [системных узлов](glossary.html#system-node) и `worker` для пользовательских приложений.

У каждой группы централизованные настройки: версия Kubernetes, ресурсы, taints, лейблы и параметры kubelet. При создании или изменении NodeGroup модуль [`node-manager`](/modules/node-manager/) приводит узлы группы к заданной конфигурации. Тип узлов один из: [Static](glossary.html#static-node), [CloudEphemeral](glossary.html#cloud-ephemeral-node), [CloudPermanent](glossary.html#cloud-permanent-node) или [CloudStatic](glossary.html#cloud-static-node).

Подробнее в разделе [«Основы управления узлами»](admin/configuration/platform-scaling/node/node-management.html).
