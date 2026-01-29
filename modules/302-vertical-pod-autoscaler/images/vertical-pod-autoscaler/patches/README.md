# Patches

## 002-openkruise-daemonset-apiversion.patch

This patch for correction work with VPA in Deckhouse with OpenKruise DaemonSet (apiVersion == apps.kruise.io/v1alpha1)

## 003-recommender.patch

This patch is not working for prometheus storage. Only for VPA checkpoints.
Have no idea, what it is for.
As we use Prometheus storage, will not move this patch.

## 004-updater.patch
1. Module
   vertical-pod-autoscaler — Updater component, package pkg/updater/restriction: restriction factory (pods_restriction_factory.go), eviction restriction (pods_eviction_restriction.go), in-place restriction (pods_inplace_restriction.go), and their tests.
2. What the patch does
   Adds a belowMinReplicas field (bool) to singleGroupStats: when a group has fewer live pods than minReplicas, it is still included in the maps but marked as “below minReplicas”.
   In GetCreatorMaps, when actual < required, the group is no longer skipped (the continue is removed): its stats (configured, running, evictionTolerance, etc.) are still computed and the group is added to the maps with belowMinReplicas = true.
   In PodsEvictionRestrictionImpl.CanEvict, a check is added at the start: if the group has belowMinReplicas == true, the function returns false and logs — eviction (recreate) is not allowed for these groups.
   PodsInPlaceRestriction does not consider belowMinReplicas: in-place decisions still use isPodDisruptable() only, so in-place updates remain allowed even when belowMinReplicas == true.
   Tests are updated/added: in-place for a singleton when below minReplicas, tolerance limit in a single loop, and blocking fallback to eviction when below minReplicas (TestFallbackToRecreateBlockedWhenBelowMinReplicas).
   Net effect: for InPlaceOrRecreate, groups below minReplicas can still receive in-place updates only; eviction/recreate is blocked. If in-place is not possible (e.g. Infeasible), fallback to eviction does not happen — the pod is not evicted, only logged.
3. Goal of the changes
   Allow VPA recommendations to be applied to replica groups below minReplicas (including a single pod) when updateMode: InPlaceOrRecreate, but only via in-place resize, never via eviction/recreate. If in-place cannot be applied, do not evict (do nothing and log), so availability is not reduced.
4. Problem it solves
   Previously, when livePods < minReplicas (e.g. one pod with default --min-replicas=2), the group was omitted from the maps in GetCreatorMaps (due to if actual < required { continue }). As a result:
   In-place was effectively unavailable for those pods (no entry in podToReplicaCreatorMap / no group stats).
   Users with a single replica or few replicas could not get VPA recommendations applied in InPlaceOrRecreate without raising minReplicas or replica count.
   The patch separates two concerns:
   Protection from eviction when replica count is low — still enforced (eviction is blocked when belowMinReplicas).
   Allowing in-place regardless of replica count — now possible: groups below minReplicas are included in the maps with the flag; in-place is allowed for them, eviction is not. If in-place is not possible, fallback to recreate does not result in an eviction. Thus, “VPA cannot apply recommendations to a singleton or small group” is addressed without introducing extra evictions.
