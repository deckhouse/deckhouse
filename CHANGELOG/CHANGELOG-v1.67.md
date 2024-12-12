# Changelog v1.67

## [MALFORMED]


 - #9658 missing section, missing summary, missing type, unknown section ""
 - #10550 invalid impact level "default | high | low", invalid type "fix | feature | chore", unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #10578 invalid type "core"
 - #10593 unknown section "global"
 - #10617 invalid type "feat"
 - #10617 invalid type "feat"
 - #10617 invalid type "feat"
 - #10676 unknown section "node-manager candi"
 - #10736 missing type, unknown section "030-cloud-provider-dynamix"
 - #10852 unknown section "global"
 - #10882 invalid type "fchore"
 - #10916 unknown section "doc"
 - #10942 unknown section "tests"
 - #10952 unknown section "chore"
 - #10954 missing section, missing summary, missing type, unknown section ""
 - #11004 missing section, missing summary, missing type, unknown section ""

## Know before update


 - All modules with distroless image will be restarted.
 - No longer supports basic-auth and the module is automatically disabled if basic-auth is used.

## Features


 - **[admission-policy-engine]** Update trivy-provider to support insecure/customCA registries. [#10749](https://github.com/deckhouse/deckhouse/pull/10749)
 - **[candi]** Added support of new cloud provider - HuaweiCloud. [#10142](https://github.com/deckhouse/deckhouse/pull/10142)
 - **[candi]** Preparatory phase for bashible without bundles. [#9761](https://github.com/deckhouse/deckhouse/pull/9761)
 - **[cni-cilium]** Added ebpf-powered dhcp server for Pods. [#10651](https://github.com/deckhouse/deckhouse/pull/10651)
 - **[deckhouse]** Update mpo to work with the new module system. Deprecate v1alpha1 for ModulePullOverride. [#10903](https://github.com/deckhouse/deckhouse/pull/10903)
 - **[deckhouse]** Fire an alert when a module config has an obsolete version. [#10796](https://github.com/deckhouse/deckhouse/pull/10796)
 - **[deckhouse]** Modules from sources are not installed by default anymore. All modules from sources are become visible by default. CRD `Module` spec observability improved. [#10336](https://github.com/deckhouse/deckhouse/pull/10336)
 - **[deckhouse-controller]** Installation of a module done without waiting `Manual` update approval or update windows. [#10684](https://github.com/deckhouse/deckhouse/pull/10684)
 - **[deckhouse-controller]** Improved deckhouse controller logger [#10479](https://github.com/deckhouse/deckhouse/pull/10479)
 - **[dhctl]** Add PostCloud step to verify cloud API access from master host during bootstrap [#10311](https://github.com/deckhouse/deckhouse/pull/10311)
 - **[dhctl]** Preparatory phase for bashible without bundles. [#9761](https://github.com/deckhouse/deckhouse/pull/9761)
 - **[node-manager]** Preparatory phase for bashible without bundles. [#9761](https://github.com/deckhouse/deckhouse/pull/9761)
 - **[operator-trivy]** add option for configure custom CAs for docker registries [#10768](https://github.com/deckhouse/deckhouse/pull/10768)
 - **[operator-trivy]** An option for disabling sbom generation. [#10701](https://github.com/deckhouse/deckhouse/pull/10701)
    Once set to true, ALL current SBOM reports are deleted (one-time operation).
 - **[registrypackages]** Update containerd to v1.7.24. [#10966](https://github.com/deckhouse/deckhouse/pull/10966)
 - **[service-with-healthchecks]** A new module has been added that performs additional checks. Based on the results of these checks, traffic can be directed to different internal processes internally independently and only if they are ready. [#9371](https://github.com/deckhouse/deckhouse/pull/9371)
 - **[user-authz]** New use subroles for dkp, and dvp aggregation. [#10936](https://github.com/deckhouse/deckhouse/pull/10936)
 - **[user-authz]** Replace use admin roles with use user roles for manage manager roles. [#10681](https://github.com/deckhouse/deckhouse/pull/10681)
    Reducing rights for manage manager roles.

## Fixes


 - **[candi]** fixed double default-unreachable-toleration-seconds in kubeadm ClusterConfiguration [#10828](https://github.com/deckhouse/deckhouse/pull/10828)
 - **[cert-manager]** bump cert-manager version [#10525](https://github.com/deckhouse/deckhouse/pull/10525)
 - **[cni-cilium]** Fixed package dropping issues with VXLAN and VMWare-hosted nodes. [#10087](https://github.com/deckhouse/deckhouse/pull/10087)
 - **[cni-flannel]** Fixed package dropping issues with VXLAN and VMWare-hosted nodes. [#10087](https://github.com/deckhouse/deckhouse/pull/10087)
 - **[deckhouse]** Fix source deletion error. [#10750](https://github.com/deckhouse/deckhouse/pull/10750)
 - **[descheduler]** fix and update descheduler [#10361](https://github.com/deckhouse/deckhouse/pull/10361)
    descheduler pod will be restarted
 - **[dhctl]** Fix the `master_ip_address_for_ssh` terraform output variable handling in converge. [#11039](https://github.com/deckhouse/deckhouse/pull/11039)
 - **[dhctl]** Add tasks for moduleconfigs routines for post bootstrap and creating with resources phases. [#10688](https://github.com/deckhouse/deckhouse/pull/10688)
 - **[dhctl]** fixed work with drain for nodes with kruise.io DaemonSet [#10578](https://github.com/deckhouse/deckhouse/pull/10578)
 - **[dhctl]** Fix converge through bastion [#10278](https://github.com/deckhouse/deckhouse/pull/10278)
 - **[docs]** Add required NetworkInterface AWS policies. [#10842](https://github.com/deckhouse/deckhouse/pull/10842)
 - **[helm_lib]** Updated helm_lib to 1.37.1, which should fix issue [#10950](https://github.com/deckhouse/deckhouse/pull/10950)
 - **[istio]** Fixed `IngressIstioController` CRD docs rendering. [#10581](https://github.com/deckhouse/deckhouse/pull/10581)
 - **[node-manager]** fixed the key usage with cert-authority [#10718](https://github.com/deckhouse/deckhouse/pull/10718)
 - **[runtime-audit-engine]** fix k8s labels collection from containers in syscall event source. [#10639](https://github.com/deckhouse/deckhouse/pull/10639)

## Chore


 - **[candi]** Bump patch versions of Kubernetes images: `v1.29.12`, `v1.30.8`, `v1.31.4` [#11046](https://github.com/deckhouse/deckhouse/pull/11046)
    Kubernetes control-plane components will restart, kubelet will restart.
 - **[candi]** Update scratch image. [#10921](https://github.com/deckhouse/deckhouse/pull/10921)
    All modules with distroless image will be restarted.
 - **[candi]** reduced the use of apt and yum [#10867](https://github.com/deckhouse/deckhouse/pull/10867)
 - **[candi]** Update Deckhouse CLI to v0.6.1 [#10669](https://github.com/deckhouse/deckhouse/pull/10669)
 - **[cloud-provider-aws]** removed legacy "098_remove_cbr0.sh.tpl" step [#10888](https://github.com/deckhouse/deckhouse/pull/10888)
 - **[cloud-provider-gcp]** removed legacy "098_remove_cbr0.sh.tpl" step [#10888](https://github.com/deckhouse/deckhouse/pull/10888)
 - **[cloud-provider-yandex]** removed legacy "098_remove_cbr0.sh.tpl" step [#10888](https://github.com/deckhouse/deckhouse/pull/10888)
 - **[dashboard]** Updated to 7.10.0 [#10301](https://github.com/deckhouse/deckhouse/pull/10301)
    No longer supports basic-auth and the module is automatically disabled if basic-auth is used.
 - **[deckhouse-controller]** Refactor release processing. [#10268](https://github.com/deckhouse/deckhouse/pull/10268)
 - **[docs]** Get rid of numeric prefixes in module URL. [#10561](https://github.com/deckhouse/deckhouse/pull/10561)
 - **[docs]** Add Deckhouse Virtualization Platform documentation. [#10223](https://github.com/deckhouse/deckhouse/pull/10223)
 - **[documentation]** Get rid of numeric prefixes in module URL. [#10561](https://github.com/deckhouse/deckhouse/pull/10561)
 - **[global-hooks]** Move `global.storageClass` to `global.modules.storageClass`. [#9859](https://github.com/deckhouse/deckhouse/pull/9859)
 - **[ingress-nginx]** Minor nginx dashboard improvements [#10800](https://github.com/deckhouse/deckhouse/pull/10800)
 - **[ingress-nginx]** Minor VHost dashboard improvements [#10370](https://github.com/deckhouse/deckhouse/pull/10370)
 - **[node-manager]** Rewrite NodeGroup convesion webhook on Python. [#10777](https://github.com/deckhouse/deckhouse/pull/10777)
 - **[operator-trivy]** Use local policies. [#10799](https://github.com/deckhouse/deckhouse/pull/10799)
 - **[user-authz]** Replace manage capabilities and scopes. [#10810](https://github.com/deckhouse/deckhouse/pull/10810)

