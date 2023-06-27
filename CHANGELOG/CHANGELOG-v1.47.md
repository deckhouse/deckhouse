# Changelog v1.47

## Know before update


 - All nodes with linstor will be restarted.
    * If NodeGroup `spec.disruptions.approvalMode` is set to `Manual`, you will receive a `NodeRequiresDisruptionApprovalForUpdate` alert.
    * If NodeGroup `spec.disruptions.approvalMode` is set to `Automatic`, nodes will be drained and restarted one by one.
 - Ingress nginx controller will restart.

## Features


 - **[candi]** Not keep compressed image layers in containerd's content store once they have been unpacked. [#4843](https://github.com/deckhouse/deckhouse/pull/4843)
    All `containerd` daemons will restart.
 - **[cni-flannel]** Images are based on a distroless image. [#4635](https://github.com/deckhouse/deckhouse/pull/4635)
 - **[deckhouse]** List all Deckhouse modules as CR. Use the `kubectl get modules` command to browse all modules. [#4478](https://github.com/deckhouse/deckhouse/pull/4478)
 - **[deckhouse-controller]** Move `tools/change-registry.sh` to `deckhouse-controller`. [#4925](https://github.com/deckhouse/deckhouse/pull/4925)
 - **[ingress-nginx]** Set `ingressClass` to `nginx` if not explicitly set. [#4927](https://github.com/deckhouse/deckhouse/pull/4927)
 - **[ingress-nginx]** Images are based on a distroless image. [#4635](https://github.com/deckhouse/deckhouse/pull/4635)
    Ingress nginx controller will restart.
 - **[istio]** A new non-public label to discard metrics scraping from application namespaces. [#4873](https://github.com/deckhouse/deckhouse/pull/4873)
 - **[istio]** Splitting istio to CE (basic functionality) and EE (extra functionality) versions. [#4171](https://github.com/deckhouse/deckhouse/pull/4171)
 - **[kube-dns]** Images are based on a distroless image. [#4635](https://github.com/deckhouse/deckhouse/pull/4635)
 - **[kube-proxy]** Images are based on a distroless image. [#4635](https://github.com/deckhouse/deckhouse/pull/4635)
 - **[linstor]** Added params for enabled SELinux support. [#4652](https://github.com/deckhouse/deckhouse/pull/4652)
    linstor satellite Pods will be restarted.
 - **[monitoring-deckhouse]** Add Debian 9 and Ubuntu 18.04 to `D8NodeHasDeprecatedOSVersion` alert. [#4862](https://github.com/deckhouse/deckhouse/pull/4862)
 - **[monitoring-kubernetes]** Images are based on a distroless image. [#4635](https://github.com/deckhouse/deckhouse/pull/4635)
 - **[node-manager]** Change calculation of `condition.ready` in a `NodeGroup`. [#4855](https://github.com/deckhouse/deckhouse/pull/4855)
 - **[operator-trivy]** Add configuration for `tolerations` and `nodeSelector` for module. [#4721](https://github.com/deckhouse/deckhouse/pull/4721)
 - **[prometheus]** Add module external labels setting. [#4968](https://github.com/deckhouse/deckhouse/pull/4968)
 - **[user-authn]** Allow users to deploy DexAuthenticator trusted by Kubernetes. [#5007](https://github.com/deckhouse/deckhouse/pull/5007)

## Fixes


 - **[cilium-hubble]** Fix the error with the install if the [modules.https.mode](https://deckhouse.io/documentation/v1/deckhouse-configure-global.html#parameters-modules-https-mode) global parameter is `OnlyInURI`. [#4846](https://github.com/deckhouse/deckhouse/pull/4846)
 - **[dashboard]** Fix the logout button (it doesn't appear). [#4929](https://github.com/deckhouse/deckhouse/pull/4929)
 - **[deckhouse-controller]** Add unit tests for change-registry. [#4949](https://github.com/deckhouse/deckhouse/pull/4949)
 - **[dhctl]** Fix parsing node index (CWE-190, CWE-681). [#5023](https://github.com/deckhouse/deckhouse/pull/5023)
 - **[dhctl]** Fix cut off terraform output. [#4800](https://github.com/deckhouse/deckhouse/pull/4800)
 - **[external-module-manager]** Prevent path traversal on zip unpacking [#5024](https://github.com/deckhouse/deckhouse/pull/5024)
 - **[global-hooks]** Delete `d8-deckhouse-validating-webhook-handler` validating webhook configurations [#5032](https://github.com/deckhouse/deckhouse/pull/5032)
 - **[ingress-nginx]** Pathch Kruse controller manager logic so that it doesn't delete more than `maxUnavailable` Pods during updates. [#5039](https://github.com/deckhouse/deckhouse/pull/5039)
    Kruise controller manager will be restarted.
 - **[monitoring-kubernetes]** Fix `kubelet-eviction-thresholds-exporter` Prometheus metric and `node-disk-usage` Prometheus rules. [#4888](https://github.com/deckhouse/deckhouse/pull/4888)
 - **[prometheus]** Fix scheme for web exported URL on Grafana main page. [#4895](https://github.com/deckhouse/deckhouse/pull/4895)
 - **[runtime-audit-engine]** Unset `FALCO_BPF_PROBE` environment variable for the Falco container. [#4931](https://github.com/deckhouse/deckhouse/pull/4931)
 - **[runtime-audit-engine]** Bump Falco version to `v0.35.0`. [#4894](https://github.com/deckhouse/deckhouse/pull/4894)
    default
 - **[user-authn]** Do not send groups header from `DexAuthenticator`. [#5027](https://github.com/deckhouse/deckhouse/pull/5027)
 - **[user-authz]** Fix access for `PrivilegedUser` role. [#4903](https://github.com/deckhouse/deckhouse/pull/4903)
 - **[user-authz]** Forbid empty `.spec.subject` field in `ClusterAuthorizationRule`. [#4850](https://github.com/deckhouse/deckhouse/pull/4850)

## Chore


 - **[candi]** Bump patch versions of Kubernetes images: `v1.24.15`, `v1.25.11`, `v1.26.6` [#4975](https://github.com/deckhouse/deckhouse/pull/4975)
    "Kubernetes control-plane components will restart, kubelet will restart"
 - **[candi]** Upgraded patch versions of Kubernetes images `1.24.14`, `1.25.10`, `1.26.5`. [#4725](https://github.com/deckhouse/deckhouse/pull/4725)
    Kubernetes control plane components will restart, kubelet will restart.
 - **[dashboard]** Add validations for the `ingressClass` field. [#4932](https://github.com/deckhouse/deckhouse/pull/4932)
 - **[deckhouse]** Add cluster-autoscaler logs into debug logs collector. [#4848](https://github.com/deckhouse/deckhouse/pull/4848)
 - **[documentation]** Add validations for the `ingressClass` field. [#4932](https://github.com/deckhouse/deckhouse/pull/4932)
 - **[flant-integration]** Set default schema for Grafana URL to 'http' in case there is problem to fetch data from the cluster. [#4978](https://github.com/deckhouse/deckhouse/pull/4978)
 - **[flant-integration]** Bump the `requests` dependency library from `2.28.1` to `2.31.0`. [#4899](https://github.com/deckhouse/deckhouse/pull/4899)
 - **[global-hooks]** Add validations for the `ingressClass` field. [#4932](https://github.com/deckhouse/deckhouse/pull/4932)
 - **[ingress-nginx]** Provide some recommendations in the `D8NginxIngressKruiseControllerPodIsRestartingTooOften` alert. [#4995](https://github.com/deckhouse/deckhouse/pull/4995)
 - **[ingress-nginx]** Rename the alert about the malfunctioning Kruise controller in `d8-ingress-nginx` namespace. [#4981](https://github.com/deckhouse/deckhouse/pull/4981)
 - **[ingress-nginx]** Add validations for the `ingressClass` field. [#4932](https://github.com/deckhouse/deckhouse/pull/4932)
 - **[linstor]** Update DRBD to `9.2.4`. [#4882](https://github.com/deckhouse/deckhouse/pull/4882)
    All nodes with linstor will be restarted.
    * If NodeGroup `spec.disruptions.approvalMode` is set to `Manual`, you will receive a `NodeRequiresDisruptionApprovalForUpdate` alert.
    * If NodeGroup `spec.disruptions.approvalMode` is set to `Automatic`, nodes will be drained and restarted one by one.
 - **[openvpn]** Add validations for the `ingressClass` field. [#4932](https://github.com/deckhouse/deckhouse/pull/4932)
 - **[prometheus]** Add validations for the `ingressClass` field. [#4932](https://github.com/deckhouse/deckhouse/pull/4932)
 - **[upmeter]** Add validations for the `ingressClass` field. [#4932](https://github.com/deckhouse/deckhouse/pull/4932)
 - **[user-authn]** Add validations for the `ingressClass` field. [#4932](https://github.com/deckhouse/deckhouse/pull/4932)
 - **[user-authn-crd]** Add validations for the `applicationIngressClassName` and `applicationIngressCertificateSecretName` fields. [#4932](https://github.com/deckhouse/deckhouse/pull/4932)

