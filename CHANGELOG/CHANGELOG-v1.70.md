# Changelog v1.70

## [MALFORMED]


 - #12733 unknown section "cloud-data"
 - #13081 missing section, missing summary, missing type, unknown section ""
 - #13165 unknown section "cloud-data"

## Know before update


 - Dashboards and alerts based on the `falco_events` metric might be broken.
 - dhctl in commander mode will skip draining errors

## Features


 - **[candi]** Update containerd to v1.7.27 with patches and runc to v1.2.5. [#13205](https://github.com/deckhouse/deckhouse/pull/13205)
    Containerd will restart.
 - **[candi]** Add ability to reboot node if annotation update.node.deckhouse.io/reboot is set. [#13176](https://github.com/deckhouse/deckhouse/pull/13176)
 - **[candi]** add cgroup version step bashible label on node [#12911](https://github.com/deckhouse/deckhouse/pull/12911)
 - **[candi]** Deleting all users created by deckhouse from NodeUser manifests. [#12908](https://github.com/deckhouse/deckhouse/pull/12908)
 - **[candi]** Use local pinned images for sandbox and kubernetes-api-proxy. [#12804](https://github.com/deckhouse/deckhouse/pull/12804)
    kubernetes-api-proxy will be restart.
 - **[candi]** Added a `bashible` step that assigns the `node.deckhouse.io/provider-id` annotation to nodes with a `static://` provider ID [#11807](https://github.com/deckhouse/deckhouse/pull/11807)
 - **[control-plane-manager]** Add settings for etcd backup. [#13193](https://github.com/deckhouse/deckhouse/pull/13193)
 - **[dhctl]** Fail drain confirmation for commander mode returns always yes [#13292](https://github.com/deckhouse/deckhouse/pull/13292)
    dhctl in commander mode will skip draining errors
 - **[dhctl]** Add waiting for become ready first master node [#12918](https://github.com/deckhouse/deckhouse/pull/12918)
 - **[dhctl]** Using opentofu instead of terraform for YandexCloud [#12688](https://github.com/deckhouse/deckhouse/pull/12688)
 - **[docs]** implement deckhouse logger inside docs-builder [#12835](https://github.com/deckhouse/deckhouse/pull/12835)
    low
 - **[istio]** Added garbage collection of istio-ca-root-cert and IstioMulticluster/IstioFederation resources after module disabling. [#13229](https://github.com/deckhouse/deckhouse/pull/13229)
 - **[istio]** Added metrics for `IstioMulticluster` remote cluster synchronization. [#12799](https://github.com/deckhouse/deckhouse/pull/12799)
 - **[node-manager]** Add event about successful draining node before deletion. [#13258](https://github.com/deckhouse/deckhouse/pull/13258)
 - **[openvpn]** Added end-of-life alerts, CA certificate re-creation and a grafana dashboard. [#12581](https://github.com/deckhouse/deckhouse/pull/12581)

## Fixes


 - **[candi]** Fix some OpenAPI schemas for cloud discovery data. [#13035](https://github.com/deckhouse/deckhouse/pull/13035)
 - **[candi]** Support for dnf package manager [#13026](https://github.com/deckhouse/deckhouse/pull/13026)
 - **[candi]** bashible configure-kubelet step fix [#12722](https://github.com/deckhouse/deckhouse/pull/12722)
 - **[cloud-provider-dynamix]** Fix bild cloud-data-discoverer [#13141](https://github.com/deckhouse/deckhouse/pull/13141)
 - **[cloud-provider-huaweicloud]** Fix bild cloud-data-discoverer [#13141](https://github.com/deckhouse/deckhouse/pull/13141)
 - **[cloud-provider-huaweicloud]** Add the `--cluster-name` CLI flag to the `cloud-controller-manager`. [#12950](https://github.com/deckhouse/deckhouse/pull/12950)
 - **[cloud-provider-openstack]** fix terraform bastion default root_disk_size [#12924](https://github.com/deckhouse/deckhouse/pull/12924)
 - **[cloud-provider-vcd]** Trim trailing slash from `VCDClusterConfiguration.provider.server`. [#13204](https://github.com/deckhouse/deckhouse/pull/13204)
 - **[cloud-provider-vcd]** The usage of `VCDCluster.spec.proxyConfigSpec` removed. [#13138](https://github.com/deckhouse/deckhouse/pull/13138)
 - **[cni-cilium]** Added restoring/hiding network access to cilium endpoint (cep) when higher/lower priority cep was removed/added. [#12793](https://github.com/deckhouse/deckhouse/pull/12793)
 - **[deckhouse]** Apply patch releases in the maintenance window if exists. [#12935](https://github.com/deckhouse/deckhouse/pull/12935)
 - **[deckhouse]** remove system-wide proxy from `/etc/systemd/system.conf.d/` [#12832](https://github.com/deckhouse/deckhouse/pull/12832)
 - **[deckhouse]** Changed the method of connecting deckhouse-controller to API-server. [#12282](https://github.com/deckhouse/deckhouse/pull/12282)
 - **[dhctl]** Disable converge deckhouse configuration for terraform autoconverger and converge from CLI [#13226](https://github.com/deckhouse/deckhouse/pull/13226)
 - **[dhctl]** Fix checking bashible already run. [#13163](https://github.com/deckhouse/deckhouse/pull/13163)
 - **[dhctl]** Add deny additional properties for validation schema eg module config [#12889](https://github.com/deckhouse/deckhouse/pull/12889)
 - **[dhctl]** Added waiting for kubeadm command completion result [#12826](https://github.com/deckhouse/deckhouse/pull/12826)
 - **[docs]** del cloud-init from non-cloud bootstrap [#13087](https://github.com/deckhouse/deckhouse/pull/13087)
 - **[ingress-nginx]** Forbidden to enable enableIstioSidecar when HostWithFailover is enabled. [#12789](https://github.com/deckhouse/deckhouse/pull/12789)
 - **[istio]** If the `cloud-provider-huaweicloud` module is enabled, define `RBAC` permissions granting the `cloud-controller-manager` access to list pods in the `d8-istio` namespace. [#13270](https://github.com/deckhouse/deckhouse/pull/13270)
 - **[istio]** Add `RBAC` rules to grant the HuaweiCloud `cloud-controller-manager` permission to view pods in the `d8-istio` namespace. [#12951](https://github.com/deckhouse/deckhouse/pull/12951)
 - **[metallb]** Dashboards are aligned with user experience expectations. [#12666](https://github.com/deckhouse/deckhouse/pull/12666)
 - **[node-manager]** Fixed increased 403 errors from capi-controller-manager accessing the Kubernetes API server root path ('/'). [#13125](https://github.com/deckhouse/deckhouse/pull/13125)
 - **[node-manager]** Fix panic in vSphere provider during VM creation [#13083](https://github.com/deckhouse/deckhouse/pull/13083)
 - **[node-manager]** Rewrite static Node adoption for `CAPS` [#11807](https://github.com/deckhouse/deckhouse/pull/11807)
 - **[upmeter]** Add a hook for replacing old sts, increase storage capacity, and scale down retention to 13 months. [#12809](https://github.com/deckhouse/deckhouse/pull/12809)
 - **[user-authn]** DexAuthenticator with digit name fails [#12902](https://github.com/deckhouse/deckhouse/pull/12902)

## Chore


 - **[cloud-provider-vcd]** The VCD provider outputs logs in JSON format [#13183](https://github.com/deckhouse/deckhouse/pull/13183)
 - **[deckhouse]** Add module version to module source. [#13128](https://github.com/deckhouse/deckhouse/pull/13128)
 - **[dhctl]** Set additionalProperties "false" for all objects in openapi [#11832](https://github.com/deckhouse/deckhouse/pull/11832)
 - **[ingress-nginx]** Added ingress-nginx version 1.12. The defaultControllerVersion is set to 1.10, all ingress controllers without specified version will restart. [#12609](https://github.com/deckhouse/deckhouse/pull/12609)
 - **[runtime-audit-engine]** Remove deprecated `falco_events` metric. [#13228](https://github.com/deckhouse/deckhouse/pull/13228)
    Dashboards and alerts based on the `falco_events` metric might be broken.

