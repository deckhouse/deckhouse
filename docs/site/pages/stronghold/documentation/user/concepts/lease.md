---
title: "Lease"
permalink: en/stronghold/documentation/user/concepts/lease.html
lang: en
description: >-
  Stronghold provides a lease with every secret. When this lease is expired, Stronghold
  will revoke that secret.
---

## Lease, renewal, and revoke

For every dynamic secret and authentication service, Stronghold creates a _lease_,
which includes metadata containing information such as duration, renewability, and more.
Stronghold guarantees that the data will be valid for the given duration (Time to Live, TTL).
Once the lease is expired, Stronghold can automatically revoke the data,
and the consumer of the secret can no longer be certain that it is valid.

Consumers of secrets have to check in with Stronghold routinely to either renew the lease (if allowed)
or request a replacement secret.
This improves the value of Stronghold audit logs and significantly simplifies the key replacement process.

All dynamic secrets in Stronghold are required to have a lease.
Even if the data is meant to be valid "forever", a lease is required to force the consumer to check in routinely.

In addition to renewals, a lease can be _revoked_.
When a lease is revoked, it invalidates that secret immediately and prevents any further renewals.

The revocation can be done manually via the API, via the `d8 stronghold lease revoke` CLI command,
via the user interface under the "Access" tab, or automatically by Stronghold.
When a lease is expired, Stronghold automatically revokes it.
When a token is revoked, Stronghold revokes all leases that were created using it.

{% alert level="info" %}
The Key/Value backend, which stores arbitrary secrets, doesn't issue leases but sometimes returns a lease duration.
For details, refer to the [`kv` secrets engine documentation](../secrets-engines/kv/overview.html).
{% endalert %}

## Lease IDs

When reading a dynamic secret (for example, using the `d8 stronghold read`command), Stronghold always returns a `lease_id`.
This ID can be used in commands such as `d8 stronghold lease renew` and `d8 stronghold lease revoke` to manage the lease of a secret.

## Lease duration and renewal

_Lease duration_ is returned along with the lease ID as a Time To Live (TTL) value, time in seconds for which the lease is valid.
A consumer of this secret must renew the lease within that timeframe.

When renewing the lease, the user can request a specific amount of time they want remaining on the lease,
which is called the `increment`.
This increment to the lease duration won't be added at the end of the current TTL but rather at the request time.
For example, the command `d8 stronghold lease renew -increment=3600 my-lease-id` would request
that the TTL of the lease be adjusted to 1 hour (3600 seconds).
Having the increment be rooted at the current time instead of the end of the lease lets users
increase or reduce the length of leases if they don't need a secret for the full possible lease period.

The requested `increment` is completely advisory.
The backend in charge of the secret can choose to completely ignore it.
For most secrets, the backend does its best to respect the `increment`, but often limits it to ensure renewals every so often.

The return value of renewals should be carefully inspected to determine what the new lease TTL is.

## Prefix-based revocation

In addition to revoking a single secret, users with proper access control can revoke multiple secrets based on their lease ID prefix.

The lease ID prefixes always contain the path where the secret was requested from.
This lets you revoke groups of secrets.
For example, to revoke all Userpass logins, it would be enough to run `d8 stronghold lease revoke -prefix auth/userpass/`.

This can be useful if there is an intrusion within a system.
All secrets of a specific backend or a certain configuration can be revoked quickly.
