# Adoption by user-authn-controller

Adoption hands an object the module chart used to render over to a reconciler of the controller.
The object keeps its name, labels and content, so applications and users notice nothing; the
reconciler removes the ownership metadata of the release engine and from then on writes the object
like any object it created. For each adoption this file says what is taken over and from whom, why,
how the controller recognises the objects, what happens when it runs again, and which merge request
introduced it.

The package holds the common part: `RenderedByRelease` and `HasManagedMetadata` tell an object of
the release or of the controller from a foreign one, `Metadata` merges the fixed labels of the
controller with the metadata the user asked for and records the keys it set, and `Writer` creates,
adopts or updates an object and refuses a foreign one (`ErrForeign`, checked again every 5 minutes).

The other half is in the module hooks, `hooks/internal/handover`: right before every module run they
stamp `helm.sh/resource-policy: keep` on every object of an adopted Kind that the release still
owns, so the release that stops rendering the objects does not prune them before the controller of
that release is running. The controller removes the annotation together with the ownership metadata.

State that earlier versions left in a form the module no longer uses is migrated, not adopted: see
[`internal/migration`](../migration/README.md).

## DexClient credentials

- **Code:** the reconciler `internal/controller/dexclient`, which runs alone in the
  `dexclient-controller` Deployment (the `dexclient` command of the image), the only ServiceAccount of
  the module allowed to write Secrets.
- **Taken over:** the Secret `dex-client-<name>` in the namespace of every DexClient, and its two
  OAuth2Clients in `d8-user-authn` (the current client id and the legacy one with a colon), which the
  chart rendered from the values of the `get_dex_client_crds` hook.
- **Why:** the chart rendered a new DexClient only on some later module run; the reconciler reacts to
  the DexClient itself. Adopting rather than recreating keeps every client secret applications use.
- **How it recognises the objects:** the Secret by its name and the release annotations, the
  OAuth2Clients by the client ids of the DexClient and the label `app=dex-client`. The client secret
  comes from the Secret, else from either OAuth2Client, and only when neither exists a new one is
  generated. On start the cache of OAuth2Clients maps every client of the chart to its DexClient, so
  the objects of a DexClient deleted before the controller started are deleted too.
- **Running again:** an adopted object carries the controller's metadata and is updated in place; an
  object that is neither the release's nor the controller's is left alone and reported in an event.
- **Introduced in:** !903.

## The kubernetes OAuth2Client

- **Code:** the reconciler `internal/controller/kubernetesclient`.
- **Taken over:** the OAuth2Client `kubernetes` in `d8-user-authn` with its `trustedPeers`, which the
  chart rendered from hook values.
- **Why:** a DexAuthenticator or DexClient granted access to Kubernetes reached `trustedPeers` only on
  some later module run, and one list in one object cannot have two writers. The chart now passes the
  static peers and a checksum of the client secret in the ConfigMap `kubernetes-oauth2client`.
- **How it recognises the object:** by its name and the release annotations.
- **Running again:** the reconciler writes the whole client from its sources every time, so an adopted
  client is simply kept up to date; the client exists while it has a peer.
- **Introduced in:** !903.
