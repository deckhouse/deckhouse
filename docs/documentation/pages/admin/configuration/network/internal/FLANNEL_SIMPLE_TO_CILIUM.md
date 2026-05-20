---
title: "Switching CNI from Flannel or Simple bridge to Cilium"
permalink: en/admin/configuration/network/internal/flannel-simple-to-cilium.html
---

{% alert level="info" %}
This guide is applicable to Deckhouse Kubernetes Platform version 1.75 and earlier.

For DKP version 1.76 and later, use the [Switching CNI in the cluster](/products/kubernetes-platform/guides/cni-migration.html) guide.
{% endalert %}

{% alert level="warning" %}
The migration requires a **full cluster downtime**: all pods and nodes will be restarted. Estimated time to complete is **about 1 hour**.
{% endalert %}

To perform the migration, follow these steps:

1. Enable the `cni-cilium` module:

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

1. Wait for the Cilium components to start:

   ```shell
   d8 k -n d8-cni-cilium get pods
   ```

   Example output:

   ```console
   NAME                      READY STATUS  RESTARTS    AGE
   agent-5zzfv               2/2   Running 5 (23m ago) 26m
   agent-gqb2b               2/2   Running 5 (23m ago) 26m
   agent-wtv4p               2/2   Running 5 (23m ago) 26m
   operator-856d69fd49-mlglv 2/2   Running 0           26m
   safe-agent-updater-26qpk  3/3   Running 0           26m
   safe-agent-updater-qlbrh  3/3   Running 0           26m
   safe-agent-updater-wjjr5  3/3   Running 0           26m
   ```

   If the Cilium agent pods do not transition to the `Ready` state:

   - Save the `d8-kube-proxy` manifest:

     ```shell
     d8 k -n kube-system get ds d8-kube-proxy -oyaml > d8-kube-proxy.yaml
     ```

   - Run the recovery actions:

     ```shell
     d8 k delete validatingadmissionpolicies.admissionregistration.k8s.io label-objects.deckhouse.io
     d8 k -n kube-system delete ds d8-kube-proxy
     d8 k -n d8-cni-cilium delete po -l app=agent
     ```

   - If Cilium startup still takes more than 5 minutes, restart `kube-proxy`:

     ```shell
     d8 k -n kube-system delete pod -l k8s-app=kube-proxy
     ```

1. Save the DaemonSet manifest:

   ```shell
   d8 k -n d8-cni-simple-bridge get ds simple-bridge -o yaml > simple-bridge.yaml
   # or
   d8 k -n d8-cni-flannel get ds flannel -o yaml > flannel.yaml
   ```

1. Disable `cni-simple-bridge` or `cni-flannel`. Perform this step only after the Cilium pods have reached the `Ready` state:

   ```shell
   d8 k -n d8-system exec -it svc/deckhouse-leader -- deckhouse-controller module disable cni-simple-bridge
   # or
   d8 k -n d8-system exec -it svc/deckhouse-leader -- deckhouse-controller module disable cni-flannel
   ```

1. Delete the `validating webhook` (if present):

   ```shell
   d8 k delete validatingwebhookconfigurations.admissionregistration.k8s.io d8-deckhouse-validating-webhook-handler-hooks
   ```

1. Delete the namespace of the old CNI (if it was not removed automatically):

   ```shell
   d8 k delete ns d8-cni-simple-bridge
   # or
   d8 k delete ns d8-cni-flannel
   ```

1. (Optional) Clean up artifacts of the old CNI. If required, remove the interface and flush iptables rules:

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

1. Restart all pods:

   ```shell
   d8 k delete pods -A --all --wait=false
   ```

1. Configure internal LoadBalancers (if used). For all TargetGroups that belong to the internal load balancer, set:

   ```text
   Preserve client IP addresses = Off
   ```

1. Reboot the cluster nodes one by one, including the master nodes.
