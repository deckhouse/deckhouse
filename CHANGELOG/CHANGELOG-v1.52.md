# Changelog v1.52

## Know before update


 - All nodes with DRBD will restart. Linstor controller and piraeus operator will restart.
 - All pods using `kube-rbac-proxy` will restart.

## Features


 - **[admission-policy-engine]** Add `external-data` and `trivy-provider` for the gatekeeper to prevent the container from starting if high or critical CVEs are found in the image. [#5376](https://github.com/deckhouse/deckhouse/pull/5376)
 - **[chrony]** Master nodes act as NTP servers for cluster. [#5802](https://github.com/deckhouse/deckhouse/pull/5802)
 - **[control-plane-manager]** All images are now based on distroless image. [#5509](https://github.com/deckhouse/deckhouse/pull/5509)
 - **[ingress-nginx]** Images are based on a distroless image. [#5937](https://github.com/deckhouse/deckhouse/pull/5937)
 - **[linstor]** A setting has been added to specify the number of parallel CSI workers (attacher, provisioner, snapshotter, resizer). [#6129](https://github.com/deckhouse/deckhouse/pull/6129)
 - **[linstor]** Update linstor controller version to `1.24.2`. Update DRBD version to `9.2.5`. [#5800](https://github.com/deckhouse/deckhouse/pull/5800)
    All nodes with DRBD will restart. Linstor controller and piraeus operator will restart.
 - **[log-shipper]** Suppress metrics timestamp to avoid out-of-order ingestion error. [#5835](https://github.com/deckhouse/deckhouse/pull/5835)
 - **[monitoring-applications]** Add Grafana dashboard for pgbouncer. [#5846](https://github.com/deckhouse/deckhouse/pull/5846)
 - **[monitoring-applications]** Update Grafana dashboard for Elasticsearch. Add minimal alert rules for Prometheus. [#5845](https://github.com/deckhouse/deckhouse/pull/5845)
 - **[monitoring-applications]** Added Grafana dashboard for displaying PHP-FPM queue status and slow requests. [#5843](https://github.com/deckhouse/deckhouse/pull/5843)
 - **[monitoring-kubernetes]** Images are based on a distroless image. Bumped `node-exporter` version to `1.6.1`. [#5515](https://github.com/deckhouse/deckhouse/pull/5515)
 - **[node-manager]** All images are now based on distroless image. [#5520](https://github.com/deckhouse/deckhouse/pull/5520)
 - **[operator-prometheus]** Add `EndpointSlice` support for service discovery. [#5856](https://github.com/deckhouse/deckhouse/pull/5856)
 - **[operator-trivy]** Add a flag to use BDU vulnerabilities instead of CVE in the vulnerability reports. [#5678](https://github.com/deckhouse/deckhouse/pull/5678)
 - **[operator-trivy]** All images are now based on distroless image. [#5521](https://github.com/deckhouse/deckhouse/pull/5521)
 - **[operator-trivy]** Run `operator-trivy` in client-server mode. Update `trivy` and `trivy-operator` versions. [#5376](https://github.com/deckhouse/deckhouse/pull/5376)
 - **[prometheus]** Add alert about `ServiceMonitors` with deprecated relabelings. [#5856](https://github.com/deckhouse/deckhouse/pull/5856)
 - **[runtime-audit-engine]** Improve documentation and add advanced usage documentation. [#5168](https://github.com/deckhouse/deckhouse/pull/5168)
 - **[user-authz]** All images are now based on distroless image. [#5511](https://github.com/deckhouse/deckhouse/pull/5511)

## Fixes


 - **[candi]** Fix deckhouse containerd start after installing new containerd-deckhouse package. [#6329](https://github.com/deckhouse/deckhouse/pull/6329)
 - **[candi]** Allow underscore in `httpProxy` and `httpsProxy` settings. [#6216](https://github.com/deckhouse/deckhouse/pull/6216)
 - **[candi]** Fix RedOS installation. [#6121](https://github.com/deckhouse/deckhouse/pull/6121)
 - **[candi]** Add creation of the `TMPDIR` directory in the `bashible.sh` script. [#6059](https://github.com/deckhouse/deckhouse/pull/6059)
 - **[candi]** Delete registrypackage files if it failed to install before retrying installation. [#5739](https://github.com/deckhouse/deckhouse/pull/5739)
 - **[chrony]** Run chrony pods in host network namespace. [#6007](https://github.com/deckhouse/deckhouse/pull/6007)
 - **[cloud-provider-yandex]** Fix working in hybrid environments. [#6094](https://github.com/deckhouse/deckhouse/pull/6094)
 - **[cloud-provider-yandex]** Updated yandex-csi-driver. In the new version, the calculation of the limit of disks per node has been fixed. [#5679](https://github.com/deckhouse/deckhouse/pull/5679)
 - **[dhctl]** Fix restarting bootstrap process. [#5892](https://github.com/deckhouse/deckhouse/pull/5892)
 - **[dhctl]** Add `registryDockerCfg` validation. [#5829](https://github.com/deckhouse/deckhouse/pull/5829)
 - **[extended-monitoring]** Fix extended monitoring rules for node disk usage. [#6227](https://github.com/deckhouse/deckhouse/pull/6227)
 - **[external-module-manager]** Handle deployed source modules with absent version directory. Avoid infinite deckhouse restart on absent module. [#5994](https://github.com/deckhouse/deckhouse/pull/5994)
 - **[flant-integration]** Fix `connect` response handling with respect of status code instead of error message. [#5765](https://github.com/deckhouse/deckhouse/pull/5765)
 - **[ingress-nginx]** Add postpone updates for main controller of `HostWithFailover` inlet. [#5988](https://github.com/deckhouse/deckhouse/pull/5988)
 - **[ingress-nginx]** Fix controller preStop probe. [#5948](https://github.com/deckhouse/deckhouse/pull/5948)
    Ingress controller will restart.
 - **[linstor]** Changed the method of loading DRBD kernel module on the nodes with active LINSTOR satellites. [#6288](https://github.com/deckhouse/deckhouse/pull/6288)
 - **[linstor]** Changes in controller liveness probe. [#6203](https://github.com/deckhouse/deckhouse/pull/6203)
 - **[linstor]** Improved symlink handling for devices. [#6187](https://github.com/deckhouse/deckhouse/pull/6187)
 - **[linstor]** Removed alert about HA-controller absence. [#6166](https://github.com/deckhouse/deckhouse/pull/6166)
 - **[linstor]** Added an init container for LINSTOR satellite, that waits for DRBD v9.x to be loaded on the node. [#6151](https://github.com/deckhouse/deckhouse/pull/6151)
 - **[linstor]** Disabled usermode_helper param on LINSTOR nodes for more stable management. [#6137](https://github.com/deckhouse/deckhouse/pull/6137)
 - **[linstor]** Removed HA controller because of harmful behavior. [#6132](https://github.com/deckhouse/deckhouse/pull/6132)
 - **[linstor]** Automatically fix symlinks for devices. [#6026](https://github.com/deckhouse/deckhouse/pull/6026)
 - **[linstor]** Fixed error in LINSTOR controller liveness probe logic. [#6083](https://github.com/deckhouse/deckhouse/pull/6083)
    Linstor controller will be restarted.
 - **[linstor]** Workaround for several annoying issues in LINSTOR related to hanging controller. [#6037](https://github.com/deckhouse/deckhouse/pull/6037)
 - **[log-shipper]** Fix validation for the buffer `ClusterLogDestination` schema. [#5925](https://github.com/deckhouse/deckhouse/pull/5925)
 - **[log-shipper]** Add stricter validation for label selectors. Prevents the Deckhouse pods from panicking. [#5925](https://github.com/deckhouse/deckhouse/pull/5925)
 - **[log-shipper]** Fix custom multiline parser validation for `PodLoggingConfig` (previously, it was impossible to use the Custom type due to a validation bug). [#5925](https://github.com/deckhouse/deckhouse/pull/5925)
 - **[log-shipper]** Remove `libssl1.1_1.1.1f-1ubuntu2.17_amd64.deb` from the final image after installation. [#5794](https://github.com/deckhouse/deckhouse/pull/5794)
 - **[multitenancy-manager]** When trying to delete a projectType used in a project, an error with project name displayed. [#5744](https://github.com/deckhouse/deckhouse/pull/5744)
 - **[prometheus]** fix fix-permissions init-container to run under kesl security. [#6082](https://github.com/deckhouse/deckhouse/pull/6082)
 - **[prometheus]** Clarify description and formula for the oldest metrics panel on the starting page of Grafana. [#5712](https://github.com/deckhouse/deckhouse/pull/5712)
 - **[registrypackages]** Fix disabling the old `containerd` service in Ubuntu `18.04`. [#6172](https://github.com/deckhouse/deckhouse/pull/6172)
    `containerd` on Ubuntu 18.04 will restart.
 - **[user-authn]** Fix generation a self signed certificate for `crowd-basic-auth-proxy`. [#6074](https://github.com/deckhouse/deckhouse/pull/6074)
 - **[user-authz]** Add matchAny parameter to namespaceSelector for granting access to all namespaces. [#6014](https://github.com/deckhouse/deckhouse/pull/6014)

## Chore


 - **[candi]** Add generation of cleanup node script for static nodes. [#5945](https://github.com/deckhouse/deckhouse/pull/5945)
 - **[candi]** Bump patch versions of Kubernetes images: `v1.25.14`, `v1.26.9`, `v1.27.6`. [#5873](https://github.com/deckhouse/deckhouse/pull/5873)
    Kubernetes control plane components will restart, kubelet will restart.
 - **[candi]** `csi-external-attacher` use distroless based image. [#5820](https://github.com/deckhouse/deckhouse/pull/5820)
 - **[candi]** `csi-external-provisioner` use distroless based image. [#5820](https://github.com/deckhouse/deckhouse/pull/5820)
 - **[candi]** `csi-external-resizer` use distroless based image. [#5820](https://github.com/deckhouse/deckhouse/pull/5820)
 - **[candi]** `csi-external-snapshotter` use distroless based image. [#5820](https://github.com/deckhouse/deckhouse/pull/5820)
 - **[candi]** `csi-livenessprobe` use distroless based image. [#5820](https://github.com/deckhouse/deckhouse/pull/5820)
 - **[candi]** `csi-node-driver-registrar` use distroless based image. [#5820](https://github.com/deckhouse/deckhouse/pull/5820)
 - **[candi]** move registry packages to build from local repos. [#5798](https://github.com/deckhouse/deckhouse/pull/5798)
 - **[candi]** Point $TMPDIR to `/opt/deckhouse/tmp` during bashible bootstrap. [#5739](https://github.com/deckhouse/deckhouse/pull/5739)
 - **[candi]** Use Deckhouse repos to build Kubernetes. [#5738](https://github.com/deckhouse/deckhouse/pull/5738)
    Kubernetes control plane components will restart.
 - **[candi]** Containerd systemd unit that ships with Deckhouse renamed to `containerd-deckhouse.service`. [#5677](https://github.com/deckhouse/deckhouse/pull/5677)
 - **[cert-manager]** Bump cert-manager to the `1.12.3` version. CAInjector was disabled by default; you can enable it via the `enableCAInjector` parameter. `kube-rbac-proxy` port was moved to the `9404` from `9403`. Port `9403` was added as `/livez` probe for the `cert-manager` container. [#5685](https://github.com/deckhouse/deckhouse/pull/5685)
 - **[common]** Common images are now based on distroless image. [#5750](https://github.com/deckhouse/deckhouse/pull/5750)
    All pods using `kube-rbac-proxy` will restart.
 - **[deckhouse-controller]** `bashible-apiserver` no longer scaled-down during registry changes. [#5722](https://github.com/deckhouse/deckhouse/pull/5722)
 - **[dhctl]** Render bootstrap bundle into `/opt/deckhouse/tmp`. [#5739](https://github.com/deckhouse/deckhouse/pull/5739)
 - **[node-manager]** `bashible-apiserver`'s readiness probe now fails if `deckhouse` deployment has no pods in `Ready` state. [#5722](https://github.com/deckhouse/deckhouse/pull/5722)
 - **[operator-prometheus]** Bump `prometheus-operator` from `0.62` to `0.68` version. [#5856](https://github.com/deckhouse/deckhouse/pull/5856)
 - **[registrypackages]** Bump `containerd` version to `1.6.24`. [#5938](https://github.com/deckhouse/deckhouse/pull/5938)
 - **[runtime-audit-engine]** Added fstek falco rules. [#5787](https://github.com/deckhouse/deckhouse/pull/5787)
 - **[runtime-audit-engine]** Removed previously implemented Falco rules, which monitor syscalls, due to improper work. [#5787](https://github.com/deckhouse/deckhouse/pull/5787)

