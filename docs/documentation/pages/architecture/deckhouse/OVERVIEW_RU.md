---
title: Подсистема Deckhouse
permalink: ru/architecture/deckhouse/
lang: ru
search: подсистема Deckhouse, контроллер Deckhouse
description: Архитектура подсистемы Deckhouse в Deckhouse Platform.
extractedLinksOnlyMax: 0
extractedLinksMax: 0
---

В данном подразделе описана архитектура модулей, входящих в подсистему Deckhouse платформы.

В подсистему Deckhouse входят следующие модули:

* [`deckhouse`](/modules/deckhouse/) — контроллер Deckhouse;
* [`console`](/modules/console/stable/) — веб-интерфейс Deckhouse;
* [`deckhouse-tools`](/modules/deckhouse-tools/) — создает веб-интерфейс для скачивания CLI-утилиты [`d8`](/products/kubernetes-platform/documentation/v1/cli/d8/);
* [`documentation`](/modules/documentation/) — создает веб-интерфейс с документацией, соответствующей запущенной версии DP;
* [`registry`](/modules/registry/) — управляет конфигурацией компонентов DP, отвечающих за работу с хранилищем образов контейнеров, и предоставляет внутреннее хранилище образов.
