---
title: Обзор
permalink: ru/architecture/deckhouse/
lang: ru
search: подсистема deckhouse, deckhouse subsystem, deckhouse controller, контроллер deckhouse, container registry, registry
---

В данном подразделе будет описана архитектура **Deckhouse Controller** и сопутствующих модулей, входящих в подсистему **Deckhouse** DKP.

В подсистему **Deckhouse** входят следующие модули:

* [deckhouse](/modules/deckhouse/) - это, собственно, сам контроллер Deckhouse,
* [console](/modules/console/stable/) - веб-интерфейс платформы Deckhouse Kubernetes,
* [deckhouse-tools](/modules/deckhouse-tools/) - создает веб-интерфейс для скачивания из кластера утилиты [Deckhouse CLI](/products/kubernetes-platform/documentation/v1/cli/d8/)  под различные операционные системы,
* [documentation](/modules/documentation/) - создает веб-интерфейс с документацией, соответствующей запущенной версии Deckhouse Kubernetes Platform,
* [registry](/modules/registry/) - отвечает за управление конфигурацией registry компонентов Deckhouse и предоставляет внутреннее хранилище образов контейнеров (container registry, registry).

В подразделе на данный момент описана [архитектура режима Direct](../registry-direct-mode.html) registry.
