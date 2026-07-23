## Patches

### 001-add-pdb.patch

Adds an extra spec field `.spec.replicas` which is set by the kruise controller every time an advanced daemonset set is reconcilied. The replicas value is calculated based on
the number of nodes eligible for scheduling the advanced daemonset's pods (cordoned nodes are counted as eligible).
Adds /scale subresource to advanced daemonset CRD so that a PDB could enforce its policy.
Adds some extra logic to the kruise controller to deal with relevant PDB's (to make them resync when necessary) and timely update replicas' count in some corner cases.

### 002-stick-to-maxunavailable.patch

In case DaemonSet has surge == 0, Kruise Controller still able to designate more pods as allowed for replacement than
maxUnavailable settings allows, resulting in parallel update instead of gradual.
We impose additional condition to check if the list of the pods, marked by the controller as Unavailable, is bigger than maxUnavailable
and drop excessive pods from the list so as to obbey maxUnavailable setting.
The thing is that having a pod marked as Unavailable from the Controller point of view doesn't mean that the pod doesn't work and should be updated ASAP.

### 003-disable-protection-logger.patch

Disables  audit log for pub and deletion protection

### 004-add-label-selector-to-scale.patch

Adds .status.labelSelector field to the daemonset crd and implements updating this status field with a serialized label selector in string form (required to implement VPA for advanced daemonsets).
(this patch should go along wth Add pdb patch)

### 005-disable-controllers.patch

By default kruise controller enables all embeded controllers and watching for all CRDs
We don't have any CRDs except `AdvancedDaemonSet`
Every CRD watch has 15 seconds timeout, so kruise-controller takes a lot of time to start and become ready.
We can check the number of workers (concurrent reconciles) and if we have 0 workers defined - disable the controller

### 006-disable-jobs.patch

Remove CRD check of `BroadcastJob` and `ImagePullJob`. We don't need them for DaemonSet workflow. We don't install that CRDs.

### 007-fix-informer.patch

Addopts multi-namespace cache instead of using sharedindexinformer for getting necessary listers, as controller-runtime since v0.15.0+ doesn't provide sharedindexinformers for namespaced caches anymore, breaking openkruise logic https://github.com/openkruise/kruise/issues/1764.

### 008-go-mod.patch

Fix vulnerabilities in components.
