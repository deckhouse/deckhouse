---
title: "The pod-reloader module"
---

The module utilizes [Reloader](https://github.com/stakater/Reloader).
It provides the ability for automatic rollout on ConfigMap or Secret changes.
The module uses annotations for operating. The module is running on **system** nodes.

**Important** Reloader does not have HighAvailability mode.

All annotations are described here. Examples of usage can be found [here](usage.html).

| Annotation                                   | Resource                           | Description                                                                                                  | Acceptable values                             |
| -------------------------------------------- | ---------------------------------- | ------------------------------------------------------------------------------------------------------------ | --------------------------------------------- |
| `pod-reloader.deckhouse.io/auto`             | Deployment, Daemonset, Statefulset | Changes in any attachecd resource, ConfigMap or Secret, cause controller's pod restart.                      | `"true"`, `"false"`                            |
| `pod-reloader.deckhouse.io/search`           | Deployment, Daemonset, Statefulset | Only resources, ConfigMap or Secret, with annotation `pod-reloader.deckhouse.io/match: "true"` cause restart | `"true"`, `"false"`                            |
| `pod-reloader.deckhouse.io/configmap-reload` | Deployment, Daemonset, Statefulset | List of ConfigMaps which should cause controller restart.                                                    | `"some-cm"`, `"some-cm1,some-cm2"`             |
| `pod-reloader.deckhouse.io/secret-reload`    | Deployment, Daemonset, Statefulset | List of Secrets which should cause controller restart.                                                       | `"some-secret"`, `"some-secret1,some-secret2"` |
| `pod-reloader.deckhouse.io/match`            | Secret, Configmap                  | Annotation mark resources for resources which should cause restart                                           | `"true"`, `"false"`                            |

**Impotant** Annotation `pod-reloader.deckhouse.io/search` cannot be used together with `pod-reloader.deckhouse.io/auto: "true"` because Reloader will ignore `pod-reloader.deckhouse.io/search` and `pod-reloader.deckhouse.io/match`. For the right behavior set `pod-reloader.deckhouse.io/auto` to `"false"` or delete it.

**Impotant** Annotations `pod-reloader.deckhouse.io/configmap-reload` and `pod-reloader.deckhouse.io/secret-reload` cannot be used together with `pod-reloader.deckhouse.io/auto: "true"` because Reloader will ignore `pod-reloader.deckhouse.io/configmap-reload` and `pod-reloader.deckhouse.io/secret-reload`. For the right behavior set `pod-reloader.deckhouse.io/auto` to `"false"` or delete it.
