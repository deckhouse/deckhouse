---
title: "Configuring access to project"
permalink: en/stronghold/documentation/user/access.html
searchable: false
sitemap_include: false
---

To configure access to your project using CLI in Deckhouse Stronghold, follow these steps:

1. Install the [`d8`](/products/kubernetes-platform/documentation/v1/cli/d8/) utility.
1. Set your Stronghold server address:

   ```shell
   export STRONGHOLD_ADDR=https://stronghold.domain.my
   ```

1. Log in to Stronghold using the following command:

   ```shell
   d8 stronghold login -path=oidc_deckhouse -method=oidc -no-print
   ```

1. After that, use the following command format for managing objects in your project:

   ```shell
   d8 stronghold <command>
   ```
