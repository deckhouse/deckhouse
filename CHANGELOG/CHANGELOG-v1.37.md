# Changelog v1.37

## Know before update


 - Control plane components will restart.
 - The majority of Pods owned by Deckhouse will be restarted.

## Features


 - **[candi]** Enabled `EphemeralContainers` feature gate for Kubernetes < 1.23. [#2578](https://github.com/deckhouse/deckhouse/pull/2578)
    control-plane components will be restarted.
 - **[cni-cilium]** 1. All distributions are properly supported when using FQDN policies.
    2. CNP and CCNP Status updates no longer clog the apiserver. [#2550](https://github.com/deckhouse/deckhouse/pull/2550)
 - **[istio]** Added global parameter for configuring tracing sample rate. [#2440](https://github.com/deckhouse/deckhouse/pull/2440)
 - **[monitoring-applications]** Reduce the number of generated service monitors. [#2527](https://github.com/deckhouse/deckhouse/pull/2527)
 - **[monitoring-kubernetes]** Add underrequested Pods to the Capacity Planning dashboard. [#2476](https://github.com/deckhouse/deckhouse/pull/2476)

## Fixes


 - **[candi]** Remove `master` taint if `control-plane` taint was removed in a single node installation. [#2809](https://github.com/deckhouse/deckhouse/pull/2809)
 - **[candi]** Fix Yandex NAT Instance netplan config. [#2730](https://github.com/deckhouse/deckhouse/pull/2730)
    No impact for existing cluster. Only fixes cluster bootstap of `WithNATInstance` layot in Yandex Cloud.
 - **[candi]** Fix routes for multi-zonal clusters when using `WithNATInstance` layout. [#2544](https://github.com/deckhouse/deckhouse/pull/2544)
 - **[cloud-provider-yandex]** Reverted changes in the YandexClusterConfiguration (removed `additionalProperties: false`). [#2649](https://github.com/deckhouse/deckhouse/pull/2649)
 - **[cloud-provider-yandex]** Fix allowing additional properties for `nodeGroups[*]` and `nodeGroups[*].instanceClass`. [#2504](https://github.com/deckhouse/deckhouse/pull/2504)
 - **[control-plane-manager]** Fixed panic when a node with minimal RAM cannot be found. [#2670](https://github.com/deckhouse/deckhouse/pull/2670)
 - **[docs]** Stick sidebar to the top of the page in the documentation. [#2686](https://github.com/deckhouse/deckhouse/pull/2686)
 - **[extended-monitoring]** Remove the `D8CertExporterStuck` alert. [#2589](https://github.com/deckhouse/deckhouse/pull/2589)
 - **[global-hooks]** Reduce static requests for control plane Pods. [#2588](https://github.com/deckhouse/deckhouse/pull/2588)
    Control plane components will restart.
 - **[go_lib]** Changed certificate re-issue time to 15 days before expiration to avoid useless `CertificateSecretExpiredSoon` alerts. [#2582](https://github.com/deckhouse/deckhouse/pull/2582)
 - **[ingress-nginx]** Increase Ingress validation webhook timeout. [#2812](https://github.com/deckhouse/deckhouse/pull/2812)
 - **[ingress-nginx]** Reload ingress controller configuration on `additionalHeaders` field change. This will automatically add configured custom headers to the nginx.conf file without restarting the controller. [#2545](https://github.com/deckhouse/deckhouse/pull/2545)
 - **[istio]** Fix the `D8IstioGlobalControlplaneDoesntWork` alert. [#2714](https://github.com/deckhouse/deckhouse/pull/2714)
 - **[istio]** Fix `D8IstioAdditionalControlplaneDoesntWork` alert. [#2665](https://github.com/deckhouse/deckhouse/pull/2665)
 - **[istio]** Added missing global validating webhook for istio. Global webhook is enabled when isiod pods for global revision are ready to handle requests.
    Added a hack to restart an istio operator that hangs in an error state.
    Added control plane alerts: 
    - `D8IstioGlobalControlplaneDoesntWork`
    - `D8IstioAdditionalControlplaneDoesntWork` [#2410](https://github.com/deckhouse/deckhouse/pull/2410)
 - **[kube-dns]** Added "prefer_udp" to stub zones. [#2800](https://github.com/deckhouse/deckhouse/pull/2800)
 - **[kube-dns]** kube-dns ExternalName Service fix — clusterDomain is taken into account. [#2430](https://github.com/deckhouse/deckhouse/pull/2430)
 - **[linstor]** Improve kernel-headers detection for СentOS. [#2641](https://github.com/deckhouse/deckhouse/pull/2641)
 - **[linstor]** Fix scheduling CNI Pods on tainted nodes. [#2551](https://github.com/deckhouse/deckhouse/pull/2551)
    CNI plugin Pods and `kube-proxy` will be restarted.
 - **[log-shipper]** Stop generating pointless 'parse_json' transform, which improves performance. [#2662](https://github.com/deckhouse/deckhouse/pull/2662)
 - **[log-shipper]** Fix the bug when the many sources point to the same input and only the last is working. [#2662](https://github.com/deckhouse/deckhouse/pull/2662)
 - **[monitoring-kubernetes]** Add deployments to kube-state-metrics's allowlist. [#2636](https://github.com/deckhouse/deckhouse/pull/2636)
 - **[node-manager]** Avoid "node-role.kubernetes.io/master" taint removal when it is explicitly set in the master NG [#2831](https://github.com/deckhouse/deckhouse/pull/2831)
 - **[node-manager]** Added VPA for early-oom daemonset. [#2695](https://github.com/deckhouse/deckhouse/pull/2695)
 - **[node-manager]** Fixed failing on not existing control-plane node labels. [#2670](https://github.com/deckhouse/deckhouse/pull/2670)
 - **[node-manager]** Fixed the `D8EarlyOOMPodIsNotReady` alert description. [#2541](https://github.com/deckhouse/deckhouse/pull/2541)
 - **[upmeter]** Reduced API calls throttling by the deduplication of preflight checks in probes. [#2687](https://github.com/deckhouse/deckhouse/pull/2687)
 - **[upmeter]** Fixed empty status page when there is no data in the upmeter server. [#2683](https://github.com/deckhouse/deckhouse/pull/2683)
 - **[upmeter]** Reduces QPS and burst in `upmeter-agent` to reduce `kube-apiserver` latency in multi-control-plane setups. [#2668](https://github.com/deckhouse/deckhouse/pull/2668)

## Chore


 - **[candi]** Bump alpine base image to `3.16.2`. [#2574](https://github.com/deckhouse/deckhouse/pull/2574)
    The majority of Pods owned by Deckhouse will be restarted.
 - **[candi]** Remove deprecated hooks and scripts. [#2491](https://github.com/deckhouse/deckhouse/pull/2491)
 - **[cilium-hubble]** Set the `cilium-hubble` module disabled by default. [#2581](https://github.com/deckhouse/deckhouse/pull/2581)
 - **[istio]** Added an alert that `tlsMode` module parameter will be removed in Deckhouse v1.38.0. [#2573](https://github.com/deckhouse/deckhouse/pull/2573)
 - **[istio]** - Istio global mutating webhook allows injecting istio sidecars into individual Pods. 
    - Redesigned logic for enabling istio-sidecar for the `ingress-nginx` module. [#2546](https://github.com/deckhouse/deckhouse/pull/2546)
 - **[istio]** Added an istio sidecar template to prevent istio-proxy from terminating before the main application's network sockets are closed. [#2513](https://github.com/deckhouse/deckhouse/pull/2513)
 - **[monitoring-kubernetes]** Remove `kube-dns` DNS application Grafana dashboard. [#2542](https://github.com/deckhouse/deckhouse/pull/2542)
 - **[node-manager]** Added automatic migration to the `control-plane` node role. [#2670](https://github.com/deckhouse/deckhouse/pull/2670)

