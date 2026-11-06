---
title: Suspended release
description: A Deckhouse Platform release whose rollout on a release channel was paused after it had been published. The release channel image is patched with a suspend flag.
lang: en
---

While a release is suspended, `d8 mirror pull` refuses to copy the [release channel](glossary.html#release-channel) and reports that the source registry contains a suspended release channel. The platform images can still be downloaded by specifying a version explicitly with the `--deckhouse-tag` flag.

For details, see [Updating DP in air-gapped environment](guides/airgapped-update.html).
