# Patches

## Disable controllers
By default kruise controller enables all embeded controllers and watching for all CRDs
We don't have any CRDs except `AdvancedDaemonSet`
Every CRD watch has 15 seconds timeout, so kruise-controller takes a lot of time to start and become ready.
We can check the number of workers (concurrent reconciles) and if we have 0 workers defined - disable the controller


## Disable jobs
Remove CRD check of `BroadcastJob` and `ImagePullJob`. We don't need them for DaemonSet workflow. We don't install that CRDs.
