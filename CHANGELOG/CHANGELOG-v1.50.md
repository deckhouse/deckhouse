e2e test


# Changelog v1.50

## Know before update


 - If the Ingress controller version is not pinned in the [IngressNginxController](https://deckhouse.io/documentation/v1/modules/402-ingress-nginx/cr.html#ingressnginxcontroller) CR, Ingress controller will be updated to version `1.6` and restart.
 - Ingress controller will restart.
 - Kubernetes will be updated to `1.25`, if the [kubernetesVersion](https://deckhouse.io/documentation/v1/installing/configuration.html#clusterconfiguration-kubernetesversion) parameter of the `ClusterConfiguration` resource is set to `Automatic`.

## Features


 - **[candi]** Add a bashible step that tries to detect Docker CRI and exits with an error if it does. [#5468](https://github.com/deckhouse/deckhouse/pull/5468)
 - **[candi]** Bump default Kubernetes version to `1.25`. [#5467](https://github.com/deckhouse/deckhouse/pull/5467)
    Kubernetes will be updated to `1.25`, if the [kubernetesVersion](https://deckhouse.io/documentation/v1/installing/configuration.html#clusterconfiguration-kubernetesversion) parameter of the `ClusterConfiguration` resource is set to `Automatic`.
 - **[extended-monitoring]** Images are based on a distroless image. [#5358](https://github.com/deckhouse/deckhouse/pull/5358)
 - **[ingress-nginx]** Bump default Ingress controller version to `1.6`. [#5466](https://github.com/deckhouse/deckhouse/pull/5466)
    If the Ingress controller version is not pinned in the [IngressNginxController](https://deckhouse.io/documentation/v1/modules/402-ingress-nginx/cr.html#ingressnginxcontroller) CR, Ingress controller will be updated to version `1.6` and restart.
 - **[linstor]** Move DRBD module build from `kernel-module-injector` to `bashible`. [#5230](https://github.com/deckhouse/deckhouse/pull/5230)
    linstor satellite on nodes will restart.
 - **[log-shipper]** Add `tenantID` option for Loki (may be required for Grafana Cloud). [#5401](https://github.com/deckhouse/deckhouse/pull/5401)
 - **[monitoring-applications]** Add Grafana dashboard for Loki. [#5383](https://github.com/deckhouse/deckhouse/pull/5383)
 - **[monitoring-kubernetes]** Images are based on a distroless image. [#5343](https://github.com/deckhouse/deckhouse/pull/5343)
 - **[node-manager]** Alert fires if Node has no config checksum annotation during a `NodeGroup` update. [#5443](https://github.com/deckhouse/deckhouse/pull/5443)
 - **[node-manager]** Add a short name for `NodeGroupConfiguration` (`ngc`). [#5367](https://github.com/deckhouse/deckhouse/pull/5367)
 - **[node-manager]** Make the versions of autoscaler correspond to the versions of Kubernetes. [#5158](https://github.com/deckhouse/deckhouse/pull/5158)
 - **[virtualization]** Allow specifying priority for virtual machines via the [PriorityClassName](https://deckhouse.io/documentation/latest/modules/490-virtualization/cr.html#virtualmachine-v1alpha1-spec-priorityclassname) parameter. [#5444](https://github.com/deckhouse/deckhouse/pull/5444)

## Fixes


 - **[candi]** Set firmware explicitly for `vsphere_virtual_machine`. [#5457](https://github.com/deckhouse/deckhouse/pull/5457)
 - **[candi]** Remove usage of temp files when creating a `NodeUser`. [#5337](https://github.com/deckhouse/deckhouse/pull/5337)
 - **[deckhouse-config]** Deckhouse-config-webhook successfully starts without the `external-module-manager` module. [#5392](https://github.com/deckhouse/deckhouse/pull/5392)
 - **[ingress-nginx]** Fix `nginx_ingress_controller_ssl_expire_time_seconds`. [#5442](https://github.com/deckhouse/deckhouse/pull/5442)
    Ingress controller will restart.
 - **[keepalived]** Add info about [how to do manual IP switching](https://deckhouse.io/documentation/latest/modules/450-keepalived/faq.html#how-to-manually-switch-keepalived). [#5335](https://github.com/deckhouse/deckhouse/pull/5335)
 - **[loki]** Mount `/tmp` on `emptyDir` to fix retention. [#5400](https://github.com/deckhouse/deckhouse/pull/5400)
 - **[operator-trivy]** Remove `upmeter` probes from trivy scanning. [#5364](https://github.com/deckhouse/deckhouse/pull/5364)
 - **[prometheus]** Fix alert expression when a `longterm-prometheus` fails to scrape the `main-prometheus` for whatever reason. [#5345](https://github.com/deckhouse/deckhouse/pull/5345)
 - **[user-authn]** Use global discovered `publishAPI` cert by default for generated kubeconfigs. [#5488](https://github.com/deckhouse/deckhouse/pull/5488)

## Chore


 - **[deckhouse-controller]** Added migration to remove deprecated ConfigMap `d8-system/deckhouse`. [#4869](https://github.com/deckhouse/deckhouse/pull/4869)

