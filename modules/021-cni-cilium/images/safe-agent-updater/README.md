== Motivation and problem

When updating the cilium-agent version or changing its image, you may encounter the following situation:
- The old Pod has been deleted from the node, and the new pod takes a long time to start due to issues with loading the new image.
  Depending on the functionality used, this may cause network problems.

== Solution and implementation

To improve the upgrade process for cilium-agent Pods, we take the following steps:

In the agent's DaemonSet manifest:
- Set the `.spec.updateStrategy.type: OnDelete`.
  This will prevent Pods from automatically restarting when the DaemonSet manifest is changed.
- Add an annotation to the Pod template section (`.spec.template.metadata.annotations`) that contain the manifest hash.
  The purpose of this is to determine if there are any changes to the manifest and if the pods need to be reloaded.


Create a service DaemonSet `safe-agent-updater`:
- Specify the same tolerations as the main DaemonSet so that its Pods will be deployed to the same nodes.
- Set the `.spec.updateStrategy.type: RollingUpdate`.
- In the first init container, preload the image that is used in the main DaemonSet onto the node.
- In the second init container, run a small application (`safe-agent-updater`) to check for a match between the hash annotation in the DaemonSet and the agent Pod running on the same node as the application itself. If there is a mismatch, reload the agent Pod.
- In the main container we start `pause`.


The logic of `safe-agent-updater` application is as follows:
- We pass the name of the node on which the application is running via ENV
- Then we connect to k8s api-server
- Get the current value of the manifest hash from the agent DaemonSet
- Get the current value of the manifest hash from the agent Pod that is running on the designated node.
- If the hashes in the Pod and DaemonSet do not match, delete the Pod.
- Wait until the new agent Pod starts correctly and enters Ready status.
- Exit with exit 0

== Summary

We get the following behavior:
- When an agent image is updated, its pods are not restarted until the new image is loaded on the node.
- The command `kubectl -n d8-cni-cilium rollout restart ds agent` will actually do nothing, and the Pods themselves need to be removed.
