---
title: Stronghold sealing
description: The state of the Deckhouse Stronghold secrets store in which it can neither read nor write secrets. The store is sealed and unsealed in the Stronghold web interface.
lang: en
---

The "Seal Stronghold" section of the web interface shows the current state of the [secrets store](glossary.html#secrets-store) and a button that seals or unseals it. While the store is sealed, the [`stronghold`](/modules/stronghold/) module does not serve requests to read or write secrets.

For details, see [Secret management web interface](user/web/stronghold.html).
