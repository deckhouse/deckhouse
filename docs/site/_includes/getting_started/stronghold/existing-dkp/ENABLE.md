Installation Requirements:
- A deployed Deckhouse Kubernetes Platform cluster of any edition except Community Edition and version no lower than 1.68.

To install Deckhouse Stronghold, enable the stronghold module. The module can be enabled via CLI.

## Enabling the Module via CLI

On a host with access to the DKP cluster, execute the following command using the [Deckhouse CLI](/products/stronghold/reference/cli/d8/):

```bash
d8 platform module enable stronghold
```
