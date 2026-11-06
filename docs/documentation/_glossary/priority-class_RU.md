---
title: Класс приоритета
description: Один из объектов PriorityClass, которые Deckhouse Platform создаёт в кластере и назначает своим компонентам. При нехватке ресурсов планировщик первыми вытесняет поды с более низким приоритетом.
lang: ru
---

Приложения используют класс приоритета через параметр `priorityClassName`. Под без него считается наименее приоритетным. Классы упорядочены от `system-node-critical` и `system-cluster-critical`, зарезервированных для критичных компонентов кластера, через `production-high`, `production-medium`, `production-low` и `staging` до `develop`, который используется по умолчанию. Дополнительные классы создаются с именем и числовым значением приоритета. Классы приоритета также задают порядок вытеснения при [перераспределении подов](glossary.html#pod-redistribution).

Подробнее в разделе [«Классы приоритета»](admin/configuration/app-scaling/pod-eviction/priority-classes.html).
