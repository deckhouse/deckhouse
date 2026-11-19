# ProjectQuota migration

The leader removes automatic ProjectQuota objects left by earlier versions of
multitenancy-manager after the switch to ProjectQuotaStatus. Their limits are already
stored in `Project.spec.quota`. Removing the old objects releases their
`projects.deckhouse.io/project-quota` finalizer and the dependency that can hold a
foreground deletion of Project.

## Running the transition

The migration follows the leader runnable pattern of
`user-authn-controller/internal/migration`. It starts alongside the normal reconcilers
after the previous-pod wait. It reads directly from the API server and retries incomplete
passes every 30 seconds. Each leader start checks the old API again.

The pass selects old objects with the label `projects.deckhouse.io/managed-by: controller`.
For each selected automatic object, it proceeds in order.

1. Read its Project. While the Project is alive and has `spec.quota`, wait for the normal
   project reconciler to create a ProjectQuotaStatus controlled by that Project, with the
   same limits and without a deletion timestamp. A deleted or deleting Project, or one
   whose quota was cleared, can proceed directly to cleanup.
1. Remove owner references to the old object's exact API version, kind and UID from
   ResourceQuota objects across all namespaces. Preserve their limits, UID and other owners.
   The new quota reconciler restores the namespace quotas from ProjectQuotaStatus.
1. Request background deletion of the old ProjectQuota with a UID precondition. Read it
   again and remove only `projects.deckhouse.io/project-quota`, using an optimistic
   resource-version check. Other finalizers keep the pass pending until their owners finish.

The pass succeeds once the selected automatic objects have disappeared. Errors on one object
allow the other objects in the pass to progress. A restart resumes from the API objects after any
completed step. An object replaced under the same name is checked again on the next pass.

The normal quota reconciler may temporarily release namespace quotas before the replacement
status is created. The migration waits for the replacement object before retiring the old
object. It also runs when an old object is already terminating, so its finalizer can finish
after the previous controller has stopped.

## Keeping the old CRD

The old `projectquotas.deckhouse.io` CustomResourceDefinition (CRD) remains installed
after its automatic objects are removed. Module CRDs are installed outside the module's Helm
release, and removing their file from `crds/` leaves the installed schema in place. Explicit CRD deletion is a separate
operation in the CRD installer. This migration cleans its automatic objects and leaves schema
cleanup to that lifecycle.

When the old API is absent, a missing mapping or an API `NotFound` completes the migration.
Permission errors and API failures are retried. The old kind uses uncached requests, so an
absent CRD does not keep an informer waiting for synchronization.

## Removing this package

Remove the package and its registration together after the following conditions have been
verified for every supported upgrade source, including development clusters running main.

- The source version cannot create automatic ProjectQuota objects.
- Existing installations have completed cleanup, with no automatic ProjectQuota objects or
  ResourceQuota owner references to those objects remaining. For installations where the old
  CRD was removed separately, verify the ResourceQuota owner references as well.
- Every live Project with `spec.quota` has its replacement ProjectQuotaStatus and namespace
  quotas. Deleting a Project with foreground propagation completes.
- The supported upgrade paths and restore procedures cannot reintroduce objects from a
  version that created automatic ProjectQuota.

The migration tests cover replacement checks, terminating projects and old objects,
owner-reference cleanup, partial failures, repeated runs, selection of automatic objects
and an absent old API.
Cluster upgrade and foreground-deletion checks complete the verification.
