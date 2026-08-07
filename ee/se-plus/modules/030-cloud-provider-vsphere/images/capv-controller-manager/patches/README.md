# Patches for capv-controller-manager

This directory contains patches applied to the upstream `kubernetes-sigs/cluster-api-provider-vsphere`
source during the `capv-controller-manager` image build.

## 001-disable-ipam-watch.patch

Removes the `IPAddressClaim` (`ipam.cluster.x-k8s.io/v1beta1`) watch from
`VSphereVMReconciler`. Deckhouse does not ship the CAPI IPAM CRDs, so the
manager fails to start with `timed out waiting for cache to be synced for Kind
*v1beta1.IPAddressClaim` when the watch is registered.

The `reconcileIPAddressClaims` / `deleteIPAddressClaims` helpers are left in
place — they iterate over `VSphereVM.Spec.Network.Devices[].AddressesFromPools`
which is always empty in Deckhouse-rendered `VSphereMachineTemplate`s, so the
outer loop is a no-op and no `IPAddressClaim` objects are ever queried at
runtime.

Mirrors the intent of `cluster-api-provider-openstack`'s
`001-disable-floating-ip-pool-controller.patch`.
