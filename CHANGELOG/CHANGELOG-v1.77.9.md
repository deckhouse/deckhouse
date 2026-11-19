# Changelog v1.77.9

## Fixes


 - **[cloud-provider-azure]** Disable the node-ipam controller in the cloud-controller-manager to prevent duplicate pod CIDRs and conflicting routes.
 - **[cloud-provider-openstack]** Fix OpenStackCluster rendering when network lists are missing or empty
 - **[cloud-provider-vcd]** optimize VM cache refresh and avoid blocking cache reads
 - **[cloud-provider-vcd]** preserve VM not-found errors to allow orphaned Node cleanup
 - **[cloud-provider-yandex]** quote label values in generated MachineClass manifests
 - **[common]** Update nginx in common/nginx-static to 1.30.7 to fix CVEs.
    Pods using common/nginx-static (registry-proxy, basic-auth, deckhouse-tools, documentation) will be restarted.
 - **[deckhouse-controller]** Releases of a still-embedded module no longer raise manual approval alerts once the module has a deployed release.
 - **[node-manager]** Disable HTTP keep-alives in kubernetes-api-proxy `/readyz` probes so connections close immediately after each check.
    Static pods and DaemonSet pods of `kubernetes-api-proxy` will restart.
 - **[node-manager]** Fix `StaticInstance` getting stuck in a bootstrap loop after cleanup, and status patches failing on legacy conditions.
 - **[node-manager]** Nodes trust the registry CA from a ModuleSource that has `dockerCfg` set again.
    bashible-apiserver no longer skips such ModuleSources, so their CA is added to the node configuration. Bashible will be rerun on all nodes.
 - **[node-manager]** Restore the default maxSurge behavior for NodeGroups with maxSurgePerZone set to 0.
 - **[registry-packages-proxy]** Reject CSE node packages with untrusted manifest signatures, including requests through registry aliases and cached downloads.
    The registry-packages-proxy pods will be restarted.

## Chore


 - **[registrypackages]** Update delivery-kit-sdk to v1.4.2 and remove C build dependencies from d8-bcheck.
