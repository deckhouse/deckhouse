## Patches

Warning! Some required patches that extend kubernetes functionally located in `ee/modules/000-common/images/kubernetes/patches/`
directory.

### local-init-configuration.patch

We want to include in join data the following:

```yaml
apiVersion: kubeadm.k8s.io/v1beta2
kind: InitConfiguration
localAPIEndpoint:
  advertiseAddress: { { .nodeIP | quote } }
  bindPort: 6443
```

> Consider finding a way to do it without patching the `kubeadm` or make a PR to the upstream.

### pdb-daemonset.patch

Supports DaemonSets in disruption controller by adding /scale subresource to daemonsets API. It allows to control the eviction rate of DaemonSet pods.

> Upstream PR https://github.com/kubernetes/kubernetes/pull/98307.

### fix-mount-hostaliases.patch

Fixes a bug where pods with hostNetwork ignored host aliases (k8s < 1.32):

> https://github.com/kubernetes/kubernetes/pull/126460

### resource-quota-ignore-mechanism.patch

Add resource quota ignore mechanism for k8s pvc and pod based on labels

### kubelet-graceful-shutdown-cleanup-memory-manager-state

This patch ensures that the Memory Manager state file is removed during a graceful node shutdown.

The Memory Manager stores the node memory state in a file. After a reboot, the amount of used memory may slightly differ from the previous state, which can make the stored state invalid and prevent the kubelet from starting. Removing the state file before shutdown ensures that the Memory Manager starts with a clean state after the reboot.
See issue: https://github.com/kubernetes/kubernetes/issues/131253

### kubelet-disable-k-panic-check

Kubelet strictly checks that the `kernel.panic` parameter equals 10, now, regardless of kubelet settings, only a warning is used. The `kernel.panic` parameter itself is strictly controlled by the DKP platform

### namespace-list-acl-filtering.patch

Allows users without cluster-wide `list/get namespaces` to receive an ACL-filtered response for `GET /api/v1/namespaces` and `GET /api/v1/namespaces/{name}`.
The kube-apiserver authorization filter bypasses the initial 403 for these requests and delegates filtering to the Namespace storage.
The storage queries the aggregated extension API `authorization.deckhouse.io/v1alpha1` resource `accessiblenamespaces` served by the `permission-browser-apiserver` APIService (`v1alpha1.authorization.deckhouse.io`) and returns only accessible namespaces.

If `permission-browser-apiserver` is not present/unavailable (APIService is not `Available=True` or request fails), the behavior falls back to vanilla Kubernetes (403 for users without permissions). `watch namespaces` is not changed.

### kubelet-inappropriate-manifest-name.patch

This patch ensures that files like `kube-apiserver.backup`, `kube-apiserver.yaml.bak`, or any other non-YAML files are not processed as static pod manifests,
this prevents kubelet from accidentally processing backup files or other non-manifest files in the `/etc/kubernetes/manifests directory`.
See issues:
- https://github.com/kubernetes/kubernetes/issues/55596
- https://github.com/kubernetes/kubernetes/issues/129364 -> https://github.com/kubernetes/kubernetes/pull/105695

### set-usage-GOPROXY.patch

Removes GOPROXY=off from the build so that our value is used when building the image.
