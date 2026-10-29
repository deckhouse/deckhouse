---
title: Статический узел
description: Узел на bare-metal-сервере или виртуальной машине, который создан вне автоматизации Deckhouse Platform и подключён к кластеру вручную или через CAPS. Описывается в [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом узлов Static.
lang: ru
---

Статические узлы не поддерживают автоматическое масштабирование и обновление: обновление kubelet, перезагрузки и замены выполняются вручную или внешними инструментами. Узел подключается либо запуском [bootstrap-скрипта](glossary.html#bootstrap-script) на сервере, либо автоматически через [CAPS](glossary.html#caps). В облачной среде cloud-controller-manager не управляет статическим узлом, даже если включён модуль [облачного провайдера](glossary.html#cloud-provider).

[Группа узлов](glossary.html#node-group) может объединять статические узлы с [узлами CloudStatic](glossary.html#cloud-static-node), на этом основан [гибридный кластер](glossary.html#hybrid-cluster).

Подробнее в разделе [«Добавление и управление bare-metal-узлами»](admin/configuration/platform-scaling/node/bare-metal-node.html).
