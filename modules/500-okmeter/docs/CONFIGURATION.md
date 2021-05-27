---
title: "The okmeter module: configuration"
---

This module is **disabled** by default. To enable it, add the following lines to the `deckhouse` ConfigMap:

```yaml
data:
  okmeterEnabled: "true"
```

You need to add the `apiKey` for the `okmeter` module to the Deckhouse configuration:

* `apiKey` - you can get this key on the `okmeter` installation page for the appropriate project  (`OKMETER_API_TOKEN`).
