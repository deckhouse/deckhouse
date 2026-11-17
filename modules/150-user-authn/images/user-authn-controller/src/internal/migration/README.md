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
  DexAuthenticator and takes an id only when no OAuth2Client has it any more: until the old client is
  deleted the id still belongs to it (the controller deletes it once the Service of the object points
  at an instance, see the dex-authenticator of every DexAuthenticator below), and an id another client happens to have is never touched (the
  DexAuthenticator `kubeconfig` in `foo` and the kubeconfig cluster `foo-dex-authenticator` share
  `kubeconfig-foo-dex-authenticator`). While an id still has an OAuth2Client, it checks again every 30
  seconds, reading the cache only (the DexAuthenticators by metadata, the OAuth2Clients by the index of
  their id), until the id is free or the leader stops; it lists the Dex objects only when an id
  becomes free that it has not cleaned yet.
- **Running again:** every step deletes or removes only what is still there, and a reference is
  removed only if it still names the same refresh token, so a restart halfway does the rest, and a
  later start finds nothing to do and returns.
- **Introduced in:** !910.

## Token exchange of DexProviders

- **Code:** `DexProviderTokenExchange` in this package; the marker is read by the module hook
  `hooks/discover_dexprovider_token_exchange_migration.go`.
- **From:** OIDC and Gitlab DexProviders without `spec.tokenExchange.enabled`. Before the field existed,
  every connector of a DexProvider served the token-exchange grant.
- **To:** `spec.tokenExchange.enabled` set on each of them: `true` while some DexClient carries
  `dexclient.deckhouse.io/allow-access-to-kubernetes: "true"` and is not being deleted, `false` otherwise. Such
  a client is a trusted peer of the `kubernetes` client and could exchange a token of the provider's issuer for
  one the Kubernetes API accepts, so a cluster that may be doing it keeps it; one that cannot loses nothing.
  A DexClient without `allowedProviders` may use every provider, and the migration does not narrow them down
  by `allowedProviders`, so one annotated client keeps every provider on. Other types are not touched: their
  connectors do not exchange tokens. Then the ConfigMap `d8-user-authn/dexprovider-token-exchange-migration`,
  which lists the providers turned on and off and the annotated clients.
- **Not counted:** the other trusted peers of the `kubernetes` client. A DexAuthenticator with
  `dexauthenticator.deckhouse.io/allow-access-to-kubernetes: "true"` is one, and patch 425 does not refuse it,
  but its oauth2-proxy never exchanges tokens: only someone who took its client secret from the Secret in the
  namespace of the application could, and the migration does not keep that path open. The peers of the module
  configuration are kubeconfig clients, to which patch 425 refuses token exchange.
- **Why:** the connector of a DexProvider does not check the audience of the exchanged token, so token
  exchange through it is off by default and can only be turned off (the ValidatingAdmissionPolicy
  `dexprovider-token-exchange.deckhouse.io`, which lets the controller through by its exact name). The
  migration keeps it for the clusters that may use it, until it is removed after the next LTS release.
- **How it applies:** on the leader at start, unless the marker exists. Until the marker exists, the Dex
  config lets a provider without the field exchange tokens, as before; the config is rendered on the
  upgrade before the controller runs, so pipelines get no 400 in between. A provider created during that
  window is migrated with the others. After the marker, a provider without the field does not exchange
  tokens.
- **Running again:** a provider that has the field is skipped, so one an administrator turned off stays
  off; the update carries the resourceVersion read, and a provider changed meanwhile is read again at the
  next pass, 30 seconds later. Once the marker exists, the migration does nothing.
- **Introduced in:** the merge request that added `spec.tokenExchange` to DexProvider.

## The dex-authenticator of every DexAuthenticator

- **Code:** `DexAuthenticatorLegacy` in this package; the reconcilers of
  `internal/controller/dexauthenticator` call it. The objects the controller keeps are taken over by
  `internal/adoption`, as those of DexClients were.
- **From:** what the module chart rendered for every DexAuthenticator before the shared
  dex-authenticator instances. In the namespace of the object: a Deployment with its
  PodDisruptionBudget, VerticalPodAutoscaler and NetworkPolicy, the Secret `dex-authenticator-<name>`
  with the secret of its Dex client and of its cookies, a copy of the registry Secret
  (`registry-dex-authenticator`), the Service `<name>-dex-authenticator` and the Ingress and HTTPRoute
  objects that lead `/dex-authenticator` to it. In `d8-user-authn`: its OAuth2Client.
- **To:** a record in the table of clients of an instance (`dex-authenticator-system` for the objects
  of the `d8-*` namespaces, `dex-authenticator` for the rest). The OAuth2Client, the Service and the
  routes stay, under the same names, as objects of the controller; the Service becomes an
  ExternalName for the Service of the instance. The Deployment with the objects around it, the Secret
  and the registry Secret are deleted.
- **Secrets:** an object of a `d8-*` namespace keeps the two secrets of its Secret, so its users stay
  signed in: the proxy of the instance restores a session from the refresh token in the cookie. An
  object of a user gets new ones, and its users sign in once more: its Secret lay in a namespace whose
  users could read it.
- **Why:** a Deployment, a Redis and two secrets per DexAuthenticator in the namespaces of
  applications, and a release of the whole module on every change of a DexAuthenticator.
- **How it applies:** the module hooks keep the release that stops rendering these objects from
  pruning them and put the label of the controller on the ones it takes over
  (`hooks/handover_dex_authenticator.go` of the module). The table reconciler gives every object a
  record; the reconciler of the objects waits until every pod of the instance serves the record, then
  in one pass writes the secret of the record into the OAuth2Client, points the Service at the
  instance, takes the routes over and deletes what is left. Until that pass the pods of the object
  serve it as before. An object the instance cannot serve (its record does not fit the table, or
  its domain is a name of the Service of an instance) keeps its own dex-authenticator.
- **The Dex client under the former ID:** an object rendered while the ID of its Dex client was
  `<name>-<namespace>-dex-authenticator` has pods that sign users in with that client, while the
  client the controller makes has the ID `dex-authenticator-<name>@<namespace>`. The module hook
  keeps the former client from the release too and hands it to the controller, so users sign in
  through the pods of the object until the switch; the controller deletes the client in the pass
  that deletes the pods, before them. An object that keeps its own dex-authenticator keeps the
  client. The client carries no name of its DexAuthenticator and is found by its name and its ID; it
  is deleted only with the labels the chart gave it (`app: dex`) and the label of the controller,
  since a client of another kind may have the same ID (see the refresh tokens above). Its refresh
  tokens are cleaned by `DexAuthenticatorTokens` once the ID is free. While the client is there, the
  former ID of an object with access to the Kubernetes API is a trusted peer of the `kubernetes`
  client (`internal/controller/kubernetesclient`): the pods of the object ask for a token
  kube-apiserver accepts under that ID.
- **Running again:** the controller lists what is left of these objects once, when it is first asked
  after its start; an object that has nothing left costs no request. Every step looks at what is
  there: a restart between the switch of the Service and the deletion deletes the rest on the next
  pass, and what is left of an object deleted meanwhile is deleted too. Only objects with the
  labels of the chart (`app: dex-authenticator`, `heritage: deckhouse`, `module: user-authn`) are
  read or deleted, and an admission policy holds the controller to that.
- **Introduced in:** the merge request that switched DexAuthenticator objects to the shared
  dex-authenticator instances.
