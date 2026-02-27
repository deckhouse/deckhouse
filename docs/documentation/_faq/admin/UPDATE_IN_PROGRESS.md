---
title: How can I tell that an update is in progress?
subsystems:
  - deckhouse
lang: en
---

During an update:

- The [`DeckhouseUpdating`](../reference/alerts.html#monitoring-deckhouse-deckhouseupdating) alert is active.
- The `deckhouse` Pod is not in the `Ready` state.
  If the Pod stays in a non-`Ready` state for a long time, it may indicate an issue with DKP that requires investigation.
