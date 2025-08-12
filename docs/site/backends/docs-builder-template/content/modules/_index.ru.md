---
title: Обзор модулей
url: modules/
layout: modules
---

Deckhouse Kubernetes Platform имеет модульную структуру. Модуль может быть либо встроенным в Deckhouse, либо подключаемым (с помощью ресурса [ModuleSource](/products/kubernetes-platform/documentation/v1/cr.html#modulesource)).

Основное отличие встроенного модуля Deckhouse от подключаемого в том, что встроенный модуль поставляется в составе платформы Deckhouse и имеет общий с Deckhouse релизный цикл. Подробную информацию по встроенным модулям Deckhouse можно найти в разделе [документации Deckhouse](/products/kubernetes-platform/documentation/v1/).

Модули Deckhouse, подключаемые с помощью ресурса [ModuleSource](/products/kubernetes-platform/documentation/v1/cr.html#modulesource), имеют независимый от Deckhouse релизный цикл, и могут обновляться независимо от версий Deckhouse. Разработка подключаемых модулей может вестись командой разработчиков, не связанной с командой разработки Deckhouse.

Определить, является ли модуль встроенным или подключаемым, можно по значению поля `SOURCE` в выводе команды `kubectl get modules`. Для встроенных модулей в этом поле указано `Embedded`, для подключаемых — имя объекта [ModuleSource](/products/kubernetes-platform/documentation/v1/cr.html#modulesource) (источник модулей, из которого производится установка модуля).

Пример:

```console
$ kubectl get modules
NAME                STAGE   SOURCE      PHASE        ENABLED   READY
cni-cilium                  Embedded    Ready        True      True
commander                   deckhouse   Available    False     False
```

В данном разделе представлена информация по модулям Deckhouse, которые могут быть подключены из источника модулей. Модули прошли предварительное тестирование совместимости и допущенным к использованию совместно с Deckhouse.
