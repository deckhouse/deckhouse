---
title: "The pod-reloader module"
---

The module utilizes [Reloader](https://github.com/stakater/Reloader).
It provides the ability for automatic rollout on ConfigMap or Secret changes.
The module uses annotations for operating. The module is running on **system** nodes.

> **Note!** Reloader does not have HighAvailability mode.

All annotations are described here. You can find examples in the [Examples](examples.html) section of the documentation.

| Annotation                                   | Resource                           | Description                                                                                                  | Acceptable values                             |
| -------------------------------------------- |------------------------------------| ------------------------------------------------------------------------------------------------------------ | --------------------------------------------- |
| `pod-reloader.deckhouse.io/auto`             | Deployment, DaemonSet, StatefulSet | Changes in any attached resource, ConfigMap or Secret, cause controller's pod restart.                      | `"true"`, `"false"`                            |
| `pod-reloader.deckhouse.io/search`           | Deployment, DaemonSet, StatefulSet | Only resources, ConfigMap or Secret, with annotation `pod-reloader.deckhouse.io/match: "true"` cause restart | `"true"`, `"false"`                            |
| `pod-reloader.deckhouse.io/configmap-reload` | Deployment, DaemonSet, StatefulSet | List of ConfigMaps which should cause controller restart.                                                    | `"some-cm"`, `"some-cm1,some-cm2"`             |
| `pod-reloader.deckhouse.io/secret-reload`    | Deployment, DaemonSet, StatefulSet | List of Secrets which should cause controller restart.                                                       | `"some-secret"`, `"some-secret1,some-secret2"` |
| `pod-reloader.deckhouse.io/match`            | Secret, Configmap                  | Annotation mark resources for resources which should cause restart                                           | `"true"`, `"false"`                            |

**Important** Annotation `pod-reloader.deckhouse.io/search` cannot be used together with `pod-reloader.deckhouse.io/auto: "true"` because Reloader will ignore `pod-reloader.deckhouse.io/search` and `pod-reloader.deckhouse.io/match`. For the right behavior set `pod-reloader.deckhouse.io/auto` to `"false"` or delete it.

**Important** Annotations `pod-reloader.deckhouse.io/configmap-reload` and `pod-reloader.deckhouse.io/secret-reload` cannot be used together with `pod-reloader.deckhouse.io/auto: "true"` because Reloader will ignore `pod-reloader.deckhouse.io/configmap-reload` and `pod-reloader.deckhouse.io/secret-reload`. For the right behavior set `pod-reloader.deckhouse.io/auto` to `"false"` or delete it.
