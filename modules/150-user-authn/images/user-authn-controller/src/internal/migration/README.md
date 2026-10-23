# Migrations of user-authn-controller

A migration converts state that an earlier version of the module left in the cluster into the form
the current version uses. For each one this file says what it converts from and to, why, how the
controller finds out that it applies, what happens when it runs again, and which merge request
introduced it.

A migration that runs a pass and is done lives in this package and is registered by `Register`. A
migration that has to follow changes for as long as an older writer keeps producing the old form is
a controller of its own under `internal/controller` and is listed here with a pointer.

Objects the module chart used to render and the controller now owns are not migrated but adopted:
see [`internal/adoption`](../adoption/README.md).

## Second factors and locks into UserIdentity

- **Code:** the controller `internal/controller/identitymigration`.
- **From:** what Dex of the previous release keeps in OfflineSessions (TOTP secret, lock by an
  external provider) and in Password (lock of a local user).
- **To:** the UserIdentity of the same (connector, user) pair, where Dex v2.46 keeps that data,
  annotated `user-authn.deckhouse.io/migrated` with a record of what was copied.
- **Why:** Dex v2.46 reads the second factor and the lock from UserIdentity only. Until a
  UserIdentity carries the annotation, the transitional Dex patch reads the old places on every read,
  so a user keeps the second factor and the lock while the copy is not done.
- **How it applies:** on the leader, a first pass pages through every OfflineSessions, every locked
  Password and every UserIdentity without the annotation, and enqueues their pairs. After that the
  controller copies a pair again when its OfflineSessions or Password changes, and marks a UserIdentity
  Dex creates for a pair the old places know nothing about. UserOperation copies a pair the same way
  before it writes the UserIdentity.
- **Running again:** the first pass runs at every leader start. A pair is copied only where its
  UserIdentity is behind the old places; nothing removed from a UserIdentity is copied back from the
  same source object, and an identity with nothing to copy is only marked. A local pair with neither a
  Password nor a User, left by a user deleted on the previous release, is not copied, and its
  OfflineSessions is deleted, so a user created again with the same name does not get its TOTP secret.
- **Why a controller:** during the rollout and until every pod runs Dex v2.46, the previous Dex keeps
  writing the old places, so the copy follows them through watches, a workqueue and metrics rather
  than one pass.
- **Introduced in:** !736.

## Refresh tokens of the legacy DexAuthenticator client ids

- **Code:** `DexAuthenticatorTokens` in this package.
- **From:** what Dex keeps for the dex client of a DexAuthenticator under its legacy id
  `<name>-<namespace>-dex-authenticator`: RefreshToken objects, the references to them in
  OfflineSessions (the entry of the client in `refresh` and its entries in `grants`), and the client
  state in browser sessions (`clientStates` of AuthSession).
- **To:** nothing. The client of the DexAuthenticator is `dex-authenticator-<name>@<namespace>` now,
  and its users sign in to it again once.
- **Why:** the chart replaced the client, so Dex refuses these tokens, but its GC deletes a refresh
  token only after it has gone unused for `refreshTokenValidIfNotUsedFor`, and UserAccount lists the
  tokens as grants of the old client until then.
- **How it applies:** on the leader at start, the migration computes the legacy id of every
  DexAuthenticator and takes an id only when no OAuth2Client has it any more: until Helm prunes the old
  client the id still belongs to it, and an id another client happens to have is never touched (the
  DexAuthenticator `kubeconfig` in `foo` and the kubeconfig cluster `foo-dex-authenticator` share
  `kubeconfig-foo-dex-authenticator`). While an id still has an OAuth2Client, it checks again every 30
  seconds, reading the cache only (the DexAuthenticators by metadata, the OAuth2Clients by the index of
  their id), until the id is free or the leader stops; it lists the Dex objects only when an id
  becomes free that it has not cleaned yet.
- **Running again:** every step deletes or removes only what is still there, and a reference is
  removed only if it still names the same refresh token, so a restart halfway does the rest, and a
  later start finds nothing to do and returns.
- **Introduced in:** !910.
