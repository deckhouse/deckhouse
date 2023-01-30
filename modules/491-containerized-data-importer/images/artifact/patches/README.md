# Patches

#### `000-bundle-images.patch`

Iternal patch which adds images bundle target with all images to build.

#### `001-deckhouse-registry.patch`

Internal patch which adds deckhouse ImagePullSecrets to kubevirt VMs

- https://github.com/kubevirt/containerized-data-importer/issues/2395
- https://kubernetes.slack.com/archives/C0163DT0R8X/p1660319072512309

#### `002-storagecapabilities.patch`

Add default storage capabilities for LISNTOR

- https://github.com/kubevirt/containerized-data-importer/pull/2537

#### `003-apiserver-node-selector-and-tolerations.patch`

Allow to override nodeSelector and tolerations for cdi-apiserver
