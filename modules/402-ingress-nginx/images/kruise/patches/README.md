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

### Add pdb
Adds an extra spec field `.spec.replicas` which is set by the kruise controller every time an advanced daemonset set is reconcilied. The replicas value is calculated based on
the number of nodes eligible for scheduling the advanced daemonset's pods (cordoned nodes are counted as eligible).
Adds /scale subresource to advanced daemonset CRD so that a PDB could enforce its policy.
Adds some extra logic to the kruise controller to deal with relevant PDB's (to make them resync when necessary) and timely update replicas' count in some conrner cases.

### Go mod
Updates library versions.
To create this patch run:
```shell
go mod edit -go 1.20
go get golang.org/x/net@v0.17.0
go get github.com/docker/distribution@v2.8.3
go get github.com/docker/docker@v20.10.24
go get github.com/opencontainers/runc@v1.1.5
go get gopkg.in/yaml.v3@v3.0.1
go mod tidy
git diff
```

### Add label selector to scale
Adds .status.labelSelector field to the daemonset crd and implements updating this status field with a serialized label selector in string form (required to implement VPA for advanced daemonsets).
(this patch should go along wth Add pdb patch)
