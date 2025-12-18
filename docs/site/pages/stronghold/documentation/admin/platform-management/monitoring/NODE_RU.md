---
title: "Мониторинг узлов"
permalink: ru/stronghold/documentation/admin/platform-management/monitoring/node.html
lang: ru
---

## Мониторинг

Для групп узлов (ресурс NodeGroup) DKP экспортирует метрики доступности группы.

### Какую информацию собирает Prometheus?

Все метрики групп узлов имеют префикс `d8_node_group_` в названии, и метку с именем группы `node_group_name`.

Следующие метрики собираются для каждой группы узлов:

- `d8_node_group_ready` — количество узлов группы, находящихся в статусе `Ready`;
- `d8_node_group_nodes` — количество узлов в группе (в любом статусе);
- `d8_node_group_instances` — количество инстансов в группе (в любом статусе);
- `d8_node_group_desired` — желаемое (целевое) количество объектов `Machines` в группе;
- `d8_node_group_min` — минимальное количество инстансов в группе;
- `d8_node_group_max` — максимальное количество инстансов в группе;
- `d8_node_group_up_to_date` — количество узлов в группе в состоянии up-to-date;
- `d8_node_group_standby` — количество резервных узлов (см. параметр [`standby`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-standby) в группе);
- `d8_node_group_has_errors` — единица, если в группе узлов есть какие-либо ошибки.
