---
title: "Tokens"
permalink: en/stronghold/documentation/user/concepts/tokens.html
lang: en
description: Tokens are a core auth method in Stronghold. Concepts and important features.
---

{% alert level="warning" %}
Tokens are opaque values so their structure is undocumented and subject to change.
Scripts and automations that rely on the internal structure of a token in scripts will break.
{% endalert %}

Tokens can be used directly or [auth methods](auth.html) can be used to dynamically generate tokens based on external identities.

All external authentication mechanisms map down to dynamically created tokens.
These tokens have all the same properties as a normal manually created token.

Within Stronghold, tokens map to information.
The most important information mapped to a token is a set of one or more attached [policies](policy.html).
These policies control what the token holder is allowed to do within Stronghold.
Other mapped information includes metadata that can be viewed and is added to the audit log,
such as creation time, last renewal time, and more.

## Token formats

Tokens are composed of a _prefix_ and a _body_.

- The prefix indicates the token's type:

  Token Type | Prefix
  -- | --
  Service tokens | `s.`
  Batch tokens | `b.`
  Recovery tokens | `r.`

- The body is a randomly-generated string of 24 or more character ([Base62 string](https://en.wikipedia.org/wiki/Base62)).

Token are expected to match the following regexp: `[sbr]\.[a-zA-Z0-9]{24,}`

Examples:

```shell
b.n6keuKu5Q6pXhaIcfnC9cFNd
r.JaKnR2AIHNk3fC4SGyyyDVoQ9O
s.raPGTZdARXdY0KvHcWSpp5wWZIHNT
```

## Token types

There are two types of tokens: `service` tokens and `batch` tokens.
A section near the bottom of this page contains detailed information about their differences,
but it is useful to understand other token concepts first.
The features in the following sections all apply to service tokens, and their applicability to batch tokens is discussed later.

## Token store

Often in documentation or in help channels, the "token store" is referenced.
This is the same as the [Token authentication backend](../auth/token.html).
This is a special backend in that it is responsible for creating and storing tokens, and cannot be disabled.
It is also the only auth method that has no login capability -- all actions require existing authenticated tokens.

## Root tokens

Root tokens are tokens that have the root policy attached to them. Root tokens can do anything in Stronghold.
In addition, they are the only type of token within Stronghold that can be set to never expire without any renewal needed.
As a result, it is purposefully hard to create root tokens; in fact there are only three ways to create root tokens:

- The initial root token generated at `d8 stronghold operator init` time. This token has no expiration date.
- By using another root token. A root token with an expiration date cannot create a root token that never expires.
- By using `d8 stronghold operator generate-root` with the permission of a quorum of unseal key holders.

## Token hierarchies and orphan tokens

Normally, when a token holder creates new tokens, these tokens will be created as children of the original token;
tokens they create will be children of them; and so on.
When a parent token is revoked, all of its child tokens -- and all of their leases -- are revoked as well.
This ensures that a user cannot escape revocation by simply generating a never-ending tree of child tokens.

Often this behavior is not desired, so users with appropriate access can create orphan tokens.
These tokens have no parent -- they are the root of their own token tree. These orphan tokens can be created:

- Via `write` access to the `auth/token/create-orphan` endpoint.
- By having `sudo` or `root` access to the `auth/token/create` and setting the `no_parent` parameter to `true`.
- Via token store roles.
- By logging in with any other auth non-Token method.

Users with appropriate permissions can also use the `auth/token/revoke-orphan` endpoint,
which revokes the given token but rather than revoke the rest of the tree,
it instead sets the tokens' immediate children to be orphans.
Use with caution!

## Token accessors

When tokens are created, a token accessor is also created and returned.
This accessor is a value that acts as a reference to a token and can only be used to perform limited actions:

- Look up a token's properties (not including the actual token ID)
- Look up a token's capabilities on a path
- Renew the token
- Revoke the token

Audit devices can optionally be set to not obfuscate token accessors in audit logs.
This provides a way to quickly revoke tokens in case of an emergency.
However, it also means that the audit logs can be used to perform a larger-scale DDoS attack.

Finally, the only way to list tokens is via the `auth/token/accessors` command, which actually gives a list of token accessors.
While this is still a dangerous endpoint (since listing all of the accessors means that they can then be used to revoke all tokens),
it also provides a way to audit and revoke the currently-active set of tokens.

## Token Time-To-Live, periodic tokens, and explicit max TTLs

Every non-root token has a time-to-live (TTL) associated with it,
which is a current period of validity since either the token's creation time or last renewal time, whichever is more recent.
Root tokens may have a TTL associated, but the TTL may also be 0, indicating a token that never expires.
After the current TTL is up, the token will no longer function -- it, and its associated leases, are revoked.

If the token is renewable, Stronghold can be asked to extend the token validity period using `d8 stronghold token renew`
or the appropriate renewal endpoint.
At this time, various factors come into play.
What happens depends upon whether the token is a periodic token
(available for creation by `root`/`sudo` users, token store roles, or some auth methods),
has an explicit maximum TTL attached, or neither.

### General case

In the general case, where there is neither a period nor explicit maximum TTL value set on the token,
the token's lifetime since it was created will be compared to the maximum TTL.
This maximum TTL value is dynamically generated and can change from renewal to renewal,
so the value cannot be displayed when a token's information is looked up. It is based on a combination of factors:

1. The system max TTL, which is 32 days but can be changed.
1. The max TTL set on a mount using `mount tuning`.
   This value is allowed to override the system max TTL -- it can be longer or shorter, and if set this value will be respected.
1. A value suggested by the auth method that issued the token.
   This might be configured on a per-role, per-group, or per-user basis.
   This value is allowed to be less than the mount max TTL (or, if not set, the system max TTL), but it is not allowed to be longer.

Note that the values in (2) and (3) may change at any given time,
which is why a final determination about the current allowed max TTL is made at renewal time using the current values.
It is also why it is important to always ensure that the TTL returned from a renewal operation is within an allowed range;
if this value is not extending, likely the TTL of the token cannot be extended past its current value
and the client may want to reauthenticate and acquire a new token.
However, outside of direct operator interaction, Stronghold will never revoke a token before the returned TTL has expired.

### Explicit max TTLs

Tokens can have an explicit max TTL set on them.
This value becomes a hard limit on the token's lifetime — no matter what the values in (1), (2), and (3)
from the general case are, the token cannot live past this explicitly-set value.
This has an effect even when using periodic tokens to escape the normal TTL mechanism.

### Periodic tokens

In some cases, having a token be revoked would be problematic — for instance, if a long-running service
needs to maintain its SQL connection pool over a long period of time.
In this scenario, a periodic token can be used. Periodic tokens can be created in a few ways:

- By having `sudo` capability or a `root` token with the `auth/token/create` endpoint.
- By using token store roles.
- By using an auth method that supports issuing these, such as AppRole.

At issue time, the TTL of a periodic token will be equal to the configured period.
At every renewal time, the TTL will be reset back to this configured period,
and as long as the token is successfully renewed within each of these periods of time, it will never expire.
Outside of `root` tokens, it is currently the only way for a token in Stronghold to have an unlimited lifetime.

The idea behind periodic tokens is that it is easy for systems and services to perform an action relatively frequently —
for instance, every two hours, or even every five minutes.
Therefore, as long as a system is actively renewing this token — in other words, as long as the system is alive —
the system is allowed to keep using the token and any associated leases.
However, if the system stops renewing within this period (for instance, if it was shut down),
the token will expire relatively quickly.
It is good practice to keep this period as short as possible,
and generally speaking it is not useful for humans to be given periodic tokens.

There are a few important things to know when using periodic tokens:

- When a periodic token is created via a token store role,
  the _current_ value of the role's period setting will be used at renewal time.
- A token with both a period and an explicit max TTL will act like a periodic token
  but will be revoked when the explicit max TTL is reached.

## CIDR-bound tokens

Some tokens are able to be bound to CIDR(s) that restrict the range of client IPs allowed to use them.
These affect all tokens except for non-expiring root tokens (those with a TTL of zero).
If a root token has an expiration, it also is affected by CIDR-binding.

## Token types in detail

There are currently two types of tokens.

### Service tokens

Service tokens are what users will generally think of as "normal" Stronghold tokens.
They support all features, such as renewal, revocation, creating child tokens, and more.
They are correspondingly heavyweight to create and track.

### Batch tokens

Batch tokens are encrypted blobs that carry enough information for them to be used for Stronghold actions,
but they require no storage on disk to track them.
As a result they are extremely lightweight and scalable,
but lack most of the flexibility and features of service tokens.

### Token type comparison

This reference chart describes the difference in behavior between service and batch tokens.

|                                                     |                                          Service Tokens |                                    Batch Tokens |
| --------------------------------------------------- | ------------------------------------------------------: | ----------------------------------------------: |
| Can Be Root Tokens                                  |                                                     Yes |                                              No |
| Can Create Child Tokens                             |                                                     Yes |                                              No |
| Can be Renewable                                    |                                                     Yes |                                              No |
| Manually Revocable                                  |                                                     Yes |                                              No |
| Can be Periodic                                     |                                                     Yes |                                              No |
| Can have Explicit Max TTL                           |                                                     Yes |                    No (always uses a fixed TTL) |
| Has Accessors                                       |                                                     Yes |                                              No |
| Has Cubbyhole                                       |                                                     Yes |                                              No |
| Revoked with Parent (if not orphan)                 |                                                     Yes |                                   Stops Working |
| Dynamic Secrets Lease Assignment                    |                                                    Self |                          Parent (if not orphan) |
| Cost                                                | Heavyweight; multiple storage writes per token creation | Lightweight; no storage cost for token creation |

### Service vs. batch token lease handling

#### Service tokens

Leases created by service tokens (including child tokens' leases) are tracked along with the service token
and revoked when the token expires.

#### Batch tokens

Leases created by batch tokens are constrained to the remaining TTL of the batch tokens and,
if the batch token is not an orphan, are tracked by the parent.
They are revoked when the batch token's TTL expires,
or when the batch token's parent is revoked (at which point the batch token is also denied access to Stronghold).
