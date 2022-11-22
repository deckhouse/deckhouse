---
title: "The deckhouse module: configuration"
---

{% include module-bundle.liquid %}

## Parameters

<!-- SCHEMA -->

**Note (!)** that Deckhouse will stop working if there is a nonexistent label in `nodeSelector` or `tolerations` specified are incorrect. You need to change the values to the correct ones in `configmap/deckhouse` and `deployment/deckhouse` to get Deckhouse back on track.
