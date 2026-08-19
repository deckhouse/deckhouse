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

Drops the CRD-migrator entry, controller and webhook registrations for the two
CRDs Deckhouse does not ship in `crds/external/`:

- `VSphereClusterIdentity` — Deckhouse manages the vCenter credential Secret
  (`capi-user-credentials`) as regular Kubernetes Secret referenced by
  `VSphereCluster.spec.identityRef.kind: Secret`, so the identity CR is
  unused. Without the patch the manager crashes with
  `no matches for kind "VSphereClusterIdentity"` at startup because the
  controller registration is unconditional.
- `VSphereClusterTemplate` — Deckhouse does not use CAPI ClusterClass, so the
  validating webhook + CRD-migrator entry are dead weight.

`VSphereDeploymentZone` / `VSphereFailureDomain` are **kept intact** — their
CRDs, controllers, webhooks and the `VSphereCluster` reconciler's
`Watches(&VSphereDeploymentZone{}, ...)` all stay in place. Deckhouse creates
the two zone CRs from `ensure_failure_domains.go` per NodeGroup zone, and CAPV
uses them at reconcile time to override `VSphereVM.spec.{Server,Datacenter,
Folder,ResourcePool,Datastore,Networks}` from the resolved topology
(`pkg/services/vimmachine.go` `overrideWithFailureDomainFunc`) — this is the
only mechanism CAPV has for real multi-zone placement.

## Why `VSphereVM` is kept even though nothing renders it

CAPV's govmomi flavor spawns a `VSphereVM` per `VSphereMachine` from
`pkg/services/vimmachine.go` (`createOrPatchVSphereVM`, owner ref →
`VSphereMachine`) and reconciles it in `controllers/vspherevm_controller.go`.
This is the layer that tracks long-running vCenter clone tasks
(`VSphereVM.Status.TaskRef`), IP allocation, and MAC address — the equivalent
of CAPO's `OpenStackServer`. Deleting the CRD sends the manager into CrashLoop
with a `CacheSyncTimeout` because the CRD is not optional in this flavor.
