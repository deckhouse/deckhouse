## Patches

### Disable controllers
By default kruise controller enables all embeded controllers and watching for all CRDs
We don't have any CRDs except `AdvancedDaemonSet`
Every CRD watch has 15 seconds timeout, so kruise-controller takes a lot of time to start and become ready.
We can check the number of workers (concurrent reconciles) and if we have 0 workers defined - disable the controller


### Disable jobs
Remove CRD check of `BroadcastJob` and `ImagePullJob`. We don't need them for DaemonSet workflow. We don't install that CRDs.

### Stick to MaxUnavailable
In case DaemonSet has surge == 0, Kruise Controller still able to designate more pods as allowed for replacement than
maxUnavailable settings allows, resulting in parallel update instead of gradual.
We impose additional condition to check if the list of the pods, marked by the controller as Unavailable, is bigger than maxUnavailable
and drop excessive pods from the list so as to obbey maxUnavailable setting.
The thing is that having a pod marked as Unavailable from the Controller point of view doesn't mean that the pod doesn't work and
should be updated ASAP.

### Set .spec.replicas
Sets an extra spec field `replicas` which is synced with .status.DesiredNumberScheduled field and is used for providing compatibility with `scale` subresource API.
In its turn, having `scale` subresource allows applying PodDistruptionBudget constraints to advanved daemon sets.
Also, extensive NoSchedule/NoExecute tolerations for daemonsets' pods are removed.
