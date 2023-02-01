# Changelog v1.42

## Know before update


 - Clusters with the `kubernetesVersion` parameter set to `Automatic` will be upgraded to Kubernetes `1.23`.
 - If there is the `ClusterConfiguration.proxy` parameter configured, it is highly important to configure the `noProxy`  parameter with your Nodes CIDRs.
 - Multiple Pods, including Prometheus and Ingress Nginx Controller, will be restarted.
 - Prometheus, Trickster, Grafana will be restarted.
 - The following components will be restarted:
    * `ebs-csi-plugin` in the `cloud-provider-aws` module;
    * `azuredisk-csi` in the `cloud-provider-azure` module;
    * `pd-csi-plugin` in the `cloud-provider-gcp` module;
    * `kube-controller-manager` in the `control-plane-manager` module;
    * `grafana` in the `prometheus` module;
    * `ui-proxy` in the `cilium-hubble` module;
    * `vector` in the `log-shipper` module;
    * `cinder-provider-openstack` and `cloud-controller-manager` in the `cloud-provider-openstack` module;
    * `vsphere-csi-plugin` and `vsphere-csi-plugin-legacy` in the `cloud-provider-vsphere` module;
    * `operator`, `pilot` and `proxyv2` in the `istio` module;
    * `grafana-agent` in the `flant-integration` module.

## Features


 - **[admission-policy-engine]** Add CRD `OperationPolicy` for configuring best-practice cluster policies. [#3115](https://github.com/deckhouse/deckhouse/pull/3115)
 - **[candi]** Upgraded patch versions of Kubernetes images: v1.22.17, v1.23.15, and v1.24.9. [#3297](https://github.com/deckhouse/deckhouse/pull/3297)
    "Kubernetes control-plane components will restart, kubelet will restart"
 - **[candi]** Upgraded patch versions of Kubernetes images: v1.22.16, v1.23.14, and v1.24.8. [#3250](https://github.com/deckhouse/deckhouse/pull/3250)
    "Kubernetes control-plane components will restart, kubelet will restart"
 - **[candi]** Migrate to NAT gateway in the Yandex.Cloud Standard layout. [#3235](https://github.com/deckhouse/deckhouse/pull/3235)
    If you have the Standard layout, follow the [migration guide](https://docs.google.com/document/d/1ssFEfX1jL7YiGD0_ZyJc1awofjQRJeRlABFmXk3E3ws) to start using the new ["NAT gateway"](https://cloud.yandex.com/en-ru/docs/vpc/operations/create-nat-gateway) feature.
 - **[candi]** Added the `proxy` parameter to the `ClusterConfiguration` resource.
    Removed the `packagesProxy` parameter from the `ClusterConfiguration` resource.
    The `modules.proxy` global parameter is deprecated.
    Added migration to convert `ClusterConfiguration.packagesProxy` and the global `modules.proxy` parameters to the 
    `ClusterConfiguration.proxy` parameter (global `modules.proxy` takes precedence). [#3185](https://github.com/deckhouse/deckhouse/pull/3185)
 - **[candi]** Show bash debug output for failed sources steps when bootstrap cluster. [#3122](https://github.com/deckhouse/deckhouse/pull/3122)
 - **[candi]** Kernel version management removed. [#3076](https://github.com/deckhouse/deckhouse/pull/3076)
 - **[candi]** Switch base images from Debian to Ubuntu & update BASE_UBUNTU to Jammy. [#2885](https://github.com/deckhouse/deckhouse/pull/2885)
    The following components will be restarted:
    * `ebs-csi-plugin` in the `cloud-provider-aws` module;
    * `azuredisk-csi` in the `cloud-provider-azure` module;
    * `pd-csi-plugin` in the `cloud-provider-gcp` module;
    * `kube-controller-manager` in the `control-plane-manager` module;
    * `grafana` in the `prometheus` module;
    * `ui-proxy` in the `cilium-hubble` module;
    * `vector` in the `log-shipper` module;
    * `cinder-provider-openstack` and `cloud-controller-manager` in the `cloud-provider-openstack` module;
    * `vsphere-csi-plugin` and `vsphere-csi-plugin-legacy` in the `cloud-provider-vsphere` module;
    * `operator`, `pilot` and `proxyv2` in the `istio` module;
    * `grafana-agent` in the `flant-integration` module.
 - **[common]** Generate self-signed CA for `kube-rbac-proxy`. [#3288](https://github.com/deckhouse/deckhouse/pull/3288)
    Multiple Pods, including Prometheus and Ingress Nginx Controller, will be restarted.
 - **[control-plane-manager]** Added support for Kubernetes 1.25. [#3247](https://github.com/deckhouse/deckhouse/pull/3247)
 - **[deckhouse]** Added releaseChannel label to DeckhouseIsUpdating alert. [#3264](https://github.com/deckhouse/deckhouse/pull/3264)
 - **[delivery]** Added the new 'delivery' module based on ArgoCD. [#707](https://github.com/deckhouse/deckhouse/pull/707)
 - **[go_lib]** Issue a new self-signed certificate if CA is not found. [#3349](https://github.com/deckhouse/deckhouse/pull/3349)
 - **[istio]** Automatic istio dataplane upgrade for `Deployment`, `Daemonset` and `StatefulSet` with a special label. [#3061](https://github.com/deckhouse/deckhouse/pull/3061)
 - **[istio]** Ability to create Ingress istio gateway controller. [#2898](https://github.com/deckhouse/deckhouse/pull/2898)
 - **[log-shipper]** Add Splunk destination. [#3117](https://github.com/deckhouse/deckhouse/pull/3117)
 - **[node-manager]** Check the `bashible` service before bootstrap. [#3140](https://github.com/deckhouse/deckhouse/pull/3140)
 - **[pod-reloader]** Redeploy workload after ConfigMap/Secret recreation. [#3096](https://github.com/deckhouse/deckhouse/pull/3096)
 - **[prometheus]** Use prometheus service account token for authentication. [#3217](https://github.com/deckhouse/deckhouse/pull/3217)
    Prometheus, Trickster, Grafana will be restarted.
 - **[user-authn]** Add claim mappings for OIDC providers. [#3198](https://github.com/deckhouse/deckhouse/pull/3198)

## Fixes


 - **[admission-policy-engine]** Add secret with credentials for a registry [#3310](https://github.com/deckhouse/deckhouse/pull/3310)
 - **[candi]** Force `libseccomp2` installation while containerd install/update (due to issue https://github.com/containerd/containerd/discussions/6577). [#3504](https://github.com/deckhouse/deckhouse/pull/3504)
 - **[candi]** Fail node bootstrap if the node has an XFS partition with ftype=0 parameter. [#3103](https://github.com/deckhouse/deckhouse/pull/3103)
 - **[ceph-csi]** Restoring the previous secret name with ceph cluster credentials. [#3387](https://github.com/deckhouse/deckhouse/pull/3387)
 - **[ceph-csi]** Delete storage classes after changing immutable fields. [#3380](https://github.com/deckhouse/deckhouse/pull/3380)
 - **[ceph-csi]** Allow non-admin ceph account for cephfs. [#3212](https://github.com/deckhouse/deckhouse/pull/3212)
 - **[cloud-provider-openstack]** Fix ordering static nodes without security groups. [#3182](https://github.com/deckhouse/deckhouse/pull/3182)
 - **[deckhouse]** Fixed unrendered backquotes in the DeckhouseRelease resource. [#3367](https://github.com/deckhouse/deckhouse/pull/3367)
 - **[docs]** Clarify usage of the `noProxy` parameter. [#3526](https://github.com/deckhouse/deckhouse/pull/3526)
    If there is the `ClusterConfiguration.proxy` parameter configured, it is highly important to configure the `noProxy`  parameter with your Nodes CIDRs.
 - **[global-hooks]** Fixes in cluster configuration migration process and proxy template for EKS cluster installation. [#3381](https://github.com/deckhouse/deckhouse/pull/3381)
 - **[ingress-nginx]** Fix client certificate update. [#3368](https://github.com/deckhouse/deckhouse/pull/3368)
 - **[ingress-nginx]** Add `minReadySeconds` for `LoadBalancer` inlet controllers. This will give some time for the Load Balancer to rebuild the endpoints. [#3121](https://github.com/deckhouse/deckhouse/pull/3121)
 - **[istio]** Switching default iptables in proxyv2 to iptables-legacy (after switching from Debian to Ubuntu). [#3579](https://github.com/deckhouse/deckhouse/pull/3579)
 - **[istio]** Replace CA for the Ingress validation of api-proxy, fix kiali `ClusterRole`. [#3395](https://github.com/deckhouse/deckhouse/pull/3395)
 - **[log-shipper]** Fix TLS certificates error for Kafka destination. [#3215](https://github.com/deckhouse/deckhouse/pull/3215)
 - **[node-local-dns]** Remove the module from the `Managed` bundle. [#3309](https://github.com/deckhouse/deckhouse/pull/3309)
 - **[node-manager]** Increase early-oom PSI threshold to 30 (from 5). [#3427](https://github.com/deckhouse/deckhouse/pull/3427)
 - **[node-manager]** Show errors on scale-from-zero capacity planning. [#3316](https://github.com/deckhouse/deckhouse/pull/3316)
 - **[prometheus]** Fix Alertmanager CA file (caused Unauthorized error). [#3723](https://github.com/deckhouse/deckhouse/pull/3723)
 - **[prometheus]** Fixed token-based authentication between Trickster and Prometheus. [#3519](https://github.com/deckhouse/deckhouse/pull/3519)
 - **[prometheus]** Fix remoteWrite tlsConfig render. [#3510](https://github.com/deckhouse/deckhouse/pull/3510)
 - **[prometheus]** Set up `maxSamples` of query for the Main and Longterm Prometheus objects from `50000000` to `100000000`. [#3124](https://github.com/deckhouse/deckhouse/pull/3124)
    the `prometheus` module will be restarted.
 - **[registrypackages]** Install flannel `1.1.2` binary to CentOS for `kubernetes-cni` newer than `0.9.1` due to missing flannel binary. [#3503](https://github.com/deckhouse/deckhouse/pull/3503)
 - **[registrypackages]** Allow downgrading RPMs in registrypackages for CentOS. [#3353](https://github.com/deckhouse/deckhouse/pull/3353)
 - **[user-authn]** OIDC insecure userInfo endpoint. [#3501](https://github.com/deckhouse/deckhouse/pull/3501)
 - **[user-authn]** Fix insecure OIDC Ca patch. [#3439](https://github.com/deckhouse/deckhouse/pull/3439)
 - **[user-authn]** Fix crowd proxy certificate generation. [#3355](https://github.com/deckhouse/deckhouse/pull/3355)
 - **[user-authn]** kubeconfig generation doc clarifications (public and non-public CA for published API). [#3237](https://github.com/deckhouse/deckhouse/pull/3237)
 - **[user-authn]** Fixed the `generate_crowd_basic_auth_proxy_cert.go` hook. [#3223](https://github.com/deckhouse/deckhouse/pull/3223)
 - **[user-authn]** Use a self-signed certificate for Dex when accessing from inside the cluster. [#3218](https://github.com/deckhouse/deckhouse/pull/3218)
 - **[user-authz]** Create patch update rights for the `user-authz:admin` clusterrole. [#3211](https://github.com/deckhouse/deckhouse/pull/3211)
 - **[vertical-pod-autoscaler]** Setting `admissionReviewVersions` to `v1` in mutating webhook. [#3397](https://github.com/deckhouse/deckhouse/pull/3397)

## Chore


 - **[candi]** Temporary disable `seccomp` for `kube-controller-manager`. [#3426](https://github.com/deckhouse/deckhouse/pull/3426)
 - **[candi]** Support for the Standard layout in Yandex Cloud. [#3411](https://github.com/deckhouse/deckhouse/pull/3411)
 - **[candi]** Upgraded patch versions of Kubernetes images: v1.25.5. [#3376](https://github.com/deckhouse/deckhouse/pull/3376)
    "Kubernetes control-plane components will restart, kubelet will restart"
 - **[ceph-csi]** Added script for semi-automatic migration of volumes from in-tree RBD driver to Ceph CSI, as well as instruction and alert. [#2973](https://github.com/deckhouse/deckhouse/pull/2973)
 - **[cert-manager]** Bump `cert-manager` version to the `1.10.1`. [#3133](https://github.com/deckhouse/deckhouse/pull/3133)
 - **[deckhouse]** Changed the default Kubernetes version to `1.23`. [#3292](https://github.com/deckhouse/deckhouse/pull/3292)
    Clusters with the `kubernetesVersion` parameter set to `Automatic` will be upgraded to Kubernetes `1.23`.
 - **[deckhouse-controller]** Update Kubernetes libs. [#3285](https://github.com/deckhouse/deckhouse/pull/3285)
 - **[deckhouse-controller]** Update the Go version to `1.19`. [#3269](https://github.com/deckhouse/deckhouse/pull/3269)
 - **[deckhouse-controller]** Use main branch for shell-operator. [#3228](https://github.com/deckhouse/deckhouse/pull/3228)
 - **[deckhouse-controller]** Remove `libjq-go` from the build. [#3098](https://github.com/deckhouse/deckhouse/pull/3098)
 - **[dhctl]** Update Kubernetes libs. [#3285](https://github.com/deckhouse/deckhouse/pull/3285)
 - **[flant-integration]** Filter master nodes based on `node.deckhouse.io/group` in pricing and consider both `node-role.kubernetes.io/master` and `node-role.kubernetes.io/control-plane` taints for dedicated master. [#3077](https://github.com/deckhouse/deckhouse/pull/3077)
    The `pricing` Pods will restart in the `d8-flant-integration` namespace.
 - **[global-hooks]** Remove x bit from *.go files in global-hooks. [#3258](https://github.com/deckhouse/deckhouse/pull/3258)
 - **[monitoring-deckhouse]** Add an alert about deprecated OS versions. [#3405](https://github.com/deckhouse/deckhouse/pull/3405)

