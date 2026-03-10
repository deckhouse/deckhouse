---
title: "Migration from cni-simple-bridge/cni-flannel to cni-cilium"
description: "Step-by-step instructions for migrating from cni-simple-bridge/cni-flannel to cni-cilium"
permalink: en/user/network/cni_migration.html
---

{% alert level="info" %}
Instructions for Deckhouse Kubernetes Platform versions <= 1.74
{% endalert %}

{% alert level="warning" %}
The work involves complete cluster downtime, including restart of all pods and all nodes.
Expected completion time: ~1 hour.
{% endalert %}

1) Enable the `cni-cilium` module:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cni-cilium
spec:
  version: 1
  settings:
    tunnelMode: VXLAN
  enabled: true
```

2) If Cilium agents fail to start, follow these steps to restart them:

```shell
# Make a backup; resources will be recreated automatically during the switchover
kubectl -n kube-system get ds d8-kube-proxy -oyaml > d8-kube-proxy.yaml
# Fixing Cilium agents crashloop
kubectl delete validatingadmissionpolicies.admissionregistration.k8s.io label-objects.deckhouse.io
kubectl -n kube-system delete ds d8-kube-proxy
kubectl -n d8-cni-cilium delete po -l app=agent
```

A prolonged startup (more than 5 minutes) may occur — a proxy restart is required.

```shell
kubectl -n kube-system delete po -l k8s-app=kube-proxy
```

3) Disable the `cni-simple-bridge`/`cni-flannel` module (after all `Cilium` pods are running).
At this stage, the webhook-handler pod may enter CrashLoopBackOff.

```shell
# Make a backup; resources will be recreated automatically during the switchover
kubectl -n d8-cni-simple-bridge get ds simple-bridge -oyaml > simple-bridge.yaml
# Or
kubectl -n d8-cni-flannel get ds flannel -oyaml > flannel.yaml
```

```shell
# Disable cni-simple-bridge
kubectl delete validatingwebhookconfigurations.admissionregistration.k8s.io d8-deckhouse-validating-webhook-handler-hooks
kubectl exec -it -n d8-system -it svc/deckhouse-leader -- deckhouse-controller module disable cni-simple-bridge
# Or
kubectl exec -it -n d8-system -it svc/deckhouse-leader -- deckhouse-controller module disable cni-flannel
```

```shell
# Should be deleted automatically; if not, delete manually. May also hang in Terminating state.
kubectl delete ns d8-cni-simple-bridge
# Or
kubectl delete ns d8-cni-flannel
```

4) After confirming that `cni-simple-bridge`/`cni-flannel` does not restart, edit iptables:
This step is optional. It is expected that during reboot, without deleting the interface and iptables rules, everything will work correctly. However, for guaranteed results, interface and rule deletion is provided as an example.

```shell
ip link delete cni0
iptables -t filter -F
iptables -t filter -X
iptables -t nat -F
iptables -t nat -X
iptables -t mangle -F
iptables -t mangle -X
iptables -t raw -F
iptables -t raw -X
iptables -P INPUT ACCEPT
iptables -P FORWARD ACCEPT
iptables -P OUTPUT ACCEPT
```

5) Restart all pods:

```shell
kubectl delete pods -A --all --wait=false
```

6) For internal load balancers, set the "Preserve client IP addresses" toggle to **Off** for all TargetGroups associated with that balancer.

7) Restart all nodes in the cluster one by one (including master nodes).
