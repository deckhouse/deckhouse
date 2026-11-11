# Changelog v1.77.7

## Know before update


 - Cilium agent pods will be restarted.
 - Flannel pods will be restarted.
 - On a NodeGroup with `spec.gpu`, a failing GPU check used to abort the bashible run and be retried forever, so the node never configured kubelet and never received updates from any other module. Such a node now converges and only its GPU stays unusable. If `/usr/bin/nvidia-container-runtime` is absent, containerd is now configured with `runc` as the default runtime instead of `nvidia`; on a node whose configuration previously selected `nvidia` this changes the containerd config and therefore restarts containerd. Previously such a node could not start any container at all.
 - Pods of the kube-proxy, cni-cilium, cni-flannel, and cni-simple-bridge agent DaemonSets will be restarted because their cni-migration-init-checker init container image is updated.
 - node-controller, capi-controller-manager, machine-controller-manager and bashible-apiserver do not run on a master tainted with a key from `global.modules.placement.customTolerationKeys`. Remove such taints from the master NodeGroup before the update.
 - simple-bridge pods will be restarted.

## Features


 - **[node-manager]** Alert when a NodeGroup uses the deprecated built-in GPU support while the `gpu` module is disabled.

## Fixes


 - **[candi]** A GPU problem no longer blocks node convergence.
    On a NodeGroup with `spec.gpu`, a failing GPU check used to abort the bashible run and be retried forever, so the node never configured kubelet and never received updates from any other module. Such a node now converges and only its GPU stays unusable. If `/usr/bin/nvidia-container-runtime` is absent, containerd is now configured with `runc` as the default runtime instead of `nvidia`; on a node whose configuration previously selected `nvidia` this changes the containerd config and therefore restarts containerd. Previously such a node could not start any container at all.
 - **[candi]** A new cloud node sets up the provider network before it installs bootstrap packages, so it can download them from a master in another zone.
 - **[candi]** Fixed an intermittent bootstrap hang where kube-apiserver never started because kubelet read its manifest while it was being written.
 - **[candi]** GPU compute capability check no longer requires `bc`.
 - **[candi]** Persist `net.core.bpf_jit_harden` for GPU nodes in an own sysctl drop-in instead of editing distribution files that may be absent.
 - **[candi]** bashible verifies the rpp-get binary against the image digest from its bundle before running it; a substituted bootstrap proxy on a master can no longer hand a binary to the nodes.
    A node that joins while a registry-packages-proxy pod of the previous release is still running on a master waits until a pod of this release answers.
 - **[cloud-provider-dvp]** Creates ModuleConfig/cloud-provider-dvp automatically when it is missing.
 - **[cloud-provider-dvp]** Fixes DVP CSI staging so volumes with filesystems detected by Kubernetes SafeFormatAndMount are preserved and unformatted volumes are formatted without using lsblk.
 - **[cni-cilium]** The ipv4-options-guard sidecar no longer writes to the host `/etc`, so Cilium agents start on Deckhouse Engine nodes.
    Cilium agent pods will be restarted.
 - **[cni-flannel]** The ipv4-options-guard sidecar no longer writes to the host `/etc`, so Flannel pods start on Deckhouse Engine nodes.
    Flannel pods will be restarted.
 - **[cni-simple-bridge]** The ipv4-options-guard sidecar no longer writes to the host `/etc`, so simple-bridge pods start on Deckhouse Engine nodes.
    simple-bridge pods will be restarted.
 - **[common]** Fixed known vulnerabilities in the cni-migration-controller and cni-migration-init-checker images.
    Pods of the kube-proxy, cni-cilium, cni-flannel, and cni-simple-bridge agent DaemonSets will be restarted because their cni-migration-init-checker init container image is updated.
 - **[common]** Updated `golang.org/x/net`, `golang.org/x/text`, `google.golang.org/grpc` and `github.com/klauspost/compress` in CSI sidecars to fix CVEs.
 - **[control-plane-manager]** Preserve custom TLS certificates when enabling a default Gateway.
 - **[control-plane-manager]** The migration hook for the api listener is added.
 - **[deckhouse-controller]** Fixed image pulls for modules served by ModulePullOverride when another source updates its registry or the override is not ready during Deckhouse startup.
 - **[local-path-provisioner]** Updated `golang.org/x/net`, `golang.org/x/text` and `github.com/klauspost/compress` to fix CVEs.
 - **[metallb]** Fixed BGP migration to preserve pool allocation flags and allow users to manage migrated resources after switching ModuleConfig to version 3.
 - **[node-manager]** A node-manager release no longer waits for capi-controller-manager when neither capi nor its webhooks change.
 - **[node-manager]** Fixed constant NodeGroup status updates by node-controller on every Node update.
 - **[node-manager]** Master controllers leave an unreachable master after about 100 seconds and are not evicted from a single master.
 - **[node-manager]** Master controllers no longer tolerate keys from `customTolerationKeys`, so a key that repeats a built-in one no longer blocks pod updates.
    node-controller, capi-controller-manager, machine-controller-manager and bashible-apiserver do not run on a master tainted with a key from `global.modules.placement.customTolerationKeys`. Remove such taints from the master NodeGroup before the update.
 - **[registry-packages-proxy]** The bootstrap port serves the manifest and the layer of the rpp-get image, so a node can verify the binary it installs.
 - **[registry]** Fixed known vulnerabilities in the docker-auth image.
    The registry in-cluster proxy and node services pods will be restarted.
 - **[registry]** The update to DP 1.78 is held as soon as `registry.mode` `Proxy` or `Local`, or a switch between `Direct` and `Unmanaged`, is set in the `deckhouse` ModuleConfig, and the message of the held release lists the migration steps for each mode.
    A cluster that uses `Proxy` or `Local` of the previous registry implementation, or `Direct` without the `registry` ModuleConfig, is not updated to DP 1.78, as before. This now also applies from the moment such a mode is set in the `deckhouse` ModuleConfig, before the switch starts. Migrate as described in "Migrating from the previous implementation to the current one" in the registry documentation. Clusters already prepared for the migration are not affected.
 - **[registrypackages]** rpp-get tries every registry-packages-proxy endpoint before an attempt fails and retries connection timeouts, so a node that reaches only the master of its own zone during bootstrap joins the cluster.
 - **[terraform-manager]** Fixed known vulnerabilities in opentofu.
    The terraform-manager pods will be restarted.
 - **[user-authn]** Default DKP gateway requirement is removed from dex-authenticator template.

## Chore


 - **[candi]** Base images updated to v3.1.24
 - **[candi]** base images updated to 3.1.23
 - **[node-manager]** Update cluster-autoscaler to gardener v1.34.4 and v1.35.2.
 - **[vertical-pod-autoscaler]** Update VPA to upstream 1.7.3.
