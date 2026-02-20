---
title: Подсистема Deckhouse
permalink: ru/architecture/deckhouse/
lang: ru
search: подсистема Deckhouse, контроллер Deckhouse
description: Общие сведения о подсистеме Deckhouse платформы Deckhouse Kubernetes Platform.
---

В данном подразделе описана архитектура контроллера Deckhouse и сопутствующих модулей, входящих в подсистему Deckhouse платформы Deckhouse Kubernetes Platform (DKP).

В подсистему Deckhouse входят следующие модули:

* [`deckhouse`](/modules/deckhouse/) — контроллер Deckhouse;
* [`console`](/modules/console/stable/) — веб-интерфейс Deckhouse;
* [`deckhouse-tools`](/modules/deckhouse-tools/) — создает веб-интерфейс для скачивания CLI-утилиты [`d8`](/products/kubernetes-platform/documentation/v1/cli/d8/);
* [`documentation`](/modules/documentation/) — создает веб-интерфейс с документацией, соответствующей запущенной версии DKP;
* [`registry`](/modules/registry/) — управляет конфигурацией компонентов DKP, отвечающих за работу с хранилищем образов контейнеров, и предоставляет внутреннее хранилище образов.

В данный момент в подразделе описана [архитектура режима Direct](../registry-direct-mode.html) модуля `registry`.
