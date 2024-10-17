# Changelog v1.65

## [MALFORMED]


 - #9703 unknown section "moniotring"
 - #9739 unknown section "candi,control-plane-manager,kube-dns"
 - #9852 missing section, missing summary, missing type, unknown section ""
 - #9909 missing section, missing summary, missing type, unknown section ""
 - #9944 unknown section "docs, documentation"
 - #9966 unknown section "cloud-provider-openstack,cloud-provider-vcd,cloud-provider-yandex,cloud-provider-vsphere,cloud-provider-zvirt,istio"
 - #9973 unknown section "ci, docs"
 - #9985 unknown section "deckhouse-controller, docs, go_lib"
 - #9999 unknown section "declhouse"
 - #10021 missing section, missing summary, missing type, unknown section ""
 - #10046 unknown section "deckhouse-controller, go-lib"
 - #10060 missing section, missing summary, missing type, unknown section ""
 - #10077 invalid impact level "default | high | low", invalid type "fix | feature | chore", unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #10125 unknown section "chore"
 - #10149 missing summary
 - #10179 unknown section "openvpn, operator-trivy"

## Know before update


 - Restart containerd.

## Features


 - **[candi]** Add support for openSUSE and mosOS. [#9436](https://github.com/deckhouse/deckhouse/pull/9436)
 - **[candi]** Install CA certificates on nodes using d8-ca-updater, which is installed from the registrypackages. [#9246](https://github.com/deckhouse/deckhouse/pull/9246)
 - **[candi]** Update containerd to 1.7.20. [#9246](https://github.com/deckhouse/deckhouse/pull/9246)
    Restart containerd.
 - **[ceph-csi]** Make ceph-csi module deprecated. [#10009](https://github.com/deckhouse/deckhouse/pull/10009)
 - **[cloud-provider-aws]** Added the ability to specify your IAM role. [#9530](https://github.com/deckhouse/deckhouse/pull/9530)
 - **[cni-cilium]** Adding support for configuring each node individually using CiliumNodeConfig resources. [#9754](https://github.com/deckhouse/deckhouse/pull/9754)
 - **[control-plane-manager]** patch etcd to support outputting of snapshots to stdout [#9948](https://github.com/deckhouse/deckhouse/pull/9948)
 - **[control-plane-manager]** Add CronJob that does etcd backup. [#9637](https://github.com/deckhouse/deckhouse/pull/9637)
 - **[deckhouse]** Get rid of the rbacgen tool. [#9622](https://github.com/deckhouse/deckhouse/pull/9622)
 - **[deckhouse]** Extend Deckhouse update settings. [#9314](https://github.com/deckhouse/deckhouse/pull/9314)
 - **[deckhouse-controller]** Added backup.deckhouse.io/cluster-config label to Deckhouse CRD [#10111](https://github.com/deckhouse/deckhouse/pull/10111)
 - **[deckhouse-controller]** Now, if there are several factors limiting deployment, all reasons with the nearest possible moment of deployment will be indicated in the release status. In addition, sending metrics about blocked module releases has been removed if the corresponding module is inactive [#9988](https://github.com/deckhouse/deckhouse/pull/9988)
 - **[deckhouse-controller]** Add discovered GVKs from modules' CRDs to global values. [#9963](https://github.com/deckhouse/deckhouse/pull/9963)
 - **[deckhouse-controller]** adding an alert that manual confirmation is required to install mr [#9943](https://github.com/deckhouse/deckhouse/pull/9943)
 - **[deckhouse-controller]** Get rid of crd modules. [#9593](https://github.com/deckhouse/deckhouse/pull/9593)
 - **[deckhouse-controller]** Improve module validation. [#9293](https://github.com/deckhouse/deckhouse/pull/9293)
 - **[dhctl]** Upon editing configuration secrets, create them if they are missing from cluster [#9689](https://github.com/deckhouse/deckhouse/pull/9689)
 - **[dhctl]** Reduces code duplication in the gRPC server message handler and log sender, refactors the graceful shutdown mechanism, and adds support for proper log output for multiple parallel instances of the dhctl server. [#9096](https://github.com/deckhouse/deckhouse/pull/9096)
 - **[dhctl]** Reduce manual operations when converging control plane nodes. [#8380](https://github.com/deckhouse/deckhouse/pull/8380)
 - **[multitenancy-manager]** Add projects render validation. [#9607](https://github.com/deckhouse/deckhouse/pull/9607)
 - **[operator-trivy]** Bump operator-trivy version to `0.22.0`. [#10045](https://github.com/deckhouse/deckhouse/pull/10045)
 - **[user-authn]** refresh groups on updating tokens [#9598](https://github.com/deckhouse/deckhouse/pull/9598)

## Fixes


 - **[candi]** candi/version_map.yml updated to use the latest changes in yandex-cloud-controller-manager [#9855](https://github.com/deckhouse/deckhouse/pull/9855)
 - **[candi]** Step "check_hostname_uniqueness" works without temporary files creation [#9756](https://github.com/deckhouse/deckhouse/pull/9756)
 - **[candi]** added statically built lsblk [#9666](https://github.com/deckhouse/deckhouse/pull/9666)
 - **[candi]** Added the ability to configure Node DNS servers via the Azure Cloud Provider. [#9554](https://github.com/deckhouse/deckhouse/pull/9554)
 - **[cloud-provider-vcd]** Fix vcd catalogs sharing. [#9802](https://github.com/deckhouse/deckhouse/pull/9802)
 - **[cloud-provider-yandex]** Add support a hybrid cluster in yandex CSI driver [#9861](https://github.com/deckhouse/deckhouse/pull/9861)
 - **[control-plane-manager]** Automatically regenerate kubeconfig for control plane components if validation fails, preventing crashes. [#9445](https://github.com/deckhouse/deckhouse/pull/9445)
 - **[deckhouse]** Fix for scaling down of webhook-handler deployment when ha mode is disabled [#9978](https://github.com/deckhouse/deckhouse/pull/9978)
 - **[deckhouse-controller]** Fixed update logic in various modes [#10105](https://github.com/deckhouse/deckhouse/pull/10105)
 - **[deckhouse-controller]** Update the documentation about the list of data the `collect-debug-info` command collects. [#10028](https://github.com/deckhouse/deckhouse/pull/10028)
 - **[dhctl]** Deny use defaultCRI type as Docker [#10022](https://github.com/deckhouse/deckhouse/pull/10022)
 - **[dhctl]** Fix lease locking. [#9982](https://github.com/deckhouse/deckhouse/pull/9982)
 - **[dhctl]** Add validation for ClusterConfiguration.cloud.prefix [#9858](https://github.com/deckhouse/deckhouse/pull/9858)
 - **[dhctl]** Added repo check to validateRegistryDockerCfg [#9688](https://github.com/deckhouse/deckhouse/pull/9688)
 - **[dhctl]** Break circle and output error in log on check dependencies if get first error [#9679](https://github.com/deckhouse/deckhouse/pull/9679)
 - **[go_lib]** add probe to the cloud-data reconciler [#9915](https://github.com/deckhouse/deckhouse/pull/9915)
 - **[go_lib]** cloud-data-discoverer continues its operation despite temporary issues within the cluster. [#9570](https://github.com/deckhouse/deckhouse/pull/9570)
 - **[kube-dns]** Graceful rollout of the `kube-dns` deployment without disrupting connections. [#9565](https://github.com/deckhouse/deckhouse/pull/9565)
 - **[monitoring-kubernetes]** add tag main for dashboard [#9677](https://github.com/deckhouse/deckhouse/pull/9677)
    dashbord can be seen on the home page
 - **[monitoring-kubernetes]** Fixed formula for triggering alerts `KubeletNodeFSInodesUsage` and `KubeletImageFSInodesUsage`. [#9436](https://github.com/deckhouse/deckhouse/pull/9436)
 - **[multitenancy-manager]** Fix prometheus labels for ingress traffic in Project templates. [#10117](https://github.com/deckhouse/deckhouse/pull/10117)
 - **[multitenancy-manager]** Change logs format to json format. [#9955](https://github.com/deckhouse/deckhouse/pull/9955)
 - **[node-manager]** Fixed several RBAC resources in the node-manager module. [#9596](https://github.com/deckhouse/deckhouse/pull/9596)
 - **[prometheus]** Fix labels for prometheus pod antiAffinity. [#10117](https://github.com/deckhouse/deckhouse/pull/10117)
 - **[prometheus]** Fix stuck GrafanaDashboardDeprecation alerts [#10024](https://github.com/deckhouse/deckhouse/pull/10024)
 - **[user-authn]** Trim spaces from email field on the login form. [#10057](https://github.com/deckhouse/deckhouse/pull/10057)

## Chore


 - **[cni-cilium]** Updating `cilium` and its components to version 1.14.14 [#9650](https://github.com/deckhouse/deckhouse/pull/9650)
    All cilium pods will be restarted.
 - **[common]** Create image for init containers. [#9992](https://github.com/deckhouse/deckhouse/pull/9992)
 - **[common]** Bump shell-operator to optimize conversion hooks in the webhook-handler. [#9983](https://github.com/deckhouse/deckhouse/pull/9983)
 - **[deckhouse-controller]** Remove the flant-integration internal module. [#8392](https://github.com/deckhouse/deckhouse/pull/8392)
 - **[dhctl]** Remove support for deprecated 'InitConfiguration.configOverrides' parameter. [#9920](https://github.com/deckhouse/deckhouse/pull/9920)
 - **[ingress-nginx]** Remove v1.6 IngressNginxController. [#9935](https://github.com/deckhouse/deckhouse/pull/9935)
 - **[ingress-nginx]** Update kruise controller to v1.7.2. [#9898](https://github.com/deckhouse/deckhouse/pull/9898)
    kriuse controller will be restarted, pods of an ingress nginx controller of v1.10 will be recreated.
 - **[monitoring-deckhouse]** Add Debian 10 to the `D8NodeHasDeprecatedOSVersion` alert. [#9798](https://github.com/deckhouse/deckhouse/pull/9798)
 - **[monitoring-kubernetes]** Update kube-state-metrics to 2.13 [#10003](https://github.com/deckhouse/deckhouse/pull/10003)
 - **[node-manager]** Fix the module's snapshots debugging. [#9995](https://github.com/deckhouse/deckhouse/pull/9995)
 - **[node-manager]** Declarative binding of SSHCredentials and StaticInstance [#9369](https://github.com/deckhouse/deckhouse/pull/9369)
 - **[prometheus]** move externalLabels to remoteWrite section [#9752](https://github.com/deckhouse/deckhouse/pull/9752)

