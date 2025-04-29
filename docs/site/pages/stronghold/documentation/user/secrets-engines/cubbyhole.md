---
title: "Cubbyhole secrets engine"
permalink: en/stronghold/documentation/user/secrets-engines/cubbyhole.html
description: >-
  The cubbyhole secrets engine can store arbitrary secrets scoped to a single
  token.
---

The `cubbyhole` secrets engine is used to store arbitrary secrets within the
configured physical storage for Stronghold namespaced to a token. In `cubbyhole`,
paths are scoped per token. No token can access another token's cubbyhole. When
the token expires, its cubbyhole is destroyed.

Also unlike the `kv` secrets engine, because the cubbyhole's lifetime is
linked to that of an authentication token, there is no concept of a TTL or
refresh interval for values contained in the token's cubbyhole.

Writing to a key in the `cubbyhole` secrets engine will completely replace the
old value.

## Setup

Most secrets engines must be configured in advance before they can perform their
functions. These steps are usually completed by an operator or configuration
management tool.

The `cubbyhole` secrets engine is enabled by default. It cannot be disabled,
moved, or enabled multiple times.

## Usage

After the secrets engine is configured and a user/machine has an Stronghold token with
the proper permission, it can generate credentials. The `cubbyhole` secrets
engine allows for writing keys with arbitrary values.

1. Write arbitrary data:

   ```text
   $ d8 stronghold write cubbyhole/my-secret my-value=s3cr3t
   Success! Data written to: cubbyhole/my-secret
   ```

2. Read arbitrary data:

   ```text
   $ d8 stronghold read cubbyhole/my-secret
   Key         Value
   ---         -----
   my-value    s3cr3t
   ```
