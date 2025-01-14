## Patches

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
