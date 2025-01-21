---
title: "Stronghold module"
---

The Stronghold module enables secure storage and lifecycle management of secrets. The secrets storage is implemented as a key-value and is compatible with the Hashicorp Vault API.

Stronghold provides access to data and can be managed through:
* the web interface available at https://stronghold.your-cluster-domain.tld/ui
* the API accessible at https://stronghold.your-cluster-domain.tld/v1

Authentication and authorization in the Stronghold can be performed through:
* Service accounts of Kubernetes cluster applications
* Tokens
* Users can authenticate through cluster Dex/OIDC
* Username/password pair

Access control to secrets within and outside Stronghold is configured using a flexible set of policies.
