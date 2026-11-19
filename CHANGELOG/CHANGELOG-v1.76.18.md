# Changelog v1.76.18

## Know before update


 - Cilium agent pods will be restarted.
 - Cilium agent pods will be restarted. With DSR, the DSR IPv4 option is stripped before the packet reaches the pod.
 - Flannel pods will be restarted.
 - Flannel pods will be restarted. The guard metrics use host port 4294.
 - TLS connections from nodes to hosts whose certificates chain to `emSign ECC Root CA - G3` or Comodo `AAA Certificate Services` (eMudhra / emSign issued) will fail. Reissue such certificates from another CA before the update.
 - simple-bridge pods will be restarted.
 - simple-bridge pods will be restarted. The guard metrics use host port 4294.

## Features


 - **[cni-cilium]** Added a CVE-2026-98432 mitigation that filters IPv4 TCP SYN packets with IP options in the host and pod network namespaces (`ipv4OptionsGuard`; `Enforce` for new clusters, `Audit` for existing ones).
    Cilium agent pods will be restarted. With DSR, the DSR IPv4 option is stripped before the packet reaches the pod.
 - **[cni-flannel]** Added a CVE-2026-98432 mitigation that filters IPv4 TCP SYN packets with IP options in the host and pod network namespaces (`ipv4OptionsGuard`; `Enforce` for new clusters, `Audit` for existing ones).
    Flannel pods will be restarted. The guard metrics use host port 4294.
 - **[cni-simple-bridge]** Added a CVE-2026-98432 mitigation that filters IPv4 TCP SYN packets with IP options in the host and pod network namespaces (`ipv4OptionsGuard`; `Enforce` for new clusters, `Audit` for existing ones).
    simple-bridge pods will be restarted. The guard metrics use host port 4294.

## Fixes


 - **[cloud-provider-vcd]** Reduce node initialization delays and restore automatic cleanup of orphaned Nodes in VCD clusters.
 - **[cni-cilium]** The ipv4-options-guard sidecar no longer writes to the host `/etc`, so Cilium agents start on Deckhouse Engine nodes.
    Cilium agent pods will be restarted.
 - **[cni-flannel]** The ipv4-options-guard sidecar no longer writes to the host `/etc`, so Flannel pods start on Deckhouse Engine nodes.
    Flannel pods will be restarted.
 - **[cni-simple-bridge]** The ipv4-options-guard sidecar no longer writes to the host `/etc`, so simple-bridge pods start on Deckhouse Engine nodes.
    simple-bridge pods will be restarted.
 - **[common]** Updated the shared static nginx binary to 1.30.7 to address security vulnerabilities.
    Pods using the updated nginx images roll out in the affected modules, including the ingress failover proxy where enabled.
 - **[deckhouse-controller]** A pending module release no longer loses its Kubernetes, Deckhouse and module requirements, so an update that the cluster does not satisfy stays pending instead of disabling the module.
 - **[deckhouse-controller]** Module sources now scan the registry every 30 minutes by default, reducing registry load
 - **[deckhouse-controller]** Pre-download modules enabled by the bundle when their ModuleConfig omits enabled, preventing upgrades from getting stuck.
 - **[deckhouse-controller]** Prevent module hooks from running with unavailable snapshots while waiting for Deckhouse to restart after a module update
 - **[deckhouse-controller]** Releases of a still-embedded module no longer raise manual approval alerts once the module has a deployed release.
 - **[deckhouse]** Show the Terminating phase while package repositories are being deleted
 - **[dhctl]** Added the ability to retry a timed-out node drain during converge, and raised drain attempts from 5 to 10.
 - **[ingress-nginx]** Nginx was updated to 1.30.7.
    All ingress-nginx controllers' pods will be restarted.
 - **[istio]** Fixed CVEs in istio images (x/net, x/crypto, klauspost/compress, x/text).
    The Istio pods will be restarted. Application sidecars will be updated only after the application pods are recreated.
 - **[istio]** Fixed ambient mode on nodes with legacy iptables, where `istio-cni-node` crashed on Istio 1.25 and used nftables instead of legacy iptables on Istio 1.27 and later.
    In ambient mode with Istio 1.27 or later, on nodes with legacy iptables, `istio-cni-node` switches from nftables to legacy iptables. Pods that were added to the mesh before the update keep their nftables rules. Restart these pods after the update. Otherwise, removing such a pod from the mesh without a restart leaves its old redirect rules in place, and the pod loses network connectivity.
 - **[registry-packages-proxy]** Fixed registry-packages-proxy hanging after a cache error and serving corrupted packages, which blocked node updates.
 - **[registrypackages]** Nodes stop trusting `emSign ECC Root CA - G3` and Comodo `AAA Certificate Services`, the roots behind the compromised eMudhra intermediate CAs.
    TLS connections from nodes to hosts whose certificates chain to `emSign ECC Root CA - G3` or Comodo `AAA Certificate Services` (eMudhra / emSign issued) will fail. Reissue such certificates from another CA before the update.
