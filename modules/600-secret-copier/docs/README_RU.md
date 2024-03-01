---
title: "Модуль secret-copier"
---

Модуль отвечает за копирование Secret во все namespace.

Модуль сам скопирует в CI Secret для пуллинга образов и заказа RBD в Ceph.

### Как работает

Модуль следит за изменениями Secret в namespace `default` с лейблом `secret-copier.deckhouse.io/enabled: ""`.
* При создании такого Secret модуль скопирует его во все namespace.
* При изменении Secret модуль скопирует его новое содержимое во все namespace.
* При удалении Secret модуль удалит его из всех namespace.
* При изменении скопированного Secret в прикладном namespace модуль перезапишет его оригинальным содержимым.
* При создании любого namespace в него копируются все Secret из namespace `default` с лейблом `secret-copier.deckhouse.io/enabled: ""`.

Кроме этого, каждую ночь модуль повторно проверяет все Secret и приводит их к состоянию в namespace `default`.

### Что нужно настроить

Чтобы все заработало, создайте в namespace `default` Secret с лейблом `secret-copier.deckhouse.io/enabled: ""`.

### Как ограничить список namespace, в которые модуль копирует Secret

Задайте label–селектор в значении аннотации `secret-copier.deckhouse.io/target-namespace-selector`. Например: `secret-copier.deckhouse.io/target-namespace-selector: "app=custom"`. Модуль создаст копию этого Secret во всех namespace, соответствующих этому label–селектору.
