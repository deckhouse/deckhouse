# Changelog v1.68

## [MALFORMED]


 - #10445 invalid impact level "default | high | low", invalid type "fix | feature | chore", unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #10446 invalid impact level "default | high | low", invalid type "fix | feature | chore", unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #10902 missing section, missing summary, missing type, unknown section ""
 - #11040 unknown section "cloud-provider-zvirt"
 - #11077 missing section, missing summary, missing type, unknown section ""
 - #11108 unknown section "monitoring"
 - #11135 unknown section "monitoring-kubernetes]"
 - #11148 unknown section "chore"
 - #11214 missing section, missing summary, missing type, unknown section ""
 - #11231 unknown section "registry-pakages-proxy"
 - #11241 unknown section "core"
 - #11252 unknown section "tesing"
 - #11261 invalid impact level "default | high | low", invalid type "fix | feature | chore", unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #11273 invalid impact level "default | high | low", invalid type "fix | feature | chore", unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #11291 unknown section "chore"
 - #11309 unknown section "core"
 - #11399 unknown section "monitoring"
 - #11404 missing section, missing summary, missing type, unknown section ""

## Know before update


 - All authenticators deployed by deckhouse will inherit the highAvailable option from the module instead of using the highAvailable option value of the user-authn. It means that if, e.g., the prometheus module is running in non HA mode, the DexAuthenticator instance will also be deployed without HA.

## Features


 - **[candi]** Added a way to control node labels from files, stored in local directory and it subdirectories. [#10905](https://github.com/deckhouse/deckhouse/pull/10905)
 - **[control-plane-manager]** Change of logic detection enough free space for etcd-backup. [#11170](https://github.com/deckhouse/deckhouse/pull/11170)
 - **[dhctl]** Parallel bootstrap cloud permanent node groups. [#11031](https://github.com/deckhouse/deckhouse/pull/11031)
 - **[extended-monitoring]** Added IAM authentication support for ECR in image-availability-exporter. [#10893](https://github.com/deckhouse/deckhouse/pull/10893)
 - **[log-shipper]** Add keepDeletedFilesOpenedFor option. Now it is possible to configure how long Vector keeps deleted file in case of unavailability of a log storage (when Vector cannot send logs to a storage and the internal buffer is already filled). Before the change, Vector used to hold log files opened indefinitely, which can cause a node outage by flooding the disk space. The option makes this behaviour configurable. [#10641](https://github.com/deckhouse/deckhouse/pull/10641)
 - **[upmeter]** Added `backup.deckhouse.io/cluster-config` label to relevant module CRDs. [#10568](https://github.com/deckhouse/deckhouse/pull/10568)
 - **[user-authn]** Add HA option to DexAuthenticator CRD. [#11049](https://github.com/deckhouse/deckhouse/pull/11049)
    All authenticators deployed by deckhouse will inherit the highAvailable option from the module instead of using the highAvailable option value of the user-authn. It means that if, e.g., the prometheus module is running in non HA mode, the DexAuthenticator instance will also be deployed without HA.

## Fixes


 - **[cloud-provider-dynamix]** provider RBAC-for-us fixes. [#11235](https://github.com/deckhouse/deckhouse/pull/11235)
 - **[local-path-provisioner]** Fix security context for helperPod. [#11322](https://github.com/deckhouse/deckhouse/pull/11322)
 - **[okmeter]** Remove RBAC from the okmeter module [#10323](https://github.com/deckhouse/deckhouse/pull/10323)
    okmeter agents are no longer able to access the Kubernetes API
 - **[prometheus]** Fix alerts-receiver CVE [#11270](https://github.com/deckhouse/deckhouse/pull/11270)
 - **[prometheus]** Fix alerts-receiver CVE [#11257](https://github.com/deckhouse/deckhouse/pull/11257)

## Chore


 - **[admission-policy-engine]** Update gatekeeper and refactor build. [#11400](https://github.com/deckhouse/deckhouse/pull/11400)
 - **[admission-policy-engine]** Update gatekeeper and refactor build. [#11356](https://github.com/deckhouse/deckhouse/pull/11356)
 - **[cert-manager]** Update build and bump version to 1.16.2 [#11198](https://github.com/deckhouse/deckhouse/pull/11198)
 - **[control-plane-manager]** Replace Logrus to slog implementation. [#10909](https://github.com/deckhouse/deckhouse/pull/10909)
    Restart components.
 - **[descheduler]** Update build and fix CVE [#11221](https://github.com/deckhouse/deckhouse/pull/11221)
 - **[dhctl]** Replace Logrus to slog implementation, delete 'simple' logger. [#10909](https://github.com/deckhouse/deckhouse/pull/10909)
 - **[ingress-nginx]** Disable VPA for Istio sidecar [#11084](https://github.com/deckhouse/deckhouse/pull/11084)
 - **[ingress-nginx]** Added a release requirement check for defaultControllerVersion parameter. [#10941](https://github.com/deckhouse/deckhouse/pull/10941)
 - **[ingress-nginx]** Filter for vhost with multiple ingress controllers in grafana [#10847](https://github.com/deckhouse/deckhouse/pull/10847)
 - **[node-local-dns]** Replace Logrus to slog implementation. [#10909](https://github.com/deckhouse/deckhouse/pull/10909)
    Restart components.
 - **[openvpn]** Replace Logrus to slog implementation. [#10909](https://github.com/deckhouse/deckhouse/pull/10909)
    Restart components.
 - **[prometheus]** Service Account permission for observability label-proxy to prometheus. [#11333](https://github.com/deckhouse/deckhouse/pull/11333)
    low
 - **[prometheus]** Deprecate Grafana v8 [#10359](https://github.com/deckhouse/deckhouse/pull/10359)
 - **[registry-packages-proxy]** Replace Logrus to slog implementation. [#10909](https://github.com/deckhouse/deckhouse/pull/10909)
    Restart components.
 - **[registrypackages]** Upgrade jq to 1.7.1 version. [#11370](https://github.com/deckhouse/deckhouse/pull/11370)
 - **[runtime-audit-engine]** Refactor build and fix some CVE's in runtime-audit-engine module [#11290](https://github.com/deckhouse/deckhouse/pull/11290)
 - **[runtime-audit-engine]** Refactor build and fix some CVE's in runtime-audit-engine module [#11260](https://github.com/deckhouse/deckhouse/pull/11260)
 - **[vertical-pod-autoscaler]** Update build and fix CVE [#11219](https://github.com/deckhouse/deckhouse/pull/11219)

