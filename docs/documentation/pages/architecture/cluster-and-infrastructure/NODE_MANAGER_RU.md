---
title: Модуль node-manager
permalink: ru/architecture/cluster-and-infrastructure/node-management/node-manager.html
lang: ru
search: архитектура node-manager
description: Архитектура модуля node-manager в Deckhouse Kubernetes Platform.
---

Управление узлами кластера осуществляется с помощью модуля `node-manager`.

Подробнее с описанием функций и настроек модуля, а также примерами его использования можно ознакомиться в [соответствующем разделе документации](/modules/node-manager/).

## Архитектура модуля

В зависимости от типа узлов архитектура модуля отличается по составу компонентов. На следующих страницах описана архитектура модуля для различных типов узлов:

* [Управление CloudEphemeral-узлами](cloud-ephemeral-nodes.html)
* [Управление CloudPermanent-узлами](cloud-permanent-nodes.html)
* [Управление CloudStatic-узлами](cloud-static-nodes.html)
* [Управление Static-узлами](static-nodes.html)
* [Управление гибридными группами и кластерами](hybrid-nodegroups-and-clusters.html)
