---
title: "Модуль secret-copier"
---

Данный модуль отвечает за копирование Secret'ов во все namespace'ы.

Нам данный модуль полезен тем, чтобы не копировать каждый раз в CI Secret'ы для пуллинга образов и заказа RBD в Ceph.

### Как работает

Данный модуль следит за изменениями Secret'ов в namespace `default` с лейблом `secret-copier.deckhouse.io/enabled: ""`.
* При создании такого Secret'а он будет скопирован во все namespace.
* При изменении Secret'а его новое содержимое будет раскопировано во все namespace.
* При удалении Secret'а он будет удален из всех namespace.
* При изменении скопированного Secret'а в прикладном namespace тот будет перезаписан оригинальным содержимым.
* При создании любого namespace в него копируются все Secret'ы из default namespace с лейблом `secret-copier.deckhouse.io/enabled: ""`.

Кроме этого, каждую ночь Secret'ы будут повторно синхронизированы и приведены к состоянию в default namespace.

### Что нужно настроить?

Чтобы все заработало, достаточно создать в default namespace Secret с лейблом `secret-copier.deckhouse.io/enabled: ""`.

### Как ограничить список namespace'ов, в которые будет производиться копирование?

Задайте label–селектор в значении аннотации `secret-copier.deckhouse.io/target-namespace-selector`. Например: `secret-copier.deckhouse.io/target-namespace-selector: "app=custom"`. Модуль создаст копию этого Secret'а во всех пространствах имен, соответствующих заданному label–селектору.
