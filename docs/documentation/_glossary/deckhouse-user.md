---
title: The deckhouse system user
description: The local user and group with UID and GID 64535 that Deckhouse Platform components use to run unprivileged processes on nodes. The name and identifiers are reserved and not configurable.
lang: en
---

If the account does not exist, [bashible](glossary.html#bashible) creates it without a password, `sudo` privileges or a login shell. The account must not be provided through AD or LDAP. Processes owned by `deckhouse` in `ps` or `top` output belong to Deckhouse Platform components. The [preflight check](glossary.html#preflight-checks) accepts an absent user and group or an existing pair with the reserved identifiers and no `sudo` privileges. The installation fails if the name is used with other identifiers or the identifiers belong to another account.

For details, see [Installation](installing/).
