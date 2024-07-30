# Changelog v1.45

## Know before update


 - All system components will restart.
 - Kubernetes `1.21` is no longer supported.
 - The `operator-trivy` module will no longer be available in Deckhouse CE.
 - There are new requirements for the [deckhouse IAM user](https://deckhouse.io/documentation/v1.45/modules/030-cloud-provider-aws/environment.html#json-policy) in AWS. It is necessary to allow the following actions: `ec2:DescribeInstanceTypes`, `ec2:DescribeSecurityGroupRules`.

## Features


 - **[admission-policy-engine]** Add gatekeeper [mutations](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation) support. [#3987](https://github.com/deckhouse/deckhouse/pull/3987)
 - **[candi]** Upgraded patch versions of Kubernetes images: `v1.24.13`, `v1.25.9`, and `v1.26.4`. [#4414](https://github.com/deckhouse/deckhouse/pull/4414)
    "Kubernetes control-plane components will restart, kubelet will restart"
 - **[candi]** Upgraded patch versions of Kubernetes images: `v1.24.12`, `v1.25.8`, `v1.26.3`. [#4172](https://github.com/deckhouse/deckhouse/pull/4172)
    Kubernetes control-plane components will restart, kubelet will restart.
 - **[candi]** Upgraded patch versions of Kubernetes images: `v1.23.17`, `v1.24.11`, `v1.25.7`, `v1.26.2` [#4012](https://github.com/deckhouse/deckhouse/pull/4012)
    "Kubernetes control-plane components will restart, kubelet will restart"
 - **[candi]** Add support for Kubernetes `1.26`. Remove support for Kubernetes `1.21`. [#3760](https://github.com/deckhouse/deckhouse/pull/3760)
    Kubernetes `1.21` is no longer supported.
 - **[candi]** Add ALT Linux support. [#3555](https://github.com/deckhouse/deckhouse/pull/3555)
 - **[candi]** Switch from pulling container images by tag to pulling by sha256 checksum. [#3318](https://github.com/deckhouse/deckhouse/pull/3318)
    All system components will restart.
 - **[cloud-provider-vsphere]** Add zones list to cloud discovery data. [#4136](https://github.com/deckhouse/deckhouse/pull/4136)
 - **[cni-cilium]** Bump `cilium` and `virt-cilium` to `v1.12.8`. [#4284](https://github.com/deckhouse/deckhouse/pull/4284)
    All cilium-agent Pods will be restarted.
 - **[containerized-data-importer]** CDI `v1.56.0`. [#3956](https://github.com/deckhouse/deckhouse/pull/3956)
 - **[deckhouse-controller]** Add commands to enable/disable modules without YAML editing. [#4245](https://github.com/deckhouse/deckhouse/pull/4245)
 - **[descheduler]** Descheduler module is configured via CRs now. Configuration from Deckhouse CM will get migrated to the "default" Descheduler CR. [#1585](https://github.com/deckhouse/deckhouse/pull/1585)
 - **[ingress-nginx]** Use `AdvancedDaemonSet` controller for smooth ingress-controller rollout. [#4124](https://github.com/deckhouse/deckhouse/pull/4124)
    Ingress-controller Pods will restart.
 - **[ingress-nginx]** Added Nginx Ingress controller `v1.6.4`.
    - **nginx-controller**:
      * Upgrade NGINX to `1.21.6`.
      * Ingress-nginx now is using `Endpointslices` instead of `Endpoints`. 
      * Update to Prometheus metric names; more information [available here]( https://github.com/kubernetes/ingress-nginx/pull/8728).
      * Deprecated Kubernetes versions `1.20`-`1.21`. Added support for `1.25`. Currently supported versions are `1.22`, `1.23`, `1.24`, `1.25`.
      * This release removes the `root` and `alias` directives in NGINX, which can avoid some potential security attacks.
      * This release also brings a special new feature of deep inspection into objects. The inspection is a walk-through of all the specs, checking for possible attempts to escape configs. Currently, such an inspection only occurs for `networking.Ingress`.
    - **nginx**:
      * Feature: the "proxy_half_close" directive in the stream module.
      * Feature: the "ssl_alpn" directive in the stream module.
      * Feature: the "mp4_start_key_frame" directive in the ngx_http_mp4_module.
      * Bugfix: requests might hang when using HTTP/2 and the "aio_write" directive.
      * Bugfix: the security level, which is available in OpenSSL 1.1.0 or newer, did not affect the loading of the server certificates when set with "@SECLEVEL=N" in the "ssl_ciphers" directive.
      * Security: 1-byte memory overwrite might occur during DNS server response processing if the "resolver" directive was used, allowing an attacker who is able to forge UDP packets from the DNS server to cause worker process crash or, potentially, arbitrary code execution (CVE-2021-23017).
      * Feature: variables support in the "proxy_ssl_certificate", "proxy_ssl_certificate_key" "grpc_ssl_certificate", "grpc_ssl_certificate_key", "uwsgi_ssl_certificate", and "uwsgi_ssl_certificate_key" directives.
      * Feature: the "max_errors" directive in the mail proxy module.
      * Feature: the "fastopen" parameter of the "listen" directive in the Stream module. [#3923](https://github.com/deckhouse/deckhouse/pull/3923)
 - **[linstor]** Update LINSTOR to `v1.21.0` and related components. [#4098](https://github.com/deckhouse/deckhouse/pull/4098)
 - **[log-shipper]** Multiline parser `Custom` type with user-provided regex. [#4247](https://github.com/deckhouse/deckhouse/pull/4247)
 - **[log-shipper]** Add status codes and errors to the dashboard. [#4117](https://github.com/deckhouse/deckhouse/pull/4117)
 - **[log-shipper]** Add the `keyField` and `exclude` parameters to the `ClusterLogDestination` resource for configuring rate limiting. [#4099](https://github.com/deckhouse/deckhouse/pull/4099)
 - **[log-shipper]** Add buffer settings for vector config. [#4095](https://github.com/deckhouse/deckhouse/pull/4095)
 - **[loki]** The new module. Based on the Grafana Loki project. [#3735](https://github.com/deckhouse/deckhouse/pull/3735)
 - **[node-manager]** Show used `NodeGroups` in the `InstanceClass` status field. [#4028](https://github.com/deckhouse/deckhouse/pull/4028)
 - **[node-manager]** Add `NodeGroup` conditions. [#3990](https://github.com/deckhouse/deckhouse/pull/3990)
 - **[prometheus]** Accelerate `grafana-dashboard-provisioner` hook. [#3691](https://github.com/deckhouse/deckhouse/pull/3691)
 - **[user-authn]** Custom login screen design. [#4305](https://github.com/deckhouse/deckhouse/pull/4305)
 - **[virtualization]** Enable snapshot feature in the `virtualization` module. [#4413](https://github.com/deckhouse/deckhouse/pull/4413)
 - **[virtualization]** KubeVirt `v0.59.0`. [#3956](https://github.com/deckhouse/deckhouse/pull/3956)

## Fixes


 - **[admission-policy-engine]** Fix `requiredLabels` OperationPolicy. [#4264](https://github.com/deckhouse/deckhouse/pull/4264)
    `requiredLabel` unmarshalling leads to an error.
 - **[candi]** Fix the error in the `detect_bundle.sh` script. [#4669](https://github.com/deckhouse/deckhouse/pull/4669)
 - **[candi]** Bump gophercloud/utils version. [#4472](https://github.com/deckhouse/deckhouse/pull/4472)
 - **[candi]** Fix SELinux permissions Bashible step in clusters without Cilium. [#4418](https://github.com/deckhouse/deckhouse/pull/4418)
 - **[candi]** Remove the `node-role.kubernetes.io/master` taint from the first control-plane node during bootstrap. [#4159](https://github.com/deckhouse/deckhouse/pull/4159)
 - **[cloud-provider-aws]** The terraform provider was bumped to `4.50.0`, therefore there are new requirements for deckhouse IAM user. It is necessary to allow the following actions: `ec2:DescribeInstanceTypes`, `ec2:DescribeSecurityGroupRules`. [#4256](https://github.com/deckhouse/deckhouse/pull/4256)
    There are new requirements for the [deckhouse IAM user](https://deckhouse.io/documentation/v1.45/modules/030-cloud-provider-aws/environment.html#json-policy) in AWS. It is necessary to allow the following actions: `ec2:DescribeInstanceTypes`, `ec2:DescribeSecurityGroupRules`.
 - **[cloud-provider-gcp]** Disabled Node IPAM in GCP CCM that conflicted with kube-controller-manager's IPAM controller. [#4110](https://github.com/deckhouse/deckhouse/pull/4110)
 - **[cloud-provider-yandex]** Set `network_acceleration_type` to software accelerated, update netfilter parameters for new Yandex nat instances. [#4196](https://github.com/deckhouse/deckhouse/pull/4196)
 - **[deckhouse]** Hours and minutes can be used simultaneously for the `minimalNotificationTime` field in ModuleConfig CR. [#4200](https://github.com/deckhouse/deckhouse/pull/4200)
 - **[descheduler]** Create a default Descheduler instance. [#4606](https://github.com/deckhouse/deckhouse/pull/4606)
 - **[descheduler]** Restored descheduler migration hook. [#4524](https://github.com/deckhouse/deckhouse/pull/4524)
 - **[dhctl]** Fix `kube-proxy` does not restart. [#4274](https://github.com/deckhouse/deckhouse/pull/4274)
 - **[extended-monitoring]** Fix RBAC rules for `image-availability-exporter`. [#4346](https://github.com/deckhouse/deckhouse/pull/4346)
 - **[extended-monitoring]** Fix `image-availability-exporter` for Kubernetes 1.25+. [#4331](https://github.com/deckhouse/deckhouse/pull/4331)
 - **[external-module-manager]** Fix chmod permissions for external modules. [#4388](https://github.com/deckhouse/deckhouse/pull/4388)
 - **[global-hooks]** Deploy PDB as a normal helm resource, not a helm hook. [#4016](https://github.com/deckhouse/deckhouse/pull/4016)
 - **[helm]** Fix deprecated k8s resources metrics. [#4751](https://github.com/deckhouse/deckhouse/pull/4751)
 - **[ingress-nginx]** Add protection for ingress-nginx-controller daemonset migration. [#4734](https://github.com/deckhouse/deckhouse/pull/4734)
 - **[ingress-nginx]** Add metrics and alerts for Nginx Ingress DaemonSets created by Kruise controller manager. [#4698](https://github.com/deckhouse/deckhouse/pull/4698)
 - **[ingress-nginx]** Set `imagePullSecrets` for `kruise-controller`. [#4369](https://github.com/deckhouse/deckhouse/pull/4369)
 - **[ingress-nginx]** Improve controller migration hook. [#4363](https://github.com/deckhouse/deckhouse/pull/4363)
 - **[ingress-nginx]** Fix RBAC rules for `kruise-controller`. [#4353](https://github.com/deckhouse/deckhouse/pull/4353)
 - **[istio]** Add registry secret for `d8-ingress-istio` namespace. [#4244](https://github.com/deckhouse/deckhouse/pull/4244)
 - **[linstor]** Fix multiple requisites in volume placement request. [#4515](https://github.com/deckhouse/deckhouse/pull/4515)
 - **[linstor]** Update `linstor-scheduler-admission` to fix admission review. [#4343](https://github.com/deckhouse/deckhouse/pull/4343)
 - **[log-shipper]** Add multiline custom parser for `PodLoggingConfig` and add validation for multiline custom parser when `startsWhen` and `endsWhen` params  are both provided [#4307](https://github.com/deckhouse/deckhouse/pull/4307)
    `log-shipper` Pods will restart.
 - **[log-shipper]** Make vector retrying request on startup with backoff. [#4270](https://github.com/deckhouse/deckhouse/pull/4270)
 - **[log-shipper]** Fix using hyphens in the `extraLabels` keys field of the `ClusterLogDestination` spec. [#4112](https://github.com/deckhouse/deckhouse/pull/4112)
 - **[loki]** Fix `ClusterLogDestination` Loki endpoint. [#4408](https://github.com/deckhouse/deckhouse/pull/4408)
 - **[node-manager]** Fix the Ready condition for node groups. [#4582](https://github.com/deckhouse/deckhouse/pull/4582)
 - **[node-manager]** Revert removing `early-oom` (the [#3821](https://github.com/deckhouse/deckhouse/pull/3821) PR). [#4376](https://github.com/deckhouse/deckhouse/pull/4376)
 - **[node-manager]** Restrict changing `nodeType` for NodeGroups and remove stale status fields from static NodeGroups. [#4257](https://github.com/deckhouse/deckhouse/pull/4257)
 - **[node-manager]** Fix `WaitingForDisruptiveApproval` status calculating. [#4250](https://github.com/deckhouse/deckhouse/pull/4250)
 - **[node-manager]** Prevent changing bashible checksum if scale/downscale NodeGroup. [#4243](https://github.com/deckhouse/deckhouse/pull/4243)
 - **[node-manager]** Hours and minutes can be used simultaneously in the `spec.chaos.period` field of the NodeGroup CR. [#4200](https://github.com/deckhouse/deckhouse/pull/4200)
 - **[node-manager]** (Reverted in 1.45.3!) Removed early-oom. Added kubelet memory reservation option. [#3821](https://github.com/deckhouse/deckhouse/pull/3821)
 - **[operator-trivy]** Fix operator ConfigMap to properly use digests. [#4449](https://github.com/deckhouse/deckhouse/pull/4449)
 - **[prometheus]** Handle error on malformed Grafana dashboard and skip it instead of failing. [#4693](https://github.com/deckhouse/deckhouse/pull/4693)
 - **[user-authn]** Fix the job image path. [#4385](https://github.com/deckhouse/deckhouse/pull/4385)
 - **[user-authn]** Hours and minutes can be used simultaneously in the `spec.tls` field of the User CR. [#4200](https://github.com/deckhouse/deckhouse/pull/4200)
 - **[user-authn]** The `discover_dex_ca` hook subscribes secret according to the used mode. [#3842](https://github.com/deckhouse/deckhouse/pull/3842)
 - **[user-authz]** Fixed cluster_authorization_rule webhook so that it doesn't crash on fields with whitespaces anymore [#4419](https://github.com/deckhouse/deckhouse/pull/4419)
 - **[user-authz]** Disabled `NetworkPolicy` editing for Editors. [#4217](https://github.com/deckhouse/deckhouse/pull/4217)
 - **[virtualization]** Fix releasing disk lease when VM is not removed, but the disk is not attached. [#4499](https://github.com/deckhouse/deckhouse/pull/4499)
 - **[virtualization]** Fix copying `VirtualMachineDisk` from `VirtualMachineDisk`. [#4499](https://github.com/deckhouse/deckhouse/pull/4499)
 - **[virtualization]** Fix AdmissionReview for KubeVirt virtual machines [#4309](https://github.com/deckhouse/deckhouse/pull/4309)
 - **[virtualization]** Support other `cloud-init` sources. [#4176](https://github.com/deckhouse/deckhouse/pull/4176)

## Chore


 - **[candi]** Changes helm tolerations template functions. [#3959](https://github.com/deckhouse/deckhouse/pull/3959)
    All Pods will restart due to toleration changes.
 - **[cloud-provider-openstack]** Update gophercloud dependency. [#4454](https://github.com/deckhouse/deckhouse/pull/4454)
 - **[cloud-provider-vsphere]** Clarified implicit defaults in `VsphereInstanceClass` documentation. [#3982](https://github.com/deckhouse/deckhouse/pull/3982)
 - **[cni-cilium]** Run CNI cilium in a non-privileged environment with the maximum permissions restriction. [#4226](https://github.com/deckhouse/deckhouse/pull/4226)
    All cilium Pods will be restarted.
 - **[cni-cilium]** Bump cilium to `v1.12.7`. [#4079](https://github.com/deckhouse/deckhouse/pull/4079)
    All cilium Pods will be restarted.
 - **[cni-cilium]** Added cilium agent dashboard. [#3949](https://github.com/deckhouse/deckhouse/pull/3949)
 - **[dashboard]** Update of dashboard module to `2.7.0`. [#4029](https://github.com/deckhouse/deckhouse/pull/4029)
 - **[delivery]** Add FAQ section with the admin password reset instruction. [#4078](https://github.com/deckhouse/deckhouse/pull/4078)
 - **[dhctl]** Add warning during bootstrap: `Some resources require at least one non-master node to be added to the cluster`. [#4283](https://github.com/deckhouse/deckhouse/pull/4283)
 - **[log-shipper]** Update vector to `0.28.1`. [#4224](https://github.com/deckhouse/deckhouse/pull/4224)
 - **[monitoring-custom]** Set a new default value for services and Pods. [#4258](https://github.com/deckhouse/deckhouse/pull/4258)
 - **[operator-trivy]** Move the `operator-trivy` module to Deckhouse EE. [#4119](https://github.com/deckhouse/deckhouse/pull/4119)
    The `operator-trivy` module will no longer be available in Deckhouse CE.
 - **[runtime-audit-engine]** Added validating webhook to validate `FalcoAuditRules`. [#4263](https://github.com/deckhouse/deckhouse/pull/4263)
    All `runtime-audit-engine` Pods will be restarted.
 - **[terraform-manager]** Document missing IAM permissions in corresponding alerts for Deckhouse >= 1.45 [#4409](https://github.com/deckhouse/deckhouse/pull/4409)

