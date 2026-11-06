---
title: Secrets store
description: An external store compatible with the HashiCorp Vault API, or the built-in Deckhouse Stronghold, from which the [`secrets-store-integration`](/modules/secrets-store-integration/) module delivers secrets to applications without storing them in etcd.
lang: en
---

The [`secrets-store-integration`](/modules/secrets-store-integration/) module delivers secrets to pods as mounted files or environment variables, refreshes mounted secrets every two minutes when the value in the store changes and injects an entrypoint for applications that cannot read secrets from the store themselves. It connects to an internal Deckhouse Stronghold instance automatically in the `DiscoverLocalStronghold` mode and works with any Vault-compatible store in the `Manual` mode.

For details, see [Secrets-store-integration module](architecture/security/secrets-store-integration.html).
