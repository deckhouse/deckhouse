---
title: Cluster API Provider Static (CAPS)
description: Механизм, который автоматически подключает, настраивает и при необходимости очищает статические узлы на основе объектов [StaticInstance](/modules/node-manager/cr.html#staticinstance) и [SSHCredentials](/modules/node-manager/cr.html#sshcredentials).
lang: ru
---

CAPS включается секцией `staticInstances` в [группе узлов](glossary.html#node-group) с типом Static: `count` задаёт количество узлов в группе, `labelSelector` выбирает используемые StaticInstance. CAPS поддерживает количество узлов равным `count`: подключается к серверу по SSH, выполняет [bootstrap-скрипт](glossary.html#bootstrap-script) и добавляет узел в кластер, а при удалении узла очищает сервер и возвращает его StaticInstance в состояние `Pending` для повторного использования.

Подробнее в разделе [«Добавление и управление bare-metal-узлами»](admin/configuration/platform-scaling/node/bare-metal-node.html).
