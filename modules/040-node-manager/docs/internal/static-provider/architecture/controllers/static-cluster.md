# StaticCluster Controller

The StaticCluster controller implements only a small subset of the CAPI Infrastructure Provider functionality required by CAPI Cluster Controller.

The StaticCluster controller main responsibilities are:

* Manage the lifecycle of the StaticCluster object referenced in `Cluster.spec.infrastructureRef`.
* Setting `spec.controlPlaneEndpoint` and `status.ready` fields.

By convention, the `StaticCluster` object **must** have `spec` and `status` objects.

The `spec` object have the following fields defined:

- `controlPlaneEndpoint` - identifies the endpoint used to connect to the target cluster API server.

The `status` object have the following fields defined:

- `ready` - a boolean field that is true when the infrastructure is ready to be used. Always set to true.
