# Changelog v1.77.4

## Know before update


 - Cilium agent and operator pods will be restarted.
 - CoreDNS pods will be restarted.
 - NodeGroups that already have preemptible: true under the CAPI engine keep their current regular VMs until the next rollout (any rollout-triggering InstanceClass change or manual-rollout-id); after that rollout their nodes become preemptible VMs and are rotated every ~20h.
 - Upgrade does not recreate any node. NodeGroups that already have preemptible: true under the CAPI engine keep their current regular VMs; Machines created after the upgrade (scale-up, replacement) are preemptible, so such NodeGroups run mixed VMs until a full rollout (any rollout-triggering InstanceClass change or the manual-rollout-id annotation). Preemptible VMs are rotated every ~20h.
 - metallb pods will be restarted.
 - node-local-dns pods will be restarted.

## Fixes


 - **[cilium-hubble]** Fixed known vulnerabilities in the Hubble relay and UI images.
 - **[cloud-provider-dvp]** A DVP cluster migrated to the new provider configuration format keeps its `networkPolicy - None` instead of silently switching to `Isolated`.
 - **[cloud-provider-dvp]** Clusters with `networkPolicy - None` in the provider configuration no longer get NetworkPolicies created for LoadBalancer Services.
 - **[cloud-provider-yandex]** Preemptible nodes are created as preemptible VMs and rotated before the 24h limit when a NodeGroup is managed by the CAPI engine.
    NodeGroups that already have preemptible: true under the CAPI engine keep their current regular VMs until the next rollout (any rollout-triggering InstanceClass change or manual-rollout-id); after that rollout their nodes become preemptible VMs and are rotated every ~20h.
 - **[cloud-provider-yandex]** Preemptible nodes are created as preemptible VMs and rotated before the 24h limit when a NodeGroup is managed by the CAPI engine.
    Upgrade does not recreate any node. NodeGroups that already have preemptible: true under the CAPI engine keep their current regular VMs; Machines created after the upgrade (scale-up, replacement) are preemptible, so such NodeGroups run mixed VMs until a full rollout (any rollout-triggering InstanceClass change or the manual-rollout-id annotation). Preemptible VMs are rotated every ~20h.
 - **[cni-cilium]** Fixed known vulnerabilities in the Cilium images.
    Cilium agent and operator pods will be restarted.
 - **[common]** Fixed known vulnerabilities in the CNI migration images.
 - **[deckhouse]** Bump containerd to v2.2.10 and dependencies to fix CVEs
 - **[kube-dns]** Fixed known vulnerabilities in the CoreDNS and sts-pods-hosts-appender images.
    CoreDNS pods will be restarted.
 - **[metallb]** Fixed known vulnerabilities in the MetalLB images.
    metallb pods will be restarted.
 - **[node-local-dns]** Fixed known vulnerabilities in the node-local-dns images.
    node-local-dns pods will be restarted.
 - **[registrypackages]** Fixed known vulnerabilities in the kubernetes-cni package.
