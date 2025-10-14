---
title: "Identity secrets engine"
permalink: en/stronghold/documentation/user/secrets-engines/identity/
---

The Identity secrets engine is the identity management solution for Stronghold. It
internally maintains the clients who are recognized by Stronghold. Each client is
internally termed as an `Entity`. An entity can have multiple `Aliases`. For
example, a single user who has accounts in both GitLab and LDAP, can be mapped
to a single entity in Stronghold that has 2 aliases, one of type GitLab and one of
type LDAP. When a client authenticates via any of the credential backends
(except the Token backend), Stronghold creates a new entity and attaches a new
alias to it, if a corresponding entity doesn't already exist. The entity identifier will
be tied to the authenticated token. When such tokens are put to use, their
entity identifiers are audit logged, marking a trail of actions performed by
specific users.

Identity store allows operators to **manage** the entities in Stronghold. Entities
can be created and aliases can be tied to entities, via the ACL'd API. There
can be policies set on the entities which adds capabilities to the tokens that
are tied to entity identifiers. The capabilities granted to tokens via the
entities are **an addition** to the existing capabilities of the token and
**not** a replacement. The capabilities of the token that get inherited from
entities are computed dynamically at request time. This provides flexibility in
controlling the access of tokens that are already issued.

{% alert level="warning" %}

**NOTE:** This secrets engine will be mounted by default. This secrets engine
cannot be disabled or moved. For more conceptual overview on identity, refer to
the [Identity](../../concepts/identity.html) documentation.

{% endalert %}

The Stronghold Identity secrets engine supports several different features. Each
one is individually documented on its own page.

- [Identity tokens](token.html)
- [OIDC Identity Provider](oidc-provider.html)
