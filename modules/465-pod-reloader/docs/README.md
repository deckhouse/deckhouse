---
title: "The pod-reloader module"
---

The module utilizes [Reloader](https://github.com/stakater/Reloader).
It provides the ability for automatic rollout on ConfigMap or Secret changes.
The module uses annotations for operating. The module is running on **system** nodes.

{% alert level="info" %}
Reloader does not have HighAvailability mode.
{% endalert %}

All annotations are described here. You can find examples in the [Examples](examples.html) section of the documentation.

| Annotation                                   | Resource                           | Description                                                                                                  | Acceptable values                             |
| -------------------------------------------- |------------------------------------| ------------------------------------------------------------------------------------------------------------ | --------------------------------------------- |
| `pod-reloader.deckhouse.io/auto` | Deployment, DaemonSet, StatefulSet | Changes to associated (mounted or used as environment variables) ConfigMap or Secret will cause a restart of this controller's pods | `"true"`, `"false"` |
| `pod-reloader.deckhouse.io/search` | Deployment, DaemonSet, StatefulSet | If this annotation is present, a restart will only occur when ConfigMaps or Secrets with the annotation `pod-reloader.deckhouse.io/match: "true"` change | `"true"`, `"false"` |
| `pod-reloader.deckhouse.io/configmap-reload` | Deployment, DaemonSet, StatefulSet | Specifying a list of ConfigMaps that the controller depends on | `"some-cm"`, `"some-cm1,some-cm2"` |
| `pod-reloader.deckhouse.io/secret-reload` | Deployment, DaemonSet, StatefulSet | Specifying a list of secrets that the controller depends on | `"some-secret"`, `"some-secret1,some-secret2"` |
| `pod-reloader.deckhouse.io/match` | Secret, ConfigMap | Annotation by which related resources are selected to track changes | `"true"`, `"false"` |

**Important** Annotation `pod-reloader.deckhouse.io/search` cannot be used together with `pod-reloader.deckhouse.io/auto: "true"` because Reloader will ignore `pod-reloader.deckhouse.io/search` and `pod-reloader.deckhouse.io/match`. For the right behavior set `pod-reloader.deckhouse.io/auto` to `"false"` or delete it.

**Important** Annotations `pod-reloader.deckhouse.io/configmap-reload` and `pod-reloader.deckhouse.io/secret-reload` cannot be used together with `pod-reloader.deckhouse.io/auto: "true"` because Reloader will ignore `pod-reloader.deckhouse.io/configmap-reload` and `pod-reloader.deckhouse.io/secret-reload`. For the right behavior set `pod-reloader.deckhouse.io/auto` to `"false"` or delete it.
