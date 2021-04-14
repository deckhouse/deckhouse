---
title: "The deckhouse module"
search: releaseChannel, release channel stabilization, auto-switching the update channel
---

In **Deckhose**, this module sets up:
- The logging level;
- The set of features;
- The desirable update channel.

- Also, this module starts the service for validating CRs that are managed by deckhouse modules.

Features specified in the [configuration](configuration.html) define the set of enabled modules. Usually, the Default set is used (it is suitable for most cases). You can explicitly enable any additional required modules in the configuration.

If you set the releaseChannel parameter in the [configuration](configuration.html), Deckhouse will switch to the selected update channel if the current and target update channels have identical Deckhouse versions.

This switching is not instantaneous and depends on the frequency of version changes on the Deckhouse update channels.
