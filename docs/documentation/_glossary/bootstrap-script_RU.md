---
title: Bootstrap-скрипт
description: Скрипт bootstrap.sh, который Deckhouse Platform создаёт для каждой [NodeGroup](/modules/node-manager/cr.html#nodegroup), чтобы подключить сервер к кластеру в качестве узла этой группы.
lang: ru
---

Скрипт хранится в секрете `manual-bootstrap-for-<NODE-GROUP>` в неймспейсе `d8-cloud-instance-manager` и доступен в веб-интерфейсе Deckhouse. Для ручного подключения [статического узла](glossary.html#static-node) скопируйте скрипт на сервер и выполните от `root`. [CAPS](glossary.html#caps) запускает его по SSH самостоятельно. Скрипт устанавливает и настраивает параметры операционной системы, containerd, kubelet и компонент `kubernetes-api-proxy`, затем регистрирует узел в кластере.

Подробнее в разделе [«Добавление и управление bare-metal-узлами»](admin/configuration/platform-scaling/node/bare-metal-node.html).
