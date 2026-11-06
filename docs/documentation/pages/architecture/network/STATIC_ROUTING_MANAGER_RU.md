---
title: Модуль static-routing-manager
permalink: ru/architecture/network/static-routing-manager.html
lang: ru
search: static-routing-manager, статические маршруты, ip rule
description: Архитектура модуля static-routing-manager в Deckhouse Platform.
---

Модуль `static-routing-manager` управляет статическими маршрутами и правилами `ip rule` на узлах кластера.

Модуль работает со следующими кастомными ресурсами API-группы `network.deckhouse.io`:

- [IPRuleSet](/modules/static-routing-manager/cr.html#ipruleset) — описывает набор правил `ip rule`, которые будут созданы на узлах, подходящих под `nodeSelector`;
- [RoutingTable](/modules/static-routing-manager/cr.html#routingtable) — описывает таблицу маршрутизации и содержащиеся в ней маршруты для узлов, подходящих под `nodeSelector`.

Модуль также работает с внутренними кастомными ресурсами API-группы `internal.network.deckhouse.io`:

- SDNInternalNodeIPRuleSet — содержит набор правил `ip rule` для конкретного узла кластера;
- SDNInternalNodeRoutingTable — содержит таблицу маршрутизации для конкретного узла кластера.

Подробнее с настройками модуля и примерами его использования можно ознакомиться [в соответствующем разделе документации](/modules/static-routing-manager/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`static-routing-manager`](/modules/static-routing-manager/) на уровне 2 модели C4 и его взаимодействия с другими компонентами Deckhouse Platform изображены на следующей диаграмме:

![Архитектура модуля static-routing-manager](../../images/architecture/network/c4-l2-static-routing-manager.ru.svg)

## Компоненты модуля

Модуль состоит из одного компонента **agent** (DaemonSet) и одного контейнера **agent**.
Agent запускается на каждом узле кластера и выполняет следующие действия:

- следит за кастомными ресурсами SDNInternalNodeRoutingTable и SDNInternalNodeIPRuleSet и обновляет их статус;
- применяет статические маршруты и правила `ip rule` к таблице маршрутизации ядра узла.

Кастомные ресурсы SDNInternalNodeRoutingTable и SDNInternalNodeIPRuleSet создаются [контроллером Deckhouse](../../glossary.html#deckhouse-controller) модуля [`deckhouse`](/modules/deckhouse/) для каждого узла кластера, подходящих под `nodeSelector` ресурсов IPRuleSet и RoutingTable.

## Взаимодействия модуля

Модуль взаимодействует с **kube-apiserver**:

- управляет кастомными ресурсами SDNInternalNodeRoutingTable и SDNInternalNodeIPRuleSet;
- следит за кастомными ресурсами IPRuleSet и RoutingTable и обновляет их статус.
