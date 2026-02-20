---
title: Модуль node-manager
permalink: ru/architecture/cluster-and-infrastructure/node-manager/
lang: ru
search: node manager, node-manager
---

Управление узлами кластера осуществляется с помощью модуля [node-manager](/modules/node-manager/).

Подробнее с описанием функций и настроек модуля, а также примерами его использования можно ознакомиться в соответствующем [разделе документации](https://deckhouse.ru/modules/node-manager/).

## Архитектура модуля

В зависимости от типа узлов архитектура модуля отличается по составу компонентов. На следующих страницах описана архитектура модуля для типов узлов:

* [Управление CloudEphemeral-узлами](../cloud-ephemeral-nodes/)
* [Управление CloudPermanent-узлами](../cloud-permanent-nodes/)
* [Управление CloudStatic-узлами](../cloud-static-nodes/)
* [Управление Static-узлами](../static-nodes/)
* [Управление гибридными группами и кластерами](../hybrid-nodegroups-and-clusters/)
