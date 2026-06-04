# WaypointInstance Controller

## Purpose

The `waypoint-controller` provides a Deckhouse-managed abstraction for Istio Ambient waypoint proxies. Users create a `WaypointInstance` custom resource in an application namespace, and the controller reconciles the full set of Kubernetes resources required to run a waypoint proxy for that namespace.

## Goals

Istio's built-in waypoint provisioning (via `istioctl waypoint apply` or the Istio Gateway controller) is limited. Replica count requires manual Deployment patching or a separately managed HPA. Node selectors, tolerations, and affinity can only be set globally, not per waypoint. There is no built-in VPA integration. Scaling modes cannot be changed declaratively.

This controller exists to solve these limitations:

- **Rich workload management.** Provide per-instance control over replicas (`Static` / `HPA`), resource requests and limits (`Static` / `VPA`), node placement (selectors, tolerations, anti-affinity), and disruption budgets -- all declaratively through a single CRD.
- **Simple user interface.** Users should not need to know internal details like HBONE port numbers, Istio identity labels, or Gateway API address tricks. Creating a `WaypointInstance` with a replica mode and resource mode should be enough.
- **Consistent Deckhouse operational model.** Waypoints should be managed the same way as other Deckhouse-controlled workloads — via a Deckhouse CRD with status reporting, owner-reference-based cleanup, and module lifecycle integration.
- **Per-instance configuration.** Each waypoint instance in a namespace can have its own replica count, resource profile, and scaling strategy, rather than relying on a single global configuration.

## Non-Goals

- **No Istio control-plane reimplementation.** The controller manages waypoint *infrastructure* (Deployments, Services, Gateways), not Istio control-plane behavior. Certificate issuance, xDS configuration, and traffic policy enforcement remain Istio's responsibility.
- **No workload attachment management.** The controller does not label namespaces, Services, or workloads with `istio.io/use-waypoint`. Attachment is the user's responsibility via standard Istio labels.
- **No traffic routing configuration.** The controller does not manage `HTTPRoute`, `TLSRoute`, `AuthorizationPolicy`, or any other policy that determines what traffic flows through the waypoint. It only ensures the waypoint proxy is running.
- **No per-instance Istio revision selection.** The first implementation uses the global Istio revision from the module. Pinning a waypoint to a specific revision is out of scope.
- **No cluster-scoped waypoint instances.** `WaypointInstance` is namespaced only. Cluster-wide waypoints are out of scope unless explicitly required later.
- **No mutation of user resources.** The controller must not modify user namespaces, workloads, Services, ServiceAccounts, or other application resources.

## User Story

A platform engineer enables Istio Ambient mode in Deckhouse. The application namespace is already enrolled in the Istio mesh (it has the `istio.io/rev` label, the `istio-ca-root-cert` ConfigMap, and the `d8-istio-sidecar-registry` image pull secret).

An application team creates a `WaypointInstance` in their namespace:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: WaypointInstance
metadata:
  name: main
spec:
  waypointFor: All
  nodeSelector:
    node-role/app: ""
  tolerations:
    - key: node-role/app
      operator: Exists
  replicasManagement:
    mode: Static
    static:
      replicas: 1
  resourcesManagement:
    mode: VPA
    vpa:
      mode: Initial
      cpu:
        min: 100m
        max: 1000m
      memory:
        min: 500Mi
        max: 2000Mi
```

The controller creates the waypoint infrastructure: a ServiceAccount, a ClusterIP Service (HBONE port 15008, status port 15021), a Deployment running the Istio `proxyv2` image in waypoint mode, a Gateway API Gateway with `gatewayClassName: istio-waypoint`, and a VPA. When the effective minimum replica count is >= 2, it also creates a PDB.

The application team attaches workloads to the waypoint using standard Istio labels (e.g., `istio.io/use-waypoint: d8-waypoint-main`). The controller does not manage attachment labels.

When the team changes `replicasManagement` to `Static` with 3 replicas, the controller updates the Deployment and creates a PDB. When they switch `resourcesManagement` to `Static`, the controller deletes the VPA and sets explicit resource requests/limits.

When the team deletes the `WaypointInstance`, all managed resources are cleaned up automatically via the controller's finalizer (deleting single-owner children and detaching from multi-owner ones).
