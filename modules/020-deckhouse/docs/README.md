---
title: "The deckhouse module"
search: releaseChannel, release channel stabilization, auto-switching the release channel
---

In Deckhouse, this module sets up:
- **[The logging level](configuration.html#parameters-loglevel)**;
- **[The set of modules](configuration.html#parameters-bundle) enabled by default**;

Usually, the `Default` set is used (it is suitable for most cases).

Regardless of the set of modules enabled by default, any module can be explicitly enabled or disabled in the Deckhouse configuration (learn more [about enabling and disabling a module](../../#enabling-and-disabling-a-module)).
- **[The release channel](configuration.html#parameters-releasechannel)**;

Deckhouse has a built-in mechanism for automatic updates. This mechanism uses [5 release channels](../../deckhouse-release-channels.html) with various stability and frequency of releases. Learn more about [how the automatic update mechanism works](.../deckhouse-faq.html#how-does-the-mechanism-of-automatic-stabilization-of-the-release-channel-work) and how you can [set the desired release channel](.../deckhouse-faq.html#how-do-i-set-the-desired-release-channel).
- **[The update mode](configuration.html#parameters-update-mode)** and **[update windows](configuration.html#parameters-update-windows)**;

Deckhouse supports **manual* and *automatic** update modes.

In the manual upgrade mode, only critical fixes (patch releases) are automatically applied, and upgrading to a more current Deckhouse release requires [manual confirmation](cr.html#deckhouserelease-v1alpha1-approved).

In the automatic update mode, Deckhouse switches to a newer release as soon as it is available in the corresponding release channel unless [update windows](configuration.html#parameters-update-windows) are **configured** for the cluster. If update windows are **configured** for the cluster, Deckhouse will upgrade to a newer release during the next available update window.

- **Service for validating Custom Resources**.

The validation service prevents creating Custom Resources with invalid values or adding such values to the existing Custom Resources. Note that it only tracks Custom Resources managed by Deckhouse modules.
