# Patches for capo-controller-manager

This directory contains patches applied to the upstream `kubernetes-sigs/cluster-api-provider-openstack`
source during the `capo-controller-manager` image build.

## 001-disable-floating-ip-pool-controller.patch

Disables the floating IP pool controller by removing the `IPAddressClaim` watch
from the `OpenStackMachineReconciler`. Deckhouse manages floating IPs through its
own resource controller, so the upstream IPAM integration is not needed and is
removed to avoid conflicts.
