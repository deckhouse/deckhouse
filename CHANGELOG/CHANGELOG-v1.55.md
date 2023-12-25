# Changelog v1.55


## Know before update


 - All cilium pods will restart. It could be regressions with network policies. There are obsolete CRDs: `CiliumEgressNATPolicy` and `CiliumBGPLoadBalancerIPPool`.
 - Clusters, created after 1.55 Deckhouse release will have Baseline Pod Security Standard by default.
 - `azuredisk-csi` pods will restart.
 - `cinder-csi-plugin` (`cloud-provider-openstack` module) pods will restart.
 - `ebs-csi-plugin` pods (`cloud-provider-aws` module) will restart.
 - `pd-csi-plugin` pods (`cloud-provider-gcp module`) will restart.
 - `vsphere-csi-plugin` pods will restart.
 - `yandex-csi-plugin` pods will restart.

## Features


 - **[admission-policy-engine]** Make default PSS policy customizable. [#6528](https://github.com/deckhouse/deckhouse/pull/6528)
    Clusters, created after 1.55 Deckhouse release will have Baseline Pod Security Standard by default.
 - **[admission-policy-engine]** Provide a way for specifying alternative pod security standards enforcement actions. [#6355](https://github.com/deckhouse/deckhouse/pull/6355)
    Pod security standards constraints will be renamed to fit new name schema. It does not affect anything while you don't use raw PSS constraints.
 - **[admission-policy-engine]** Additional status fields for custom resource `SecurityPolicy`. [#5274](https://github.com/deckhouse/deckhouse/pull/5274)
 - **[basic-auth]** Nginx image is based on a distroless image. [#6395](https://github.com/deckhouse/deckhouse/pull/6395)
 - **[candi]** Parallel download registry packages in separate step before installation. [#6415](https://github.com/deckhouse/deckhouse/pull/6415)
 - **[documentation]** documentation module is based on a distroless image. [#6396](https://github.com/deckhouse/deckhouse/pull/6396)
 - **[go_lib]** Ignore `/path` when checking registry credentials. [#6433](https://github.com/deckhouse/deckhouse/pull/6433)
 - **[linstor]** Add a custom script for eviction of LINSTOR resources from a node. [#6400](https://github.com/deckhouse/deckhouse/pull/6400)
 - **[node-manager]** Alert about Yandex Cloud `ru-central-c` zone deprecation. [#6614](https://github.com/deckhouse/deckhouse/pull/6614)
 - **[node-manager]** Additional status fields for custom resource `NodeGroup`. [#5274](https://github.com/deckhouse/deckhouse/pull/5274)
 - **[prometheus]** Additional status fields for custom resource `CustomAlertManager`. [#5274](https://github.com/deckhouse/deckhouse/pull/5274)
 - **[upmeter]** Images are based on a distroless image. [#6176](https://github.com/deckhouse/deckhouse/pull/6176)

## Fixes


 - **[admission-policy-engine]** Fixed labels in anti-affinity for `gatekeeper-controller`. [#6555](https://github.com/deckhouse/deckhouse/pull/6555)
 - **[candi]** Resolve names to IPv4 addresses with d8-curl. [#6944](https://github.com/deckhouse/deckhouse/pull/6944)
 - **[candi]** Run chmod on file only if it exists. [#6880](https://github.com/deckhouse/deckhouse/pull/6880)
 - **[candi]** Handle registry packages fetch errors. [#6860](https://github.com/deckhouse/deckhouse/pull/6860)
 - **[candi]** Disable managing "foreign" ip rules by systemd-networkd. [#6561](https://github.com/deckhouse/deckhouse/pull/6561)
    systemd-networkd.service will be restarted to apply the settings.
 - **[candi]** Do not wait Instance status patch indefinitely during bootstrap. [#6551](https://github.com/deckhouse/deckhouse/pull/6551)
 - **[candi]** Fixed wait apt update. [#6040](https://github.com/deckhouse/deckhouse/pull/6040)
 - **[ceph-csi]** Use different liveness probe ports for csi-controller-cephfs and csi-controller-rbd. [#6727](https://github.com/deckhouse/deckhouse/pull/6727)
 - **[cloud-provider-azure]** Azure cloud-controller-manager has been updated to the latest versions for all supported Kubernetes versions. [#6574](https://github.com/deckhouse/deckhouse/pull/6574)
    cloud-controller-manager will restart.
 - **[cni-cilium]** Fixed pprof listen address for cilium-agents. [#6989](https://github.com/deckhouse/deckhouse/pull/6989)
    cilium-agents will restart. Some fqdn-basic network policies will blink awile.
 - **[cni-cilium]** Cilium version bumped to 1.14.5 [#6881](https://github.com/deckhouse/deckhouse/pull/6881)
    Cilium agents will restart, during restart some policies won't work.
 - **[cni-cilium]** Restore removed API versions in CRDs. [#6690](https://github.com/deckhouse/deckhouse/pull/6690)
 - **[common]** Fix vulnerabilities in csi-external-* images: `CVE-2023-44487`, `CVE-2022-41723`, `GHSA-m425-mq94-257g`. [#6313](https://github.com/deckhouse/deckhouse/pull/6313)
 - **[control-plane-manager]** Remove the use of crictl when backing up etcd. [#6720](https://github.com/deckhouse/deckhouse/pull/6720)
 - **[deckhouse-controller]** Fix getting Deckhouse version in debugging. [#6517](https://github.com/deckhouse/deckhouse/pull/6517)
 - **[deckhouse-controller]** Fix CVE issues in deckhouse-controller image. [#6393](https://github.com/deckhouse/deckhouse/pull/6393)
 - **[extended-monitoring]** Add a job to sift metrics from custom exporters. [#5996](https://github.com/deckhouse/deckhouse/pull/5996)
 - **[kube-dns]** Fixed vulnerabilities: CVE-2022-1996, CVE-2022-27664, CVE-2022-41723, CVE-2023-39325, CVE-2022-32149, CVE-2021-33194, CVE-2021-38561. [#6397](https://github.com/deckhouse/deckhouse/pull/6397)
 - **[loki]** Fix CVE issue in Loki image. [#6494](https://github.com/deckhouse/deckhouse/pull/6494)
 - **[monitoring-kubernetes]** Fix CVE issues in `node-exporter`, `kubelet-eviction-tresholds-exporter` image. [#6523](https://github.com/deckhouse/deckhouse/pull/6523)
 - **[monitoring-kubernetes]** Capacity Planning dashboard shows correct number of Pods usage [#5934](https://github.com/deckhouse/deckhouse/pull/5934)
 - **[node-manager]** Remove the validating webhook for the Node deletion operation. [#6938](https://github.com/deckhouse/deckhouse/pull/6938)
 - **[node-manager]** add NodeGroup name validation only for 'CREATE' operation. [#6879](https://github.com/deckhouse/deckhouse/pull/6879)
 - **[node-manager]** Add MachineHealthCheck for CAPS. [#6609](https://github.com/deckhouse/deckhouse/pull/6609)
 - **[node-manager]** Fix node-manager does not remove `node.deckhouse.io/unitialized` taint when using one taint with different effects. [#6671](https://github.com/deckhouse/deckhouse/pull/6671)
 - **[node-manager]** Fix nodeGroup validation webhook if global mc does not exists. [#6583](https://github.com/deckhouse/deckhouse/pull/6583)
 - **[node-manager]** Fix CVE issue in fix cve in `bashible-apiserver` image. [#6526](https://github.com/deckhouse/deckhouse/pull/6526)
 - **[operator-prometheus]** Fix CVE issues in `operator-prometheus` image. [#6456](https://github.com/deckhouse/deckhouse/pull/6456)
 - **[operator-trivy]** Fix CVE issues in `operator-trivy` image. [#6463](https://github.com/deckhouse/deckhouse/pull/6463)
 - **[prometheus]** Fixes update_alertmanager_status hook when there is an alertmanager via a labeled service in the cluster. [#6699](https://github.com/deckhouse/deckhouse/pull/6699)
 - **[prometheus]** Fix CVE issues in alertsreceiver image. [#6503](https://github.com/deckhouse/deckhouse/pull/6503)
 - **[prometheus-metrics-adapter]** Fix CVE issues in k8sPrometheusAdapter image. [#6506](https://github.com/deckhouse/deckhouse/pull/6506)
 - **[runtime-audit-engine]** Add request to search for nodes with non-working pods in `D8RuntimeAuditEngineNotScheduledInCluster` prometheus-rule. [#5946](https://github.com/deckhouse/deckhouse/pull/5946)
 - **[user-authn]** Fix vulnerabilities: CVE-2022-41721, CVE-2022-41723, CVE-2023-39325, CVE-2022-32149, GHSA-m425-mq94-257g, CVE-2021-33194, CVE-2022-27664, CVE-2022-21698, CVE-2021-43565, CVE-2022-27191, CVE-2021-38561, CVE-2020-29652, CVE-2020-7919, CVE-2020-9283, CVE-2019-9512, CVE-2019-9514, CVE-2022-3064. [#6502](https://github.com/deckhouse/deckhouse/pull/6502)
    dex and kubeconfig-generator pods will restart.
 - **[user-authz]** Fixed liveness probe for `user-authz-webhook.` [#6525](https://github.com/deckhouse/deckhouse/pull/6525)
 - **[user-authz]** Fix CVE issues in `user-authz` image. [#6473](https://github.com/deckhouse/deckhouse/pull/6473)

## Chore


 - **[candi]** Bump patch versions of Kubernetes images: `v1.25.16`, `v1.26.11`, `v1.27.8`, `v1.28.4`. [#6621](https://github.com/deckhouse/deckhouse/pull/6621)
    Kubernetes control plane components will restart, kubelet will restart.
 - **[candi]** Move caps-controller image to distroless. [#6476](https://github.com/deckhouse/deckhouse/pull/6476)
    caps-controller should be restarted
 - **[cloud-provider-aws]** `node-termination-handler` use distroless based image. [#6376](https://github.com/deckhouse/deckhouse/pull/6376)
 - **[cloud-provider-aws]** `ebs-csi-plugin` use distroless based image. [#6073](https://github.com/deckhouse/deckhouse/pull/6073)
    `ebs-csi-plugin` pods (`cloud-provider-aws` module) will restart.
 - **[cloud-provider-azure]** `azuredisk-csi` use distroless based image. [#6073](https://github.com/deckhouse/deckhouse/pull/6073)
    `azuredisk-csi` pods will restart.
 - **[cloud-provider-gcp]** `pd-csi-plugin` use distroless based image. [#6073](https://github.com/deckhouse/deckhouse/pull/6073)
    `pd-csi-plugin` pods (`cloud-provider-gcp module`) will restart.
 - **[cloud-provider-openstack]** `cinder-csi-plugin` use distroless based image. [#6073](https://github.com/deckhouse/deckhouse/pull/6073)
    `cinder-csi-plugin` (`cloud-provider-openstack` module) pods will restart.
 - **[cloud-provider-vsphere]** `vsphere-csi-plugin` and `vsphere-csi-plugin-legacy` use distroless based image. [#6073](https://github.com/deckhouse/deckhouse/pull/6073)
    `vsphere-csi-plugin` pods will restart.
 - **[cloud-provider-yandex]** `cloud-metrics-exporter` is based on distroless image. [#6377](https://github.com/deckhouse/deckhouse/pull/6377)
 - **[cloud-provider-yandex]** `yandex-csi-plugin` use distroless based image. [#6073](https://github.com/deckhouse/deckhouse/pull/6073)
    `yandex-csi-plugin` pods will restart.
 - **[cni-cilium]** Enabled pprof interface in cilium-agent. [#6883](https://github.com/deckhouse/deckhouse/pull/6883)
    cillium-agent pods will restart.
 - **[cni-cilium]** Bump cilium version to `v1.14.4`. [#6185](https://github.com/deckhouse/deckhouse/pull/6185)
    All cilium pods will restart. It could be regressions with network policies. There are obsolete CRDs: `CiliumEgressNATPolicy` and `CiliumBGPLoadBalancerIPPool`.
 - **[ingress-nginx]** Fix `HostWithFailover` dropping requests on a failover if `.spec.acceptRequestsFrom` is set. [#6428](https://github.com/deckhouse/deckhouse/pull/6428)
    Proxy-failover pods of `HostWithFailover` Ingress controllers will be recreated.
 - **[istio]** Add the `idleTimeout` parameter to ModuleСonfig to control proxy IdleTimeout. [#6581](https://github.com/deckhouse/deckhouse/pull/6581)
 - **[monitoring-kubernetes]** Bump kube-state-metrics 2.7.0. [#6521](https://github.com/deckhouse/deckhouse/pull/6521)
 - **[node-local-dns]** Is based on distroless image. [#6490](https://github.com/deckhouse/deckhouse/pull/6490)
    `node-local-dns` pods will restart.

