---
title: "Switching CNI from Flannel to Cilium"
permalink: en/admin/configuration/network/internal/flannel-to-cilium.html
---

## Procedure for switching CNI from Flannel to Cilium

1. Disable the [`kube-proxy`](/modules/kube-proxy/) module:

   ```shell
   d8 k apply -f - << EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: kube-proxy
   spec:
     enabled: false
   EOF
   ```

1. Enable the [`cni-cilium`](/modules/cni-cilium/) module:

   ```shell
   d8 k create -f - << EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: cni-cilium
   spec:
     version: 1
     enabled: true
     settings:
     tunnelMode: VXLAN
   EOF
   ```

1. Check that all Cilium agents are in the `Running` status.

   ```shell
   d8 k get po -n d8-cni-cilium
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

1. Reboot master nodes.

1. Reboot the other cluster nodes.

   > If Cilium agents can't reach the `Running` status, reboot the associated nodes.

1. Disable the [`cni-flannel`](/modules/cni-flannel/) module:

   ```shell
   d8 k apply -f - << EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: cni-flannel
   spec:
     enabled: false
   EOF
   ```

1. Enable the [`node-local-dns`](/modules/node-local-dns/) module:

   ```shell
   d8 k apply -f - << EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: node-local-dns
   spec:
     enabled: true
   EOF
   ```

   Once you enable the module, wait until all Cilium agents are in the `Running` status.

1. Check that the switching of the CNIs was completed successfully.

## Ensuring the CNI was successfully switched

To ensure the CNI switching from Flannel to Cilium was completed successfully, follow these steps:

1. Check the Deckhouse queue:

   - If using a single master node:

     ```shell
     d8 system queue list
     ```

   - If using a multi-master installation:

     ```shell
     d8 system queue list
     ```

1. Check the Cilium agents. They must be in the `Running` status:

   ```shell
   d8 k get po -n d8-cni-cilium
   ```

   Example output:

   ```console
   NAME        READY STATUS  RESTARTS    AGE
   agent-5zzfv 2/2   Running 5 (23m ago) 26m
   agent-gqb2b 2/2   Running 5 (23m ago) 26m
   agent-wtv4p 2/2   Running 5 (23m ago) 26m
   ```

1. Check that the `cni-flannel` module has been disabled:

   ```shell
   d8 k get modules | grep flannel
   ```

   Example output:

   ```console
   cni-flannel                         35     Disabled    Embedded
   ```

1. Check that the `node-local-dns` module has been enabled:

   ```shell
   d8 k get modules | grep node-local-dns
   ```

   Example output:

   ```console
   node-local-dns                      350    Enabled     Embedded     Ready
   ```
