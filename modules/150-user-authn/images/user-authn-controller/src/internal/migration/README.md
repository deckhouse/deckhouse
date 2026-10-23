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
