---
title: Защищённые параметры
description: Параметры кластера, критичные для его работы, которые по умолчанию нельзя изменить в работающем кластере. podSubnetCIDR, podSubnetNodeCIDRPrefix и serviceSubnetCIDR.
lang: ru
---

Защищённые параметры входят в [конфигурацию кластера](glossary.html#cluster-configuration). Попытка изменить их блокируется admission-вебхуком с сообщением об ошибке. Порядок их изменения, с помощью [установщика](glossary.html#installer) или вручную, описан в документации.

Подробнее в разделе [«Конфигурация Deckhouse Platform»](admin/configuration/).
