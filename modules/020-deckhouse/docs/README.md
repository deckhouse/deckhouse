---
title: "The deckhouse module"
search: releaseChannel, release channel stabilization, auto-switching the release channel
---

In Deckhouse, this module sets up:
- The logging level;
- The set of features;
- The desirable release channel;
- Update windows;
- Update mode (Manual / Auto); 
- The service for validating custom resources that are managed by Deckhouse modules.

The bundle parameter specified in the [configuration](configuration.html#parameters-bundle) defines the set of enabled modules. Usually, the `Default` set is used (it is suitable for most cases). You can explicitly enable any additional required modules in the configuration.

If you set the [releaseChannel](configuration.html#parameters-releasechannel) parameter in the configuration, Deckhouse will switch to the selected release channel if the current and target release channels have identical Deckhouse versions. This switching is not instantaneous and depends on the frequency of version changes on the Deckhouse release channels.
