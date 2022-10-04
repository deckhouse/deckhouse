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


 - **[candi]** Fix routes for multi-zonal clusters when using `WithNATInstance` layout. [#2544](https://github.com/deckhouse/deckhouse/pull/2544)
 - **[cloud-provider-yandex]** Fix allowing additional properties for `nodeGroups[*]` and `nodeGroups[*].instanceClass`. [#2504](https://github.com/deckhouse/deckhouse/pull/2504)
 - **[extended-monitoring]** Remove the `D8CertExporterStuck` alert. [#2589](https://github.com/deckhouse/deckhouse/pull/2589)
 - **[global-hooks]** Reduce static requests for control plane Pods. [#2588](https://github.com/deckhouse/deckhouse/pull/2588)
    Control plane components will restart.
 - **[go_lib]** Changed certificate re-issue time to 15 days before expiration to avoid useless `CertificateSecretExpiredSoon` alerts. [#2582](https://github.com/deckhouse/deckhouse/pull/2582)
 - **[ingress-nginx]** Reload ingress controller configuration on `additionalHeaders` field change. This will automatically add configured custom headers to the nginx.conf file without restarting the controller. [#2545](https://github.com/deckhouse/deckhouse/pull/2545)
 - **[istio]** Added missing global validating webhook for istio. Global webhook is enabled when isiod pods for global revision are ready to handle requests.
    Added a hack to restart an istio operator that hangs in an error state.
    Added control plane alerts: 
    - `D8IstioGlobalControlplaneDoesntWork`
    - `D8IstioAdditionalControlplaneDoesntWork` [#2410](https://github.com/deckhouse/deckhouse/pull/2410)
 - **[kube-dns]** kube-dns ExternalName Service fix — clusterDomain is taken into account. [#2430](https://github.com/deckhouse/deckhouse/pull/2430)
 - **[linstor]** Fix scheduling CNI Pods on tainted nodes. [#2551](https://github.com/deckhouse/deckhouse/pull/2551)
    CNI plugin Pods and `kube-proxy` will be restarted.
 - **[monitoring-kubernetes]** Add deployments to kube-state-metrics's allowlist. [#2636](https://github.com/deckhouse/deckhouse/pull/2636)
 - **[node-manager]** Fixed the `D8EarlyOOMPodIsNotReady` alert description. [#2541](https://github.com/deckhouse/deckhouse/pull/2541)

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

