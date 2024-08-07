# Changelog v1.63

## Know before update


 - All cilium-related pods will restart. L7 and FQDN-based policies will flap.
 - For all new Istio sidecar containers the CPU limit will be set to 2 (you can tune this). To apply the limit to existing pods, you will need to restart them manually.
 - If cluster has ModuleSource resources with custom CA, containerd will be restarted on all nodes in cluster.
 - In the runtime-audit-engine, the webhook-handler port moved from `9680` to `4227`. This may require updating firewall policies before upgrading the cluster.

## Features


 - **[candi]** Add support for ALT Linux p11. [#9040](https://github.com/deckhouse/deckhouse/pull/9040)
 - **[candi]** Add ModuleSource custom CA on nodes. [#9000](https://github.com/deckhouse/deckhouse/pull/9000)
    If cluster has ModuleSource resources with custom CA, containerd will be restarted on all nodes in cluster.
 - **[candi]** Added E2E tests for vCloud Director in Deckhouse. [#8852](https://github.com/deckhouse/deckhouse/pull/8852)
    Improves the reliability and stability of Deckhouse deployments in vCloud Director environments by catching integration issues early.
 - **[candi]** Single bootstrap for all bundles. [#8640](https://github.com/deckhouse/deckhouse/pull/8640)
    Possible impact on ordering new nodes.
 - **[cloud-provider-vcd]** Add the ability to specify an organization as the first part of the template path for master and cloud permanence node groups. [#9220](https://github.com/deckhouse/deckhouse/pull/9220)
 - **[cloud-provider-vcd]** Add support for token-based authentication. [#8862](https://github.com/deckhouse/deckhouse/pull/8862)
 - **[deckhouse]** Add DELETE verbs for restrict operations on `heritage=deckhouse` label. [#9180](https://github.com/deckhouse/deckhouse/pull/9180)
 - **[deckhouse-controller]** Consider `values.yaml` file as values default, not config-values. [#8837](https://github.com/deckhouse/deckhouse/pull/8837)
 - **[deckhouse-controller]** Deckhouse update logic has been moved to a separate controller. [#8667](https://github.com/deckhouse/deckhouse/pull/8667)
 - **[dhctl]** Add `--config` flag and add deprecation warning abour `--resources` flag for `bootstrap-phase create-resources` command. [#9105](https://github.com/deckhouse/deckhouse/pull/9105)
 - **[dhctl]** Add a preflight checks for system requirements on the master nodes. [#8961](https://github.com/deckhouse/deckhouse/pull/8961)
 - **[dhctl]** Added a preflight check for sudo permissions. [#8908](https://github.com/deckhouse/deckhouse/pull/8908)
 - **[dhctl]** Preflight check exist embedded containerd. [#8734](https://github.com/deckhouse/deckhouse/pull/8734)
 - **[dhctl]** Add retries for bashible routines. [#8627](https://github.com/deckhouse/deckhouse/pull/8627)
 - **[dhctl]** Preflight check hostname uniqueness on servers when setting up a bare metal cluster and check only one `--ssh-host` parameter used. [#8515](https://github.com/deckhouse/deckhouse/pull/8515)
 - **[docs]** Add a list of DKP alerts to the documentation. [#8861](https://github.com/deckhouse/deckhouse/pull/8861)
 - **[ingress-nginx]** Bump ingress-nginx to 1.10.3. [#9115](https://github.com/deckhouse/deckhouse/pull/9115)
    ingress-nginx controllers' pods with controller version 1.10 will be recreated.
 - **[ingress-nginx]** Add SSLPassthrough inlets. [#9021](https://github.com/deckhouse/deckhouse/pull/9021)
 - **[l2-load-balancer]** The new module for redundant L2 load-balancing. [#8465](https://github.com/deckhouse/deckhouse/pull/8465)
 - **[node-manager]** Cluster API Provider Static can adopt manually bootstrapped static nodes. [#8753](https://github.com/deckhouse/deckhouse/pull/8753)
 - **[operator-trivy]** Add `reportResourceLabels` option. [#9022](https://github.com/deckhouse/deckhouse/pull/9022)
 - **[prometheus]** Made Grafana version 10 the default on primary domain, moved Grafana 8 to secondary domain. [#9076](https://github.com/deckhouse/deckhouse/pull/9076)
 - **[registrypackages]** Use growpart from registrypackages. [#8850](https://github.com/deckhouse/deckhouse/pull/8850)

## Fixes


 - **[candi]** Add new dirs to cleanup on `cleanup_static_node.sh`. [#9159](https://github.com/deckhouse/deckhouse/pull/9159)
 - **[candi]** Fix work `registry-packages-proxy` with module sources. [#9112](https://github.com/deckhouse/deckhouse/pull/9112)
    `registry-packages-proxy` should be restarted.
 - **[candi]** Enable guest customization in Terraform configuration for master and cloud static nodes. [#9098](https://github.com/deckhouse/deckhouse/pull/9098)
 - **[candi]** AWS NAT Gateways are forced to be created in first non-local zone. [#9063](https://github.com/deckhouse/deckhouse/pull/9063)
 - **[candi]** Fix node-manager render network-script. [#9040](https://github.com/deckhouse/deckhouse/pull/9040)
 - **[candi]** Set bootstrap-network as first  bootstrap script. [#9040](https://github.com/deckhouse/deckhouse/pull/9040)
 - **[candi]** Fix index overflow when retrieving values from the list of external IP addresses. [#8877](https://github.com/deckhouse/deckhouse/pull/8877)
 - **[candi]** Fixed kubelet server certificate rotation. [#8603](https://github.com/deckhouse/deckhouse/pull/8603)
 - **[cni-cilium]** Improved the `CiliumAgentUnreachableHealthEndpoints` metric expression to avoid false positives. [#9198](https://github.com/deckhouse/deckhouse/pull/9198)
 - **[deckhouse-controller]** Fix panic on invalid module image [#9231](https://github.com/deckhouse/deckhouse/pull/9231)
 - **[deckhouse-controller]** Deckhouse update metrics restored. [#9147](https://github.com/deckhouse/deckhouse/pull/9147)
 - **[dhctl]** Wait for resources required by manifest being created. [#9157](https://github.com/deckhouse/deckhouse/pull/9157)
 - **[dhctl]** Fix creation cloudPermanent nodes with valid length name (no longer 42 symbols). [#9075](https://github.com/deckhouse/deckhouse/pull/9075)
 - **[dhctl]** Automatically use `--ssh-user` as `--ssh-bastion-user` if not set by the user. [#9061](https://github.com/deckhouse/deckhouse/pull/9061)
 - **[dhctl]** Fix watching CustomResource while custom `apiVersion` has not yet been applied. [#9040](https://github.com/deckhouse/deckhouse/pull/9040)
 - **[dhctl]** Validate the length of the list of external IP addresses in the `YandexClusterConfiguration`. [#8877](https://github.com/deckhouse/deckhouse/pull/8877)
 - **[docs]** Fix zone and disk in `volumeTypeMap` VK Cloud `config.yml` from Getting Started. [#9005](https://github.com/deckhouse/deckhouse/pull/9005)
    Fix bootstrap error in the Getting Started `config.yml` for VK Cloud.
 - **[go_lib]** Fix work `registry-packages-proxy` with module sources. [#9112](https://github.com/deckhouse/deckhouse/pull/9112)
    `registry-packages-proxy` should be restarted.
 - **[istio]** Istio 1.19 version compatibility extended with K8s versions 1.29 and 1.30. [#9217](https://github.com/deckhouse/deckhouse/pull/9217)
 - **[l2-load-balancer]** An internal refactoring and the fix for handling external and internal traffic policies in the LoadBalancer Service. [#9114](https://github.com/deckhouse/deckhouse/pull/9114)
 - **[log-shipper]** Make `extraLabels` and `CEF` encoding work for `Socket` destination. [#9149](https://github.com/deckhouse/deckhouse/pull/9149)
 - **[multitenancy-manager]** Fix templates bugs. [#9205](https://github.com/deckhouse/deckhouse/pull/9205)
 - **[node-manager]** Reducing unnecessary kube-apiserver logsl. [#9134](https://github.com/deckhouse/deckhouse/pull/9134)
    Reducing unnecessary kube-apiserver logs.
 - **[node-manager]** Added handling for graceful shutdown of node-critical pods during cluster scale-down. [#8609](https://github.com/deckhouse/deckhouse/pull/8609)
    Ensures `csi-node-*` pods are not terminated before PV-mounted pods are gracefully terminated, preventing hanging pods.
 - **[prometheus]** Fixes to Grafana dashboards for improved data accuracy in summary tables, network consumption, PVC usage and handling Pod restarts. [#9066](https://github.com/deckhouse/deckhouse/pull/9066)
 - **[registry-packages-proxy]** Fix work `registry-packages-proxy` with module sources. [#9112](https://github.com/deckhouse/deckhouse/pull/9112)
    `registry-packages-proxy` should be restarted.
 - **[registry-packages-proxy]** package-proxy in hostNetwork. [#9099](https://github.com/deckhouse/deckhouse/pull/9099)
 - **[user-authn]** Allow to create users with invalid email. [#9171](https://github.com/deckhouse/deckhouse/pull/9171)

## Chore


 - **[candi]** Bump patch versions of Kubernetes images: `v1.27.16`, `v1.28.12`, `v1.29.7`, `v1.30.3` [#9203](https://github.com/deckhouse/deckhouse/pull/9203)
    Kubernetes control-plane components will restart, kubelet will restart.
 - **[candi]** Remove references to static `BASE_SHELL_OPERATOR` image. [#9162](https://github.com/deckhouse/deckhouse/pull/9162)
 - **[chrony]** Optimized permissions and capabilities for chrony and chrony-master. NTP listen ports changed. [#8881](https://github.com/deckhouse/deckhouse/pull/8881)
 - **[cni-cilium]** Distroless images. [#8636](https://github.com/deckhouse/deckhouse/pull/8636)
    All cilium-related pods will restart. L7 and FQDN-based policies will flap.
 - **[deckhouse]** Update addon-operator's version to `v1.4.2`. [#9136](https://github.com/deckhouse/deckhouse/pull/9136)
 - **[deckhouse]** Update addon-operator to `v1.4.1`. [#8285](https://github.com/deckhouse/deckhouse/pull/8285)
 - **[deckhouse-controller]** Set default releaseChannel for ebedded deckhouse policy. [#9155](https://github.com/deckhouse/deckhouse/pull/9155)
 - **[dhctl]** Added resource type field to the terraform plan outputs [#9161](https://github.com/deckhouse/deckhouse/pull/9161)
 - **[docs]** Documented the new `d8 mirror modules` filter feature usage. [#9186](https://github.com/deckhouse/deckhouse/pull/9186)
 - **[ingress-nginx]** Adjust RBAC for Kruise controller. [#9014](https://github.com/deckhouse/deckhouse/pull/9014)
    Kruise controller's pods will be recreated.
 - **[istio]** Remove references to static `BASE_SHELL_OPERATOR` image. [#9162](https://github.com/deckhouse/deckhouse/pull/9162)
 - **[istio]** For all new pods, the CPU limit will be set to 2 in the Istio sidecar container. If this value is too small for your cluster, you can set a higher value in the istio ModuleConfig. To apply the new limits to previously created pods, you will need to restart them manually. [#9036](https://github.com/deckhouse/deckhouse/pull/9036)
    For all new Istio sidecar containers the CPU limit will be set to 2 (you can tune this). To apply the limit to existing pods, you will need to restart them manually.
 - **[istio]** Kiali inherits cluster access rules from an authenticated user (impersonate him), not considering him as anonymous and not provides unwanted privileges. [#8830](https://github.com/deckhouse/deckhouse/pull/8830)
 - **[node-manager]** Add the ability to customize the topology-manager in a NodeGroup. [#7199](https://github.com/deckhouse/deckhouse/pull/7199)
 - **[prometheus]** Disable Grafanav v10 unified alerting navigation. [#9150](https://github.com/deckhouse/deckhouse/pull/9150)
 - **[prometheus]** Update Grafana to `v10.4.5`. [#9088](https://github.com/deckhouse/deckhouse/pull/9088)
 - **[registrypackages]** Update d8-cli to `v0.3.1`. [#9207](https://github.com/deckhouse/deckhouse/pull/9207)
 - **[registrypackages]** Updated d8-cli to `v0.3.0`. [#9158](https://github.com/deckhouse/deckhouse/pull/9158)
 - **[runtime-audit-engine]** The webhook-handler port moved from `9680` to `4227` (to the DKP range 4200-4299). [#8887](https://github.com/deckhouse/deckhouse/pull/8887)
    In the runtime-audit-engine, the webhook-handler port moved from `9680` to `4227`. This may require updating firewall policies before upgrading the cluster.
 - **[user-authn]** Validate email and password on user create. [#9059](https://github.com/deckhouse/deckhouse/pull/9059)

