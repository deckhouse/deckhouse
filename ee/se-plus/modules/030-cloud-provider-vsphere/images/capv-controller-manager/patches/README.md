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

## 002-drop-unused-controllers.patch

Drops the controllers, webhooks, and CRD-migrator entries for CRDs that
Deckhouse does not ship in `crds/external/`:

- `VSphereClusterIdentity` — Deckhouse manages the vCenter credential Secret
  (`capi-user-credentials`) as regular Kubernetes Secret referenced by
  `VSphereCluster.spec.identityRef.kind: Secret`, so the identity CR is
  unused. Without the patch the manager crashes with
  `no matches for kind "VSphereClusterIdentity"` at startup because the
  controller registration is unconditional.
- `VSphereDeploymentZone` / `VSphereFailureDomain` — Deckhouse renders
  the resource-pool / datastore / network directly into
  `VSphereMachineTemplate`, matching the CAPO pattern of a stringly-typed
  `failureDomain`. The CRD-based multi-zone topology is not used.
- `VSphereClusterTemplate` — Deckhouse does not use CAPI ClusterClass, so
  the validating webhook + CRD-migrator entry are dead weight.

Result: CAPV owns only the four CRDs Deckhouse actually renders or CAPV
creates as an intermediate object: `VSphereCluster`, `VSphereMachine`,
`VSphereMachineTemplate`, `VSphereVM` — one-to-one with CAPO's
`OpenStackCluster` / `OpenStackMachine` / `OpenStackMachineTemplate` /
`OpenStackServer`.
