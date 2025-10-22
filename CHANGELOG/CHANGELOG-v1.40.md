# Changelog v1.40

## Know before update


 - Certificates in the old API group (you can check presence via `kubectl get certificates.certmanager.k8s.io  -A`) will not be renewed anymore.
 - Ingress-nginx v1.1 pods will be restarted.
 - Pods in the following modules will be restarted: admission-policy-engine, cni-cilium, kube-proxy, cloud-provider-yandex, node-local-dns, node-manager, metallb, terraform-manager, linstor, kube-dns, snapshot-controller, cert-manager, user-authn, vertical-pod-autoscaler, extended-monitoring, monitoring-kubernetes, ingress-nginx, log-shipper, pod-reloader, dashboard, openvpn, upmeter.
 - The `monitoring-kubernetes-control-plane` module now works only if the `control-plane-manager` module is enabled.

## Features


 - **[cert-manager]** Removed deprecated legacy cert-manager (`certmanager.k8s.io` API group). [#2947](https://github.com/deckhouse/deckhouse/pull/2947)
    Certificates in the old API group (you can check presence via `kubectl get certificates.certmanager.k8s.io  -A`) will not be renewed anymore.
 - **[common]** Updated kube-rbac-proxy. [#2905](https://github.com/deckhouse/deckhouse/pull/2905)
    Pods in the following modules will be restarted: admission-policy-engine, cni-cilium, kube-proxy, cloud-provider-yandex, node-local-dns, node-manager, metallb, terraform-manager, linstor, kube-dns, snapshot-controller, cert-manager, user-authn, vertical-pod-autoscaler, extended-monitoring, monitoring-kubernetes, ingress-nginx, log-shipper, pod-reloader, dashboard, openvpn, upmeter.
 - **[deckhouse]** Check kernel version dependencies for sensitive modules. [#2709](https://github.com/deckhouse/deckhouse/pull/2709)
    In case of unmet kernel dependencies, cilium or cilium + istio or cilium + openvpn or cilium + node-local-dns  modules will be locked by special init-contaiters.
 - **[ingress-nginx]** Remove Ingress Nginx controllers of versions below 1.1. [#2927](https://github.com/deckhouse/deckhouse/pull/2927)
 - **[monitoring-kubernetes-control-plane]** The module has been entirely reworked. [#2905](https://github.com/deckhouse/deckhouse/pull/2905)
    The `monitoring-kubernetes-control-plane` module now works only if the `control-plane-manager` module is enabled.
 - **[node-manager]** Add ability to scale node groups from zero size. You can set minPerZone = 0 and maxPerZone > 0 now. [#2762](https://github.com/deckhouse/deckhouse/pull/2762)
 - **[prometheus]** Updated Prometheus-(Self) dashboard. [#2968](https://github.com/deckhouse/deckhouse/pull/2968)
 - **[prometheus]** Added Prometheus benchmark dashboard. [#2967](https://github.com/deckhouse/deckhouse/pull/2967)
 - **[user-authn]** Added `RootCAData` and `InsecureSkipVerify` options for OIDC providers. [#2963](https://github.com/deckhouse/deckhouse/pull/2963)

## Fixes


 - **[candi]** Changed desired kernel version to 5.11.0-46-generic for ubuntu 20.04. [#3043](https://github.com/deckhouse/deckhouse/pull/3043)
 - **[deckhouse]** Removed the `Approved` column from the status columns. [#2958](https://github.com/deckhouse/deckhouse/pull/2958)
 - **[extended-monitoring]** Restart if metrics were last collected more than 15 minutes. [#2988](https://github.com/deckhouse/deckhouse/pull/2988)
 - **[ingress-nginx]** Fix erroneous redirect to nil://example.com. [#2970](https://github.com/deckhouse/deckhouse/pull/2970)
    Ingress-nginx v1.1 pods will be restarted.
 - **[monitoring-applications]** Fix the discovery hook for the `monitoring-applications` module. [#3044](https://github.com/deckhouse/deckhouse/pull/3044)
 - **[monitoring-kubernetes-control-plane]** Port to listen changed to 8008 because it is already used by the ingress-nginx module. [#3019](https://github.com/deckhouse/deckhouse/pull/3019)
 - **[node-local-dns]** Disable `node-local-dns` for cilium installations on nodes with kernel < 5.7 due to problems with `ebpf-socket` and resolved endpoints. [#2995](https://github.com/deckhouse/deckhouse/pull/2995)
    The node-local-dns module stops working for cilium installations on nodes with kernels < 5.7.
 - **[node-manager]** Fix `cluster-autoscaler` configMap generation when a few node groups have the same priority. [#3062](https://github.com/deckhouse/deckhouse/pull/3062)

## Chore


 - **[ceph-csi]** Update csi-attacher and csi-resizer. [#2561](https://github.com/deckhouse/deckhouse/pull/2561)
    csi-plugin pods will be restarted.
 - **[cloud-provider-aws]** Update csi-attacher and csi-resizer. [#2561](https://github.com/deckhouse/deckhouse/pull/2561)
    csi-plugin pods will be restarted.
 - **[cloud-provider-azure]** Update csi-attacher and csi-resizer. [#2561](https://github.com/deckhouse/deckhouse/pull/2561)
    csi-plugin pods will be restarted.
 - **[cloud-provider-gcp]** Update csi-attacher and csi-resizer. [#2561](https://github.com/deckhouse/deckhouse/pull/2561)
    csi-plugin pods will be restarted.
 - **[cloud-provider-openstack]** Update csi-attacher and csi-resizer. [#2561](https://github.com/deckhouse/deckhouse/pull/2561)
    csi-plugin pods will be restarted.
 - **[cloud-provider-vsphere]** Update csi-attacher and csi-resizer. [#2561](https://github.com/deckhouse/deckhouse/pull/2561)
    csi-plugin pods will be restarted.
 - **[cloud-provider-yandex]** Update csi-attacher and csi-resizer. [#2561](https://github.com/deckhouse/deckhouse/pull/2561)
    csi-plugin pods will be restarted.
 - **[linstor]** Update components version. [#2561](https://github.com/deckhouse/deckhouse/pull/2561)
    linstor and csi-plugin pods will be restarted.
 - **[metallb]** Add metallb module usage example [#2833](https://github.com/deckhouse/deckhouse/pull/2833)

