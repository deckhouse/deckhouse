# Changelog v1.53

## Features


 - **[candi]** Remove AlterOS support. [#6212](https://github.com/deckhouse/deckhouse/pull/6212)
 - **[candi]** Add Kubernetes 1.28 support. [#5894](https://github.com/deckhouse/deckhouse/pull/5894)
    All control plane components will be restarted.
 - **[candi]** Move rendering of the boostrap scripts to `bashible-apiserver` to reduce size of the `cloud-init` data. [#4907](https://github.com/deckhouse/deckhouse/pull/4907)
 - **[cert-manager]** Use distroless images in the module. [#6084](https://github.com/deckhouse/deckhouse/pull/6084)
 - **[cloud-provider-aws]** `cloud-data-discoverer` uses distroless image. [#6052](https://github.com/deckhouse/deckhouse/pull/6052)
 - **[cloud-provider-aws]** `cloud-controller-manager` uses distroless image. [#5985](https://github.com/deckhouse/deckhouse/pull/5985)
 - **[cloud-provider-azure]** `cloud-data-discoverer` uses distroless image. [#6052](https://github.com/deckhouse/deckhouse/pull/6052)
 - **[cloud-provider-azure]** `cloud-controller-manager` uses distroless image. [#5985](https://github.com/deckhouse/deckhouse/pull/5985)
 - **[cloud-provider-gcp]** `cloud-data-discoverer` uses distroless image. [#6052](https://github.com/deckhouse/deckhouse/pull/6052)
 - **[cloud-provider-gcp]** `cloud-controller-manager` uses distroless image. [#5985](https://github.com/deckhouse/deckhouse/pull/5985)
 - **[cloud-provider-openstack]** `cloud-data-discoverer` uses distroless image. [#6052](https://github.com/deckhouse/deckhouse/pull/6052)
 - **[cloud-provider-openstack]** `cloud-controller-manager` uses distroless image. [#5985](https://github.com/deckhouse/deckhouse/pull/5985)
 - **[cloud-provider-vsphere]** `cloud-controller-manager` uses distroless image. [#5985](https://github.com/deckhouse/deckhouse/pull/5985)
 - **[cloud-provider-yandex]** `cloud-controller-manager` uses distroless image. [#5985](https://github.com/deckhouse/deckhouse/pull/5985)
 - **[descheduler]** Use distroless images in the module. [#6105](https://github.com/deckhouse/deckhouse/pull/6105)
 - **[dhctl]** Allow installing Deckhouse from tag. Refactor preflight checks code. [#5955](https://github.com/deckhouse/deckhouse/pull/5955)
 - **[dhctl]** Dhctl will check if container registry can be reached with provided HTTP\HTTPS proxy [#5926](https://github.com/deckhouse/deckhouse/pull/5926)
 - **[ingress-nginx]** `proxy-failover` uses distroless image. [#6028](https://github.com/deckhouse/deckhouse/pull/6028)
 - **[ingress-nginx]** Kruise controller image uses distroless image. [#5993](https://github.com/deckhouse/deckhouse/pull/5993)
    `kruise-controller-manager` pod will restart.
 - **[log-shipper]** Add Kafka SASL auth settings to configure. [#6171](https://github.com/deckhouse/deckhouse/pull/6171)
 - **[metallb]** Use distroless images in the module. [#6125](https://github.com/deckhouse/deckhouse/pull/6125)
    The metallb pods will restart.
 - **[node-manager]** Add Cluster API Provider Static. [#5432](https://github.com/deckhouse/deckhouse/pull/5432)
 - **[operator-trivy]** Add periodic updates of report-updater's dictionary. [#5973](https://github.com/deckhouse/deckhouse/pull/5973)
    `report-updater` will restart.
 - **[pod-reloader]** Use distroless images in the module. [#6126](https://github.com/deckhouse/deckhouse/pull/6126)
    `pod-reloader` pod will restart.
 - **[user-authn]** Allow setting user password in Base64. [#6030](https://github.com/deckhouse/deckhouse/pull/6030)
 - **[vertical-pod-autoscaler]** `admission-controller`, `recommender` and `updater` use distroless images. [#6099](https://github.com/deckhouse/deckhouse/pull/6099)
    `admission-controller`, `recommender` and `updater` pods will restart.

## Fixes


 - **[candi]** Send bootstrap logs to console in case of manual bootstrap. [#6332](https://github.com/deckhouse/deckhouse/pull/6332)
 - **[candi]** Improve bashible steps running order. [#6307](https://github.com/deckhouse/deckhouse/pull/6307)
 - **[candi]** Send bootstrap logs from cluster-api static instances. [#6252](https://github.com/deckhouse/deckhouse/pull/6252)
 - **[candi]** Fix CAPI kubeconfig hook, which cannot work on fresh installations. [#6252](https://github.com/deckhouse/deckhouse/pull/6252)
 - **[candi]** Removed `shortNames` from CAPI CRDs. [#6252](https://github.com/deckhouse/deckhouse/pull/6252)
 - **[candi]** Add ALT Linux 10.0, 10.2 support. [#6215](https://github.com/deckhouse/deckhouse/pull/6215)
 - **[candi]** Revert curl version pinning for Red OS. [#6210](https://github.com/deckhouse/deckhouse/pull/6210)
 - **[candi]** Fix working of bootstrap cloud-networks setup scripts. [#6193](https://github.com/deckhouse/deckhouse/pull/6193)
 - **[candi]** Allow underscore in `httpProxy` and `httpsProxy` settings. [#6169](https://github.com/deckhouse/deckhouse/pull/6169)
 - **[cloud-provider-vsphere]** Fix slugification datastore names containing hyphens. [#6286](https://github.com/deckhouse/deckhouse/pull/6286)
 - **[common]** Fix build of `csi-external-*` images. [#6378](https://github.com/deckhouse/deckhouse/pull/6378)
    `csi-controller` pod will restart.
 - **[deckhouse]** Canary release disabled for manual update mode [#6229](https://github.com/deckhouse/deckhouse/pull/6229)
 - **[deckhouse]** Fix bash completion. [#6225](https://github.com/deckhouse/deckhouse/pull/6225)
 - **[deckhouse]** Restrict the webhook to validate only Deckhouse ConfigMaps. [#5997](https://github.com/deckhouse/deckhouse/pull/5997)
 - **[external-module-manager]** Change the image export logic. [#6123](https://github.com/deckhouse/deckhouse/pull/6123)
 - **[ingress-nginx]** Fix Ingress controller panic when an endpointslice doesn't have the `.conditions` field. [#6111](https://github.com/deckhouse/deckhouse/pull/6111)
    Ingress controller will restart.
 - **[linstor]** Changed the method of loading DRBD kernel module on the nodes with active LINSTOR satellites. [#6278](https://github.com/deckhouse/deckhouse/pull/6278)
 - **[node-manager]** Do not show the 'Approve with reboot' button for nodes in the Deckhouse UI when the `RollingUpdate` mode is selected. [#5440](https://github.com/deckhouse/deckhouse/pull/5440)
 - **[pod-reloader]** Run pod-reloader from deckhouse user. [#6258](https://github.com/deckhouse/deckhouse/pull/6258)
 - **[prometheus]** Fix settings source for external auth. [#6103](https://github.com/deckhouse/deckhouse/pull/6103)
 - **[runtime-audit-engine]** Set Notice priority for rules requiring notification of security officer [#6232](https://github.com/deckhouse/deckhouse/pull/6232)
 - **[runtime-audit-engine]** Fix events unbuffered output mode. [#6124](https://github.com/deckhouse/deckhouse/pull/6124)
 - **[user-authn]** Return 401 instead of 200 if a password authentication attempt failed. [#6045](https://github.com/deckhouse/deckhouse/pull/6045)
 - **[user-authz]** `webhook` no longer crashes if called without data. [#6066](https://github.com/deckhouse/deckhouse/pull/6066)

## Chore


 - **[candi]** Disable the in-tree RBD plugin for Kubernetes >= 1.24. [#6134](https://github.com/deckhouse/deckhouse/pull/6134)
    Control plane pods will restart.
 - **[user-authn]** Make the `userID` parameter of the User resource deprecated. [#6106](https://github.com/deckhouse/deckhouse/pull/6106)
 - **[user-authz]** Add missing multitenancy constraint for cluster authorization rules. Fixes user validation webhook. [#6256](https://github.com/deckhouse/deckhouse/pull/6256)

