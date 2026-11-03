# Changelog v1.77.6

## Know before update


 - Cilium agent pods will be restarted. With DSR, the DSR IPv4 option is stripped before the packet reaches the pod.
 - Flannel pods will be restarted.
 - The metallb components (controller, speaker, l2lb) will restart after the update.
 - kube-proxy pods will be restarted.
 - simple-bridge pods will be restarted.

## Features


 - **[cni-cilium]** Added a CVE-2026-98432 mitigation that filters IPv4 TCP SYN packets with IP options in the host and pod network namespaces (`ipv4OptionsGuard`; `Enforce` for new clusters, `Audit` for existing ones).
    Cilium agent pods will be restarted. With DSR, the DSR IPv4 option is stripped before the packet reaches the pod.
 - **[cni-flannel]** Added a CVE-2026-98432 mitigation that filters IPv4 TCP SYN packets with IP options in the host and pod network namespaces (`ipv4OptionsGuard`; `Enforce` for new clusters, `Audit` for existing ones).
    Flannel pods will be restarted.
 - **[cni-simple-bridge]** Added a CVE-2026-98432 mitigation that filters IPv4 TCP SYN packets with IP options in the host and pod network namespaces (`ipv4OptionsGuard`; `Enforce` for new clusters, `Audit` for existing ones).
    simple-bridge pods will be restarted.

## Fixes


 - **[admission-policy-engine]** An unavailable Gatekeeper no longer blocks system components in namespaces that opted into security policy checks, which could deadlock the cluster together with the CNI agent.
 - **[admission-policy-engine]** Fixed CVE-2026-116585, CVE-2026-116586 and CVE-2026-116587 in `github.com/go-jose/go-jose/v4` and CVE-2026-78661 and CVE-2026-97045 in `golang.org/x/net`.
 - **[cloud-provider-openstack]** Fixes creation of nodes with `rootDiskSize` when the node availability zone does not exist in Cinder.
 - **[cloud-provider-openstack]** fix CloudEphemeral node creation in non-default availability zones via automatic Cinder-AZ discovery and new OpenStackInstanceClass.volumeTypeMap field
 - **[cni-cilium]** Fixed file descriptor, connection and lock leaks in Cilium found by static analysis.
 - **[control-plane-manager]** Fixed a goroutine leak in the helm release deprecation scan when listing releases fails.
 - **[control-plane-manager]** The basic audit policy records an exec into a pod (`pods/exec`) in all namespaces again, as it did before Deckhouse 1.77.
 - **[deckhouse-controller]** A pending module release no longer loses its Kubernetes, Deckhouse and module requirements, so an update that the cluster does not satisfy stays pending instead of disabling the module.
 - **[deckhouse-controller]** Prevent module hooks from running with unavailable snapshots while waiting for Deckhouse to restart after a module update
 - **[deckhouse]** Resolve `registry.d8-system.svc` to a node running the in-cluster registry proxy during bootstrap, instead of always to `127.0.0.1`.
    Applying a discovered address restarts the Deckhouse Pod once.
 - **[dhctl]** Keep the SSH host list and master readiness checks in sync with the infrastructure state during master converge.
 - **[istio]** Fixed ambient mode on nodes with legacy iptables, where `istio-cni-node` crashed on Istio 1.25 and used nftables instead of legacy iptables on Istio 1.27 and later.
    In ambient mode with Istio 1.27 or later, on nodes with legacy iptables, `istio-cni-node` switches from nftables to legacy iptables. Pods that were added to the mesh before the update keep their nftables rules. Restart these pods after the update. Otherwise, removing such a pod from the mesh without a restart leaves its old redirect rules in place, and the pod loses network connectivity.
 - **[kube-proxy]** Fixed known vulnerabilities in the kube-proxy images.
    kube-proxy pods will be restarted.
 - **[local-path-provisioner]** `config.json` of the provisioner is serialized from a structure, and the CRD accepts in `spec.path` only an absolute path without quotes, backslashes, whitespace and control characters. A crafted path can no longer add manifests to the module release.
 - **[metallb]** Fixed CVE-2026-78661 and CVE-2026-97045 in the metallb and l2lb images.
    The metallb components (controller, speaker, l2lb) will restart after the update.
 - **[multitenancy-manager]** Fixed CVE-2026-116585, CVE-2026-116586 and CVE-2026-116587 in `github.com/go-jose/go-jose/v4` and CVE-2026-78661 and CVE-2026-97045 in `golang.org/x/net`.
 - **[registry-packages-proxy]** Fixed registry-packages-proxy hanging after a cache error and serving corrupted packages, which blocked node updates.
 - **[registry]** Require brackets around IPv6 upstream registry URL hosts and accept bracketed addresses with or without a port.
 - **[registrypackages]** Support rebuilding legacy image layers and preserve verity data in CSE.
 - **[service-with-healthchecks]** Status updates were refactored.
    The controller and agent pods will be restared.
 - **[user-authn]** Fixed CVE-2026-116585, CVE-2026-116586 and CVE-2026-116587 in `github.com/go-jose/go-jose/v4` and CVE-2026-78661 and CVE-2026-97045 in `golang.org/x/net`.
 - **[user-authn]** Local users keep the passwords they changed in Dex when the cluster updates from 1.76.
 - **[user-authz]** Fixed CVE-2026-78661 and CVE-2026-97045 in `golang.org/x/net`.
 - **[vertical-pod-autoscaler]** Fix a panic in vpa-updater and vpa-admission-controller when a pod with a container without resources is capped to a LimitRange of type Pod.
