## Patches

### 002-search-node-by-provider-id-annotation.patch

Add support for searching nodes using the `node.deckhouse.io/provider-id` annotation

### 003-go-mod.patch

Bump libraries versions to resolve CVE

### 004-max-surge.patch

Now maxSurge affects the scaleUp of MachineDeployment. maxSurge limits the number of machines ordered simultaneously. New machines will not be ordered until the previous batch of machines transitions to ready