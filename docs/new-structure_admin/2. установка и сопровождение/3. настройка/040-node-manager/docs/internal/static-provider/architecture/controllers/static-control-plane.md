# StaticControlPlane Controller

The StaticControlPlane controller implements only a small subset of the CAPI Control Plane controller functionality required by CAPI Machine Controller.

The StaticControlPlane controller main responsibilities are:

* Manage the lifecycle of the StaticControlPlane object referenced in `Cluster.spec.controlPlaneRef`.
* Setting the `status.initialized` field.
* Setting the `status.ready` field.
* Setting the `status.externalManagedControlPlane` field.

By convention, the `StaticControlPlane` object **must** have a `status` object.

The `status` object have the following fields defined:

* `initialized` - a boolean field that is true when the target cluster has
  completed initialization such that at least once, the
  target's control plane has been contactable. Always set to true.
* `ready` - denotes that the target API Server is ready to receive requests. Always set to true.
* `externalManagedControlPlane` - is a bool that should be set to true if the Node objects do not
  exist in the cluster. It is important to hide Node objects which is not associated with StaticInstance objects.
  Always set to true.
