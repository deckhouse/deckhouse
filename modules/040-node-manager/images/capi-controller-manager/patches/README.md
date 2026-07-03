## Patches

### 001-go-mod.patch

Bump libraries versions to resolve CVE

### 002-search-node-by-provider-id-annotation.patch

Add support for searching nodes using the `node.deckhouse.io/provider-id` annotation

### 003-use-bootstrap-data-secret-instead-bootstrap-config-ref.patch

Mark bootstrap initialization complete with BootstrapDataSecretCreated field when DataSecretName is already present.

### 004-disable-clusterresourceset.patch

Disable the ClusterResourceSet and ClusterResourceSetBinding controllers, webhooks and CRD storage-version migration. Since CAPI 1.12 the ClusterResourceSet feature graduated to GA and its feature gate was removed, so these controllers now start unconditionally. Deckhouse does not ship the ClusterResourceSet CRDs, which made the manager crash on cache sync timeout.
