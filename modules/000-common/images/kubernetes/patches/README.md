## Patches

Warning! Some required patches that extend kubernetes functionally located in `ee/modules/000-common/images/kubernetes/patches/`
directory.

### local-init-configuration.patch

We want to include in join data the following:
```yaml
apiVersion: kubeadm.k8s.io/v1beta2
kind: InitConfiguration
localAPIEndpoint:
  advertiseAddress: {{ .nodeIP | quote }}
  bindPort: 6443
```

> Consider finding a way to do it without patching the `kubeadm` or make a PR to the upstream.

### kubeadm-etcd-join.patch

Unhides the `etcd-join` sub-phase under `kubeadm join phase control-plane-join` for Kubernetes >= 1.33 and makes it version-aware. This allows for control-plane scaling when the `ControlPlaneKubeletLocalMode` feature gate is enabled (default in 1.33+), which otherwise breaks the standard etcd joining workflow.

> Upstream PRs:
> - https://github.com/kubernetes/kubernetes/pull/135481
> - https://github.com/kubernetes/kubernetes/pull/135482

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
