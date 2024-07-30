# Changelog v1.56

## Know before update


 - Ingress controller pods will restart.
 - Resources in the project namespaces (the `multitenancy-manager`) will lose prefix of the project. It would affect `secretRef`, `configMapRef` or other resource references if you use it.
 - Uninstalling the virtualization module will destroy all the virtual machines created with the module. Take care to save virtual machine data if necessary.

## Features


 - **[admission-policy-engine]** Operation policy for replica value checking. [#6775](https://github.com/deckhouse/deckhouse/pull/6775)
 - **[candi]** Add support for the `ru-central1-d` zone in Yandex Cloud. [#6652](https://github.com/deckhouse/deckhouse/pull/6652)
 - **[deckhouse]** Added annotation `release.deckhouse.io/apply-now` which allows to apply the update without waiting for time restrictions. [#6651](https://github.com/deckhouse/deckhouse/pull/6651)
 - **[deckhouse-controller]** Add new resource ModulePullOverride to pull and apply modules in the development mode. [#6781](https://github.com/deckhouse/deckhouse/pull/6781)
 - **[dhctl]** Support for mirroring Deckhouse CSE. [#7017](https://github.com/deckhouse/deckhouse/pull/7017)
 - **[dhctl]** Mirror flag to allow source registry selection. [#6889](https://github.com/deckhouse/deckhouse/pull/6889)
 - **[dhctl]** dhctl will now resume interrupted pulls if pulled data is fresh enough. [#6810](https://github.com/deckhouse/deckhouse/pull/6810)
 - **[dhctl]** Calculate Streebog GOST checksum for tar bundle only if asked to. [#6784](https://github.com/deckhouse/deckhouse/pull/6784)
 - **[dhctl]** ModuleSource mirroring via `dhctl`. [#6603](https://github.com/deckhouse/deckhouse/pull/6603)
 - **[external-module-manager]** Add `ModuleUpdatePolicy` custom resource and implemet its logic. [#6668](https://github.com/deckhouse/deckhouse/pull/6668)
    New module releases will not be created and deployed without relevant module update policies. ModuleUpdatePolicy for the `deckhouse` ModuleSource is created automatically.
 - **[istio]** Bump Istio version to `1.19.4`. Bump Kiali version to `1.67`. [#6616](https://github.com/deckhouse/deckhouse/pull/6616)
    In environments where legacy versions of istio are used, the D8 update will be blocked, and the `D8IstioDeprecatedIstioVersionInstalled` alert will be fired.
 - **[node-local-dns]** Remove `hostPorts` from manifest when `hostNetwork=false`. [#6579](https://github.com/deckhouse/deckhouse/pull/6579)
 - **[node-manager]** Use dependency-free cross-version python script for second phase bootstrap loading. [#6762](https://github.com/deckhouse/deckhouse/pull/6762)
 - **[node-manager]** Add DeckhouseControlPlane for Cluster API. [#6664](https://github.com/deckhouse/deckhouse/pull/6664)
 - **[prometheus]** Grafana image are based on a distroless image. [#6680](https://github.com/deckhouse/deckhouse/pull/6680)
 - **[prometheus]** Images are based on a distroless image. [#6644](https://github.com/deckhouse/deckhouse/pull/6644)

## Fixes


 - **[candi]** Resolve names to IPv4 addresses with d8-curl. [#6898](https://github.com/deckhouse/deckhouse/pull/6898)
 - **[candi]** Fix building terraform providers. [#6772](https://github.com/deckhouse/deckhouse/pull/6772)
 - **[candi]** Restart kubelet after containerd restart to fix containerd.sock loose. [#6735](https://github.com/deckhouse/deckhouse/pull/6735)
 - **[cni-cilium]** Cilium version bumped to 1.14.5 [#6872](https://github.com/deckhouse/deckhouse/pull/6872)
    Cilium agents will restart, during restart some policies won't work.
 - **[deckhouse]** Webhook-handler can ignore validation error during a cluster bootstrap. [#6980](https://github.com/deckhouse/deckhouse/pull/6980)
 - **[deckhouse-controller]** Fix modules registration on startup. [#7129](https://github.com/deckhouse/deckhouse/pull/7129)
 - **[deckhouse-controller]** Disable the requirement that blocks Deckhouse updates when the built-in embedded virtualization module is enabled. [#7084](https://github.com/deckhouse/deckhouse/pull/7084)
 - **[deckhouse-controller]** Fix config values for dynamically enabled modules. [#7036](https://github.com/deckhouse/deckhouse/pull/7036)
 - **[deckhouse-controller]** Fix global values change and source modules validation. [#6814](https://github.com/deckhouse/deckhouse/pull/6814)
 - **[deckhouse-controller]** Create module directory with desired version only when ModuleRelease is deployed. [#6807](https://github.com/deckhouse/deckhouse/pull/6807)
 - **[deckhouse-controller]** Bump addon-operator. Change internal work with modules. Behavior of commands `deckhouse-controller module values <name>` was changed, no more top levels keys by default. To have old behavior use '-g' flag. [#6495](https://github.com/deckhouse/deckhouse/pull/6495)
 - **[dhctl]** dhctl mirror will tag release channels images with their versions as well. [#7008](https://github.com/deckhouse/deckhouse/pull/7008)
 - **[dhctl]** mirror will validate tar bundle path more strictly. [#6981](https://github.com/deckhouse/deckhouse/pull/6981)
 - **[dhctl]** Fix for pulling older Deckhouse versions with tags instead of digests in installers. [#6932](https://github.com/deckhouse/deckhouse/pull/6932)
 - **[dhctl]** Allow missing module release channels. [#6848](https://github.com/deckhouse/deckhouse/pull/6848)
 - **[dhctl]** Fix mirror not adding module-named tags at modules repo root. [#6782](https://github.com/deckhouse/deckhouse/pull/6782)
 - **[dhctl]** Validate passed credentials against registry prior to mirroring. [#6629](https://github.com/deckhouse/deckhouse/pull/6629)
 - **[extended-monitoring]** Fix wrong permissions for `k8s-image-availability-exporter`. [#6754](https://github.com/deckhouse/deckhouse/pull/6754)
 - **[external-module-manager]** Fix multiple symlinks for a single module in the symlink folder. [#7228](https://github.com/deckhouse/deckhouse/pull/7228)
 - **[external-module-manager]** Fix outdated module versions in multi-master environment. [#7222](https://github.com/deckhouse/deckhouse/pull/7222)
 - **[external-module-manager]** Fix ModuleRelease nightly cleanup. [#7108](https://github.com/deckhouse/deckhouse/pull/7108)
 - **[external-module-manager]** Get scheme for `deckhouse` ModuleSource from the deckhouse values. [#7105](https://github.com/deckhouse/deckhouse/pull/7105)
 - **[external-module-manager]** Remove default `ModuleUpdatePolicy` for the `deckhouse` ModuleSource. [#6822](https://github.com/deckhouse/deckhouse/pull/6822)
 - **[external-module-manager]** Absent module sources are restored on Deckhouse startup. [#6607](https://github.com/deckhouse/deckhouse/pull/6607)
 - **[flant-integration]** Correct `metrics_path`. [#7198](https://github.com/deckhouse/deckhouse/pull/7198)
 - **[flant-integration]** Run hook envs_from_nodes OnBeforeHelm to prevent main queue stuck. [#6750](https://github.com/deckhouse/deckhouse/pull/6750)
 - **[go_lib]** Restore `.global.enabledModules` values field for source modules. [#6935](https://github.com/deckhouse/deckhouse/pull/6935)
 - **[go_lib]** Limit registry client operations with 30 seconds timeout. [#6701](https://github.com/deckhouse/deckhouse/pull/6701)
 - **[ingress-nginx]** Fix `HostWithFailover` inlet proxy filter. [#7183](https://github.com/deckhouse/deckhouse/pull/7183)
 - **[istio]** Fix diagnostic instructions in istio federation/multicluster alerts. [#7197](https://github.com/deckhouse/deckhouse/pull/7197)
 - **[istio]** Adding alert about istio and K8s versions incompatibility. [#6919](https://github.com/deckhouse/deckhouse/pull/6919)
 - **[istio]** Remove istio/k8s compatibility check. [#6914](https://github.com/deckhouse/deckhouse/pull/6914)
 - **[linstor]** Fix the `D8DrbdPeerDeviceIsOutOfSync` alert. [#6778](https://github.com/deckhouse/deckhouse/pull/6778)
 - **[log-shipper]** Fix empty field trailing dots in filters. [#6794](https://github.com/deckhouse/deckhouse/pull/6794)
 - **[multitenancy-manager]** Prevents the creation of a new "helm" release when no changes are made to it. [#6811](https://github.com/deckhouse/deckhouse/pull/6811)
 - **[multitenancy-manager]** Resources are created without a prefix as a project name. [#6734](https://github.com/deckhouse/deckhouse/pull/6734)
    Resources in the project namespaces (the `multitenancy-manager`) will lose prefix of the project. It would affect `secretRef`, `configMapRef` or other resource references if you use it.
 - **[node-local-dns]** Fix for CiliumLocalRedirectPolicy for node-local-redirect (wrong protocol for dns-tcp rule). [#6844](https://github.com/deckhouse/deckhouse/pull/6844)
 - **[node-manager]** Fix NodeGroup deletion for CAPS. [#6590](https://github.com/deckhouse/deckhouse/pull/6590)
 - **[node-manager]** Generate a `static-kubeconfig` with X.509 certificates for the Cluster API controller. [#6552](https://github.com/deckhouse/deckhouse/pull/6552)
 - **[node-manager]** Do not parse annotations in the filter. [#6436](https://github.com/deckhouse/deckhouse/pull/6436)
 - **[operator-trivy]** Fixed trivy working with private container registry with self-signed CA. [#6747](https://github.com/deckhouse/deckhouse/pull/6747)
 - **[prometheus]** Add df utility to an image. [#6757](https://github.com/deckhouse/deckhouse/pull/6757)
 - **[runtime-audit-engine]** Fixed vulnerabilities: GHSA-62mh-w5cv-p88c, CVE-2021-3127, GHSA-j756-f273-xhp4, CVE-2022-21698, CVE-2020-29652, CVE-2021-43565, CVE-2022-27191, CVE-2021-33194, CVE-2022-27664, CVE-2022-41723, CVE-2023-39325, CVE-2021-38561, CVE-2022-32149, GHSA-m425-mq94-257g, CVE-2022-28948. [#6501](https://github.com/deckhouse/deckhouse/pull/6501)
 - **[user-authn]** OIDC allows unverified email. [#6964](https://github.com/deckhouse/deckhouse/pull/6964)

## Chore


 - **[candi]** Bump patch versions of Kubernetes images: `v1.26.12`, `v1.27.9`, `v1.28.5`. [#7022](https://github.com/deckhouse/deckhouse/pull/7022)
    Kubernetes control-plane components will restart, kubelet will restart.
 - **[candi]** Fix release requirements. [#7003](https://github.com/deckhouse/deckhouse/pull/7003)
 - **[cni-cilium]** Enabled pprof interface in cilium-agent. [#6874](https://github.com/deckhouse/deckhouse/pull/6874)
    cillium-agent pods will restart.
 - **[cni-simple-bridge]** Use distroless based image. [#6469](https://github.com/deckhouse/deckhouse/pull/6469)
    `simple-bridge` pods will restart; affected clouds are AWS, Azure, GCP, Yandex.Cloud.
 - **[deckhouse-controller]** Use uniform method to install CRDs for hooks and `deckhouse-controller`. [#6960](https://github.com/deckhouse/deckhouse/pull/6960)
 - **[deckhouse-controller]** Bump Go and Kubernetes client versions. [#6698](https://github.com/deckhouse/deckhouse/pull/6698)
 - **[deckhouse-controller]** Remove virtualization embedded module [#5554](https://github.com/deckhouse/deckhouse/pull/5554)
    Uninstalling the virtualization module will destroy all the virtual machines created with the module. Take care to save virtual machine data if necessary.
 - **[docs]** Add documentation render at the https://deckhouse.io website for source modules in the deckhouse registry. [#6317](https://github.com/deckhouse/deckhouse/pull/6317)
 - **[external-module-manager]** Refactor the module. Make `ModuleSource` and `ModuleRelease` resources more manageable. [#6607](https://github.com/deckhouse/deckhouse/pull/6607)
 - **[ingress-nginx]** Add alerts for the deprecated Ingress controller version. [#6728](https://github.com/deckhouse/deckhouse/pull/6728)
 - **[ingress-nginx]** Add validation for httpsPort field of the IngressNginxController CRD. [#6631](https://github.com/deckhouse/deckhouse/pull/6631)
    Ingress controller pods will restart.
 - **[linstor]** Simultaneously enabling the Linstor and SDS-DRBD modules is prohibited. [#6776](https://github.com/deckhouse/deckhouse/pull/6776)
 - **[monitoring-kubernetes]** Correct the `LoadBalancerServiceWithoutExternalIP` alert. [#7150](https://github.com/deckhouse/deckhouse/pull/7150)
 - **[prometheus]** Add examples of CustomAlertmanager for Slack and Opsgenie. [#7030](https://github.com/deckhouse/deckhouse/pull/7030)
 - **[terraform-manager]** Build distroless-based terraform-manager images. [#6639](https://github.com/deckhouse/deckhouse/pull/6639)

