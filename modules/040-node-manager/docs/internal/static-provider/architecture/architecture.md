# Concepts

In terms of [Cluster API concepts](https://cluster-api.sigs.k8s.io/user/concepts), the management cluster and the workload cluster are the same Deckhouse managed Kubernetes cluster.

The Deckhouse Kubernetes platform components (hooks, bashible, node manager) implement the behavior of the Cluster API bootstrap provider.

## Controllers

CAPS has a number of controllers:

- [StaticCluster](./controllers/static-cluster.md)
- [StaticControlPlane](./controllers/static-control-plane.md)
- [StaticMachine](./controllers/static-machine.md)
