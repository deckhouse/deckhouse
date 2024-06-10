# Changelog v1.60

## [MALFORMED]


 - #8429 unknown section "l2-load-balancer"

## Know before update


 - Deckhouse will not be updated if Ingress controller 1.1 is used.
 - Ingress controllers without explicitly set version will update.
 - Okagent will update from the Deckhouse Observability platform source and will start sending metrics to DOP for FE installations.
 - The method for configuring network interfaces and routes has changed in OpenStack instances.
 - You can use multiple `--config` flag in bootstrap command for logical separation bootstrap configuration. The `--resources` flag is now deprecated.

## Features


 - **[admission-policy-engine]** Add a separate queue and throttling for bootstrap hook. [#8028](https://github.com/deckhouse/deckhouse/pull/8028)
 - **[candi]** Forbid to use Debian 9. [#7710](https://github.com/deckhouse/deckhouse/pull/7710)
 - **[cloud-provider-vcd]** Support multiple VCD API versions. [#8451](https://github.com/deckhouse/deckhouse/pull/8451)
 - **[cloud-provider-yandex]** Add option to create nodes with SSD IO root disks. [#8382](https://github.com/deckhouse/deckhouse/pull/8382)
 - **[cloud-provider-yandex]** Add custom target group node annotation. [#8171](https://github.com/deckhouse/deckhouse/pull/8171)
    Yandex CCM should be restarted.
 - **[deckhouse]** Replace go-based conversions with jq-based. [#8193](https://github.com/deckhouse/deckhouse/pull/8193)
 - **[deckhouse]** Add validation webhook for `publicDomainTemplate`. [#8107](https://github.com/deckhouse/deckhouse/pull/8107)
 - **[deckhouse-controller]** Move validation logic to the module object. [#8218](https://github.com/deckhouse/deckhouse/pull/8218)
 - **[deckhouse-controller]** replace insecure flag with scheme flag [#8076](https://github.com/deckhouse/deckhouse/pull/8076)
 - **[deckhouse-controller]** Reapply modules' registry settings after update their module source. [#8067](https://github.com/deckhouse/deckhouse/pull/8067)
 - **[deckhouse-controller]** Add a module release mechanism similar to the Deckhouse release mechanism. [#7348](https://github.com/deckhouse/deckhouse/pull/7348)
 - **[dhctl]** Improve config validation error messages. [#8453](https://github.com/deckhouse/deckhouse/pull/8453)
 - **[dhctl]** Add bootstrap, converge, destroy, abort, import gRPC services. [#8455](https://github.com/deckhouse/deckhouse/pull/8455)
 - **[dhctl]** Add debug log file for all commands. [#8303](https://github.com/deckhouse/deckhouse/pull/8303)
 - **[dhctl]** One flag for resources and configs in bootstrap command. [#8277](https://github.com/deckhouse/deckhouse/pull/8277)
    You can use multiple `--config` flag in bootstrap command for logical separation bootstrap configuration. The `--resources` flag is now deprecated.
 - **[dhctl]** Control-plane readiness check after bootstrap. [#8230](https://github.com/deckhouse/deckhouse/pull/8230)
 - **[dhctl]** Create `DeckhouseRelease` for a new cluster. [#8143](https://github.com/deckhouse/deckhouse/pull/8143)
 - **[ingress-nginx]** Add open-telemetry support to 1.9 ingress nginx. [#8232](https://github.com/deckhouse/deckhouse/pull/8232)
    If an ingress nginx of 1.9 version is used, its pods will be recreated.
 - **[kube-dns]** Add validation webhook for `clusterDomainAliases`. [#8107](https://github.com/deckhouse/deckhouse/pull/8107)
 - **[log-shipper]** Add extraLabels validation. [#8031](https://github.com/deckhouse/deckhouse/pull/8031)
 - **[monitoring-kubernetes]** Add 'Storage Filesystem' -> 'Filesystem size' in grafana-dashboard node. [#7816](https://github.com/deckhouse/deckhouse/pull/7816)
 - **[multitenancy-manager]** Set `heritage: deckhouse` label to embedded ProjectTemplates. Deny ProjectTemplates copying with this label. [#8141](https://github.com/deckhouse/deckhouse/pull/8141)
 - **[node-manager]** Drain advanced DaemonSet pods when a node is deleted or updated. [#8194](https://github.com/deckhouse/deckhouse/pull/8194)
 - **[okmeter]** Add `okagent` module for FE with customized environment variables to communicate with Deckhouse Observability platform. [#8313](https://github.com/deckhouse/deckhouse/pull/8313)
    Okagent will update from the Deckhouse Observability platform source and will start sending metrics to DOP for FE installations.
 - **[operator-trivy]** Аdd ability to explicitly set storageClass. [#8373](https://github.com/deckhouse/deckhouse/pull/8373)
 - **[operator-trivy]** Add flag to create vulnerability reports only with desired severity levels. [#8096](https://github.com/deckhouse/deckhouse/pull/8096)
 - **[registry-packages-proxy]** Add in-cluster proxy for registry packages. [#7751](https://github.com/deckhouse/deckhouse/pull/7751)

## Fixes


 - **[candi]** Fix incorrect condition for hostname discovery. [#8458](https://github.com/deckhouse/deckhouse/pull/8458)
 - **[candi]** Set the default `standard.bastion.instanceClass.rootDiskSize` property to 50 gigabytes in the `OpenStackClusterConfiguration` schema. [#8309](https://github.com/deckhouse/deckhouse/pull/8309)
 - **[cloud-provider-vcd]** Remove required from discoveryData property. [#8541](https://github.com/deckhouse/deckhouse/pull/8541)
 - **[cloud-provider-vsphere]** Update `govmomi` lib to fix discoverer for vSphere `8.0.2`. [#8204](https://github.com/deckhouse/deckhouse/pull/8204)
 - **[deckhouse]** Fix the number of deckhouse replicas for not bootstrapped clusters. [#8526](https://github.com/deckhouse/deckhouse/pull/8526)
 - **[deckhouse]** Validate that registry address is always present in the DKP configuration. [#8242](https://github.com/deckhouse/deckhouse/pull/8242)
 - **[deckhouse]** Fix annotantions on ns `d8-cloud-intance-manager` to move to the another module. [#8196](https://github.com/deckhouse/deckhouse/pull/8196)
 - **[deckhouse-controller]** Fix default module source template. [#8567](https://github.com/deckhouse/deckhouse/pull/8567)
 - **[deckhouse-controller]** Disable manager's internal metrics server. [#8558](https://github.com/deckhouse/deckhouse/pull/8558)
 - **[deckhouse-controller]** Start addon-operator after controllers' preflight checks. [#8485](https://github.com/deckhouse/deckhouse/pull/8485)
 - **[deckhouse-controller]** Fix MPO _out of sync_ in HA mode. [#8370](https://github.com/deckhouse/deckhouse/pull/8370)
 - **[deckhouse-controller]** Fix updates to module loading statistics and an error related to concurrent requests for module documentation building. [#8235](https://github.com/deckhouse/deckhouse/pull/8235)
 - **[deckhouse-controller]** Repeated queries with a limited number of attempts to get CRDs and apply conversion strategies. [#7949](https://github.com/deckhouse/deckhouse/pull/7949)
 - **[dhctl]** Fix registry path calculation. [#8646](https://github.com/deckhouse/deckhouse/pull/8646)
    Registry packages proxy should be restarted.
 - **[dhctl]** Print invalid YAML documents. [#8296](https://github.com/deckhouse/deckhouse/pull/8296)
 - **[dhctl]** Fix preflight ports checking when static cluster is bootstrapping. [#7917](https://github.com/deckhouse/deckhouse/pull/7917)
 - **[docs]** Add a guide for full etcd recovery in the cluster. [#8405](https://github.com/deckhouse/deckhouse/pull/8405)
 - **[docs]** Update kube-bench documentation to use `kubectl` and `jq` instead of `yq`. [#8439](https://github.com/deckhouse/deckhouse/pull/8439)
 - **[documentation]** Fix search. Fix version badge. [#8520](https://github.com/deckhouse/deckhouse/pull/8520)
 - **[extended-monitoring]** Add support of mirrors for container registries for `image_availability_exporter`. [#8568](https://github.com/deckhouse/deckhouse/pull/8568)
 - **[ingress-nginx]** Fix pod descheduling from a not-ready node. [#8647](https://github.com/deckhouse/deckhouse/pull/8647)
    Kruise controller's pods will be recreadted.
 - **[ingress-nginx]** Fix usage custom logs formats without our fields. [#8621](https://github.com/deckhouse/deckhouse/pull/8621)
 - **[ingress-nginx]** Digital ocean Kubernetes upgrade, update `timeoutSeconds`. [#8413](https://github.com/deckhouse/deckhouse/pull/8413)
 - **[ingress-nginx]** Replace status with formatted status in logs. [#8385](https://github.com/deckhouse/deckhouse/pull/8385)
 - **[kube-dns]** Fix empty array error in domain alias validation webhook. [#8503](https://github.com/deckhouse/deckhouse/pull/8503)
 - **[kube-proxy]** Start `kube-proxy` only if `cni-flannel` or `cni-simple-bridge` modules is enabled. [#8258](https://github.com/deckhouse/deckhouse/pull/8258)
 - **[metallb]** Switched to state-timeline plugin in MetalLB  Grafana dashboard. [#8389](https://github.com/deckhouse/deckhouse/pull/8389)
 - **[monitoring-applications]** Fix rabbitmq dashboard. [#7979](https://github.com/deckhouse/deckhouse/pull/7979)
 - **[monitoring-ping]** Skip nodes without IP address. [#8440](https://github.com/deckhouse/deckhouse/pull/8440)
 - **[node-manager]** Fix RBAC permissions and startup schedule cleanup of NodeUser creation errors. [#8639](https://github.com/deckhouse/deckhouse/pull/8639)
 - **[node-manager]** Errors when adding a NodeUser do not block bashible. [#8054](https://github.com/deckhouse/deckhouse/pull/8054)
 - **[okmeter]** Respect `nodeSelector` and `tolerations` configuration options from the `ModuleConfig`. [#8377](https://github.com/deckhouse/deckhouse/pull/8377)
 - **[okmeter]** Restart okagent pods on API token change. [#8343](https://github.com/deckhouse/deckhouse/pull/8343)
 - **[operator-trivy]** Fix incorrect reports links. [#7997](https://github.com/deckhouse/deckhouse/pull/7997)
 - **[prometheus]** Change `ignore_error` value from `true` to `false` in the aggregation proxy config to prevent caching empty results. [#8415](https://github.com/deckhouse/deckhouse/pull/8415)
 - **[prometheus]** Drop the metrics named `memcached_slab_*` from memcached. [#8342](https://github.com/deckhouse/deckhouse/pull/8342)
 - **[registry-packages-proxy]** Fix registry path calculation. [#8646](https://github.com/deckhouse/deckhouse/pull/8646)
    Registry packages proxy should be restarted.
 - **[registry-packages-proxy]** Don't replace the system CA certificates if a custom CA is used. [#8537](https://github.com/deckhouse/deckhouse/pull/8537)
 - **[registry-packages-proxy]** Fix panic when using custom CA. [#8276](https://github.com/deckhouse/deckhouse/pull/8276)
 - **[upmeter]** Fix agent error "cannot add series for probe <probename>: limit reached". [#8304](https://github.com/deckhouse/deckhouse/pull/8304)
 - **[user-authn]** Fix boundary value when `idTokenTTL` is too large. [#7903](https://github.com/deckhouse/deckhouse/pull/7903)

## Chore


 - **[candi]** Bump patch versions of Kubernetes images: `1.27.14`, `1.28.10`, `1.29.5`. [#8435](https://github.com/deckhouse/deckhouse/pull/8435)
    Kubernetes control-plane components will restart, kubelet will restart.
 - **[candi]** Bump patch versions of Kubernetes images: `1.27.13`, `1.28.9`, `1.29.4`. [#8239](https://github.com/deckhouse/deckhouse/pull/8239)
    Kubernetes control-plane components will restart, kubelet will restart.
 - **[candi]** Update dev-images. [#7934](https://github.com/deckhouse/deckhouse/pull/7934)
 - **[candi]** Refactoring the network configuration with multiple routes in OpenStack. [#5665](https://github.com/deckhouse/deckhouse/pull/5665)
    The method for configuring network interfaces and routes has changed in OpenStack instances.
 - **[deckhouse]** Fetch `clusterDomain` value from `global.discovery`, rather than from `global.clusterConfiguration`. [#8402](https://github.com/deckhouse/deckhouse/pull/8402)
 - **[deckhouse-controller]** Use Go 1.22. [#8333](https://github.com/deckhouse/deckhouse/pull/8333)
 - **[deckhouse-controller]** Add hooks for `okagent` module in DKP FE. [#8313](https://github.com/deckhouse/deckhouse/pull/8313)
 - **[deckhouse-controller]** Provide documentation for modules deployed by `ModulePullOverride`. [#7985](https://github.com/deckhouse/deckhouse/pull/7985)
 - **[docs]** Update documentation to use `d8 mirror` instead of `dhctl mirror`. [#8378](https://github.com/deckhouse/deckhouse/pull/8378)
 - **[docs]** Fix `etcdctl` command from pods to distroless syntax. [#8344](https://github.com/deckhouse/deckhouse/pull/8344)
 - **[docs]** Add getting started for VMware Cloud Director. [#7818](https://github.com/deckhouse/deckhouse/pull/7818)
 - **[ingress-nginx]** Set default Ingress controller version to 1.9. [#8223](https://github.com/deckhouse/deckhouse/pull/8223)
    Ingress controllers without explicitly set version will update.
 - **[ingress-nginx]** Remove support of the Ingress controller version 1.1 . [#8223](https://github.com/deckhouse/deckhouse/pull/8223)
    Deckhouse will not be updated if Ingress controller 1.1 is used.
 - **[ingress-nginx]** Fix base image for the Ingress nginx controller v1.6. [#8183](https://github.com/deckhouse/deckhouse/pull/8183)
 - **[ingress-nginx]** Fix the way the number of ingress nginx replicas is counted. [#8155](https://github.com/deckhouse/deckhouse/pull/8155)
    Kruise-controller pod will be recreated.
 - **[prometheus]** Fix URLs to the documentation in the Grafana home panel. [#8072](https://github.com/deckhouse/deckhouse/pull/8072)
 - **[registrypackages]** Update the `d8` registry package version. [#8563](https://github.com/deckhouse/deckhouse/pull/8563)
 - **[registrypackages]** Bump d8 CLI version to v0.1.0. [#8378](https://github.com/deckhouse/deckhouse/pull/8378)

