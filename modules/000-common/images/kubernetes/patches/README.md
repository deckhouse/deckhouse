## Patches

Warning! Some required patches that extend kubernetes functionally located in `ee/modules/000-common/images/kubernetes/patches/`
directory.

### pdb-daemonset.patch

Supports DaemonSets in disruption controller by adding /scale subresource to daemonsets API. It allows to control the eviction rate of DaemonSet pods.

> Upstream PR https://github.com/kubernetes/kubernetes/pull/98307.

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

### fix-mount-hostaliases.patch

Fixes a bug where pods with hostNetwork ignored host aliases (k8s < 1.32):
> https://github.com/kubernetes/kubernetes/pull/126460
