# Specifics

## How the modes work

1. Direct, Unmanaged — can only run in clusters managed by DKP;
1. Local, Proxy — can only run in static clusters.

## Bootstrap

1. If the cluster has `clusterIsBootstrapped: false`, changing the registry parameters is blocked. Since deckhouse runs in the host network, the registry must also run in the host network (to reach the registry API). Because of this, switching modes may cause a registry port collision. For this reason, the module "freezes" the input parameters and blocks parameter changes. To unblock it, you need to create a worker node.

## Mode switching

1. The following transitions are blocked:
   - Local -> Proxy;
   - Proxy -> Local;
   - Local -> non-configurable Unmanaged.
