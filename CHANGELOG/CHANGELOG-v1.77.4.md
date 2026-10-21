# Changelog v1.77.4

## Know before update


 - Cilium agent and operator pods will be restarted.
 - CoreDNS pods will be restarted.
 - NodeGroups that already have preemptible: true under the CAPI engine keep their current regular VMs until the next rollout (any rollout-triggering InstanceClass change or manual-rollout-id); after that rollout their nodes become preemptible VMs and are rotated every ~20h.
 - Upgrade does not recreate any node. NodeGroups that already have preemptible: true under the CAPI engine keep their current regular VMs; Machines created after the upgrade (scale-up, replacement) are preemptible, so such NodeGroups run mixed VMs until a full rollout (any rollout-triggering InstanceClass change or the manual-rollout-id annotation). Preemptible VMs are rotated every ~20h.
 - metallb pods will be restarted.
 - node-local-dns pods will be restarted.

## Fixes


 - **[cilium-hubble]** Fixed known vulnerabilities in the Hubble relay and UI images. [#713](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/713)
 - **[cloud-provider-dvp]** A DVP cluster migrated to the new provider configuration format keeps its `networkPolicy - None` instead of silently switching to `Isolated`. [#845](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/845)
 - **[cloud-provider-dvp]** Clusters with `networkPolicy - None` in the provider configuration no longer get NetworkPolicies created for LoadBalancer Services. [#859](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/859)
 - **[cloud-provider-yandex]** Preemptible nodes are created as preemptible VMs and rotated before the 24h limit when a NodeGroup is managed by the CAPI engine. [#76](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/76)
    NodeGroups that already have preemptible: true under the CAPI engine keep their current regular VMs until the next rollout (any rollout-triggering InstanceClass change or manual-rollout-id); after that rollout their nodes become preemptible VMs and are rotated every ~20h.
 - **[cloud-provider-yandex]** Preemptible nodes are created as preemptible VMs and rotated before the 24h limit when a NodeGroup is managed by the CAPI engine. [#771](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/771)
    Upgrade does not recreate any node. NodeGroups that already have preemptible: true under the CAPI engine keep their current regular VMs; Machines created after the upgrade (scale-up, replacement) are preemptible, so such NodeGroups run mixed VMs until a full rollout (any rollout-triggering InstanceClass change or the manual-rollout-id annotation). Preemptible VMs are rotated every ~20h.
 - **[cni-cilium]** Fixed known vulnerabilities in the Cilium images. [#713](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/713)
    Cilium agent and operator pods will be restarted.
 - **[common]** Fixed known vulnerabilities in the CNI migration images. [#715](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/715)
 - **[deckhouse]** Bump containerd to v2.2.10 and dependencies to fix CVEs [#821](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/821)
 - **[kube-dns]** Fixed known vulnerabilities in the CoreDNS and sts-pods-hosts-appender images. [#714](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/714)
    CoreDNS pods will be restarted.
 - **[metallb]** Fixed known vulnerabilities in the MetalLB images. [#719](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/719)
    metallb pods will be restarted.
 - **[node-local-dns]** Fixed known vulnerabilities in the node-local-dns images. [#714](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/714)
    node-local-dns pods will be restarted.
 - **[registrypackages]** Fixed known vulnerabilities in the kubernetes-cni package. [#721](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/721)
