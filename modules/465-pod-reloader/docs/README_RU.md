---
title: "Модуль pod-reloader"
---

Модуль создан на основе [Reloader](https://github.com/stakater/Reloader).
Он предоставляет возможность автоматически произвести rollout в случае изменения ConfigMap или Secret.
Для управления используются аннотации. Модуль запускается на **системных** узлах.

> **Важно!** У Reloader отсутствует отказоустойчивость.

В этом документе описаны основные аннотации. Вы можете найти больше примеров в разделе [Примеры](examples.html) документации.

| Аннотация                                    | Ресурс                             | Описание                                                                                                                                                                 | Примеры значений                              |
| -------------------------------------------- |------------------------------------| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | --------------------------------------------- |
| `pod-reloader.deckhouse.io/auto`             | Deployment, DaemonSet, StatefulSet | В случае изменения в связанных, то есть примонтированных или использованных как переменные окружения, ConfigMap'ах или Secret'ах произойдет перезапуск подов этого контроллера | `"true"`, `"false"`  |
| `pod-reloader.deckhouse.io/search`           | Deployment, DaemonSet, StatefulSet | В случае наличия этой аннотации перезапуск будет производиться исключительно при изменении ConfigMap'ов или Secret'ов с аннотацией `pod-reloader.deckhouse.io/match: "true"` | `"true"`, `"false"` |
| `pod-reloader.deckhouse.io/configmap-reload` | Deployment, DaemonSet, StatefulSet | Указать список ConfigMap'ов, от которых зависит контроллер                                                                                                                   | `"some-cm"`, `"some-cm1,some-cm2"` |
| `pod-reloader.deckhouse.io/secret-reload`    | Deployment, DaemonSet, StatefulSet | Указать список Secret'ов, от которых зависит контроллер                                                                                                                      | `"some-secret"`, `"some-secret1,some-secret2"` |
| `pod-reloader.deckhouse.io/match`            | Secret, ConfigMap                  | Аннотация, по которой из связанных ресурсов выбираются те, за изменениями которых мы следим                                                                               | `"true"`, `"false"` |

**Важно** Аннотация `pod-reloader.deckhouse.io/search` не может быть использована вместе с `pod-reloader.deckhouse.io/auto: "true"`, так как Reloader будет игнорировать `pod-reloader.deckhouse.io/search` и `pod-reloader.deckhouse.io/match`. Для корректной работы установите аннотации `pod-reloader.deckhouse.io/auto` значение `"false"` или удалите ее.

**Важно** Аннотации `pod-reloader.deckhouse.io/configmap-reload` и `pod-reloader.deckhouse.io/secret-reload` не могут быть использованы вместе с `pod-reloader.deckhouse.io/auto: "true"`, так как Reloader будет игнорировать `pod-reloader.deckhouse.io/search` и `pod-reloader.deckhouse.io/match`. Для корректной работы установите аннотации `pod-reloader.deckhouse.io/auto` значение `"false"` или удалите ее.
