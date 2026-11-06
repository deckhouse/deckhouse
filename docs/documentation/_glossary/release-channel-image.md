---
title: Release channel image
description: The release-channel image in the Deckhouse Platform registry whose tag is the name of a release channel. The tag points to the image of a specific Deckhouse Platform version and is moved to a new image when a new version is released.
lang: en
---

The image contains `version.json` with the current version of the [release channel](glossary.html#release-channel) and `changelog.yaml`. When the version in the image changes, Deckhouse Platform creates a [DeckhouseRelease](reference/api/cr.html#deckhouserelease) object and starts the update. A rollout can be paused by patching the image with a `suspend` flag, which makes it a [suspended release](glossary.html#suspended-release). In a [private environment](glossary.html#private-environment) the image is copied to the private registry together with the [image bundle](glossary.html#mirror-image-bundle).

For details, see [Updating DP in air-gapped environment](guides/airgapped-update.html).
