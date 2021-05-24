---
title: "The okmeter module"
---

This module installs the [okmeter](http://okmeter.io) agent as a `daemonset` in the `d8-okmeter` namespace, and deletes `okmeter` installed manually.

Configuration
------------

### Enabling the module

This module is **disabled** by default. To enable it, add the following lines to the `deckhouse` ConfigMap:

```yaml
data:
  okmeterEnabled: "true"
```

### What do I need to configure?

You need to add the `apiKey` for the `okmeter` module to the Deckhouse configuration:

* `apiKey` - you can get this key on the `okmeter` installation page for the appropriate project  (`OKMETER_API_TOKEN`).

An example:

```yaml
okmeterEnabled: "true"
okmeter: |
  apiKey: 5ff9z2a3-9127-1sh4-2192-06a3fc6e13e3
```

