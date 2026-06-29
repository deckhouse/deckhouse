# Patches for capo-controller-manager

This directory contains patches applied to the upstream `kubernetes-sigs/cluster-api-provider-openstack`
source during the `capo-controller-manager` image build.

## 001-disable-floating-ip-pool-controller.patch

Disables the floating IP pool controller by removing the `IPAddressClaim` watch
from the `OpenStackMachineReconciler` and by not starting the
`OpenStackFloatingIPPoolReconciler`. Deckhouse does not use the upstream CAPO
IPAM flow for floating IP management, so this integration is removed to avoid
conflicts.
