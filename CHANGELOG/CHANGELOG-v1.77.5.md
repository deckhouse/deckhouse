# Changelog v1.77.5

## Know before update


 - Cilium agent and operator pods will be restarted.
 - Control plane components and kubelet will be restarted.
 - CoreDNS pods will be restarted.
 - TLS connections from nodes to hosts whose certificates chain to `emSign ECC Root CA - G3` or Comodo `AAA Certificate Services` (eMudhra / emSign issued) will fail. Reissue such certificates from another CA before the update.
 - cni-flannel pods will be restarted.
 - kube-proxy pods will be restarted.
 - network-policy-engine pods will be restarted.
 - node-local-dns pods will be restarted.

## Features


 - **[control-plane-manager]** The basic audit policy logs Pod status updates from kubelet with the request body, so container terminations and their reasons get into the audit log.
 - **[user-authn]** DexAuthenticator logins, logouts, failed and denied logins, CSRF mismatches and failed session refreshes are delivered as security events when the `security-events-manager` module is enabled.

## Fixes


 - **[admission-policy-engine]** Fixed CVE-2026-114437 in `github.com/go-chi/chi/v5` and CVE-2026-63209 in `github.com/klauspost/compress`.
 - **[candi]** The first master bootstrap waits for `kube-apiserver` readiness instead of failing the control-plane step on `connection refused`.
 - **[cert-manager]** The metrics proxy of the controller is no longer restarted by its liveness probe while its node is short of CPU.
    The `cert-manager` controller pod restarts.
 - **[cilium-hubble]** Fixed a known vulnerability in the Hubble relay image.
 - **[cloud-provider-dvp]** Fixed deletion of load balancers backed by ServiceWithHealthchecks, which failed because the child Service is protected by the `heritage - deckhouse` label in the parent cluster.
 - **[cloud-provider-huaweicloud]** Recreated the etcd disk when a master VM is recreated. [#20801](https://github.com/deckhouse/deckhouse/pull/20801)
 - **[cloud-provider-openstack]** CAPI now mirrors MCM — `configDrive` honors `externalNetworkDHCP`, and additional networks always produce ports
 - **[cloud-provider-vcd]** Fixes the module getting stuck in Error on upgrade from 1.76 because of the immutable selector of the `capcd-controller-manager` Deployment.
 - **[cni-cilium]** Fixed a known vulnerability in the Cilium images.
    Cilium agent and operator pods will be restarted.
 - **[cni-flannel]** Fixed known vulnerabilities in the flanneld image.
    cni-flannel pods will be restarted.
 - **[control-plane-manager]** Control plane components are no longer restarted in parallel when a node is NotReady or after a certificate renewal.
 - **[control-plane-manager]** Updated Go dependencies of Kubernetes control plane components to fix known vulnerabilities.
    Control plane components and kubelet will be restarted.
 - **[deckhouse-controller]** A module enabled in the bundle is no longer disabled when its ModuleConfig does not set `enabled`.
 - **[deckhouse-controller]** Module sources now scan the registry every 30 minutes by default, reducing registry load
 - **[deckhouse]** Restore ClusterAdmin read access to ModuleSettingsDefinition so the console can show module settings again.
 - **[deckhouse]** Show the Terminating phase while package repositories are being deleted
 - **[deckhouse]** Update d8 CLI and scoped runtime VEX exceptions for known vulnerabilities.
 - **[istio]** Fixed `alliance-healthcheck` keeping the sidecar of the previous Istio version after a `globalVersion` change.
 - **[istio]** Fixed istio module installation with nelm v2 when Istio is managed by the istio operator (versions below 1.27).
 - **[kube-dns]** Fixed a known vulnerability in the CoreDNS image.
    CoreDNS pods will be restarted.
 - **[kube-proxy]** Fixed known vulnerabilities in the kube-proxy images.
    kube-proxy pods will be restarted.
 - **[kube-proxy]** Fixed known vulnerabilities in the kube-proxy init-container image.
    kube-proxy pods will be restarted.
 - **[multitenancy-manager]** Fixed CVE-2026-63209 in `github.com/klauspost/compress`.
 - **[network-policy-engine]** Fixed known vulnerabilities in the kube-router image.
    network-policy-engine pods will be restarted.
 - **[node-local-dns]** Fixed DNS errors during node-local-dns pod restarts in clusters with Cilium.
 - **[node-local-dns]** Fixed a known vulnerability in the node-local-dns CoreDNS image.
    node-local-dns pods will be restarted.
 - **[openvpn]** Fixed known vulnerabilities in the ovpn-admin and easyrsa-migrator images.
 - **[registry]** In the `Proxy` mode, reuse upstream auth challenges and tokens instead of requesting them before every request to the upstream registry.
 - **[registrypackages]** Nodes stop trusting `emSign ECC Root CA - G3` and Comodo `AAA Certificate Services`, the roots behind the compromised eMudhra intermediate CAs.
    TLS connections from nodes to hosts whose certificates chain to `emSign ECC Root CA - G3` or Comodo `AAA Certificate Services` (eMudhra / emSign issued) will fail. Reissue such certificates from another CA before the update.
 - **[registrypackages]** Update containerdv2 patch
 - **[user-authn]** A password reset of a local user keeps the replaced password in the password history, and the password change form refuses the current password in every case.
    After a reset, the previous password cannot be reused when passwordHistoryLimit is set; the current password cannot be set again through the change form, and a lowered passwordHistoryLimit applies at the next password change.
 - **[user-authn]** dex-authenticator no longer writes the request headers, cookies included, to the log when the CSRF cookie is missing.
