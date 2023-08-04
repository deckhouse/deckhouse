# Changelog v1.48

## Features


 - **[admission-policy-engine]** Add `RequiredAnnotation` policy to the Deckhouse `OperationPolicy` resource. [#5090](https://github.com/deckhouse/deckhouse/pull/5090)
 - **[admission-policy-engine]** Add a security policy entity to check workload against adjustable set of security rules. [#4828](https://github.com/deckhouse/deckhouse/pull/4828)
 - **[candi]** Add Kubernetes 1.27 support. [#4631](https://github.com/deckhouse/deckhouse/pull/4631)
    All control plane components will be restarted.
 - **[cloud-provider-openstack]** Add OpenStack cloud provider discovery data. [#4793](https://github.com/deckhouse/deckhouse/pull/4793)
 - **[dashboard]** Show username and groups. [#5128](https://github.com/deckhouse/deckhouse/pull/5128)
 - **[ingress-nginx]** Provide a High Availability setting for `ingress-nginx` module's control-plane components. [#5046](https://github.com/deckhouse/deckhouse/pull/5046)
 - **[istio]** Added a way to globally override resources for `istio-proxy`. [#4852](https://github.com/deckhouse/deckhouse/pull/4852)
 - **[okmeter]** Mount `containerd` socket into `okmeter` DaemonSet Pods to meet the requirement for `containerd` monitoring feature. [#5304](https://github.com/deckhouse/deckhouse/pull/5304)
    Rolling upgrade of the `okmeter` DaemonSet. This will cause a short disruption in node monitoring.
 - **[prometheus]** Improves the `TargetSampleLimitExceeded` alert by adding target labels. [#4795](https://github.com/deckhouse/deckhouse/pull/4795)
 - **[user-authn]** Add `Group` object and migration. The `groups` property of the `User` object becomes read-only.  Migration hook will create groups for all users based on this property. To continue to deploy `User` objects, users must remove groups from the manifest. [#4406](https://github.com/deckhouse/deckhouse/pull/4406)

## Fixes


 - **[admission-policy-engine]** Improve bootstrap handler logic. [#5242](https://github.com/deckhouse/deckhouse/pull/5242)
 - **[admission-policy-engine]** Fix constraint exporter - avoid duplications and invalid resource handling. [#5240](https://github.com/deckhouse/deckhouse/pull/5240)
 - **[candi]** Fix router metric if the `additionalNetworks` parameter is specified. [#5286](https://github.com/deckhouse/deckhouse/pull/5286)
 - **[candi]** Fix bootstraping master node for AltLinux bundle. [#5099](https://github.com/deckhouse/deckhouse/pull/5099)
 - **[candi]** Add a node approval annotations step. [#5047](https://github.com/deckhouse/deckhouse/pull/5047)
 - **[candi]** Remove the property `etcdDisk` in `nodeGroups` and remove anchor inheritance in the `AWSClusterConfiguration` OpenAPI specification. [#4977](https://github.com/deckhouse/deckhouse/pull/4977)
 - **[cloud-provider-aws]** Add rbac Role to access d8-provider-cluster-configuration and d8-cloud-provider-discovery-data secrets. [#5291](https://github.com/deckhouse/deckhouse/pull/5291)
 - **[cloud-provider-azure]** Add rbac Role to access d8-provider-cluster-configuration and d8-cloud-provider-discovery-data secrets. [#5291](https://github.com/deckhouse/deckhouse/pull/5291)
 - **[cloud-provider-azure]** Fix behavior of the `acceleratedNetworking` variable. [#5000](https://github.com/deckhouse/deckhouse/pull/5000)
 - **[cloud-provider-gcp]** Add rbac Role to access d8-provider-cluster-configuration and d8-cloud-provider-discovery-data secrets. [#5291](https://github.com/deckhouse/deckhouse/pull/5291)
 - **[cloud-provider-openstack]** Remove 'minLength: 1' requirement from mainNetwork and defaultImageName properties in OpenAPI specification. [#5386](https://github.com/deckhouse/deckhouse/pull/5386)
 - **[cloud-provider-openstack]** Fix discovery security groups and errors in hybrid clusters. [#5341](https://github.com/deckhouse/deckhouse/pull/5341)
 - **[cloud-provider-openstack]** Fix creating long name of backup secret in state. [#5332](https://github.com/deckhouse/deckhouse/pull/5332)
 - **[cloud-provider-openstack]** Remove duplicates from additional security groups in cloud-data-discoverer. [#5279](https://github.com/deckhouse/deckhouse/pull/5279)
 - **[cloud-provider-openstack]** Fix migration from `openstack_blockstorage_volume_v2` to `openstack_blockstorage_volume_v3` for bastion hosts. [#5271](https://github.com/deckhouse/deckhouse/pull/5271)
 - **[cloud-provider-openstack]** Remove duplicates from images list in `cloud-data-discoverer`. [#5241](https://github.com/deckhouse/deckhouse/pull/5241)
 - **[cloud-provider-openstack]** Fix discover volume types hooks incorrect fallback to storage classes in another modules. [#5233](https://github.com/deckhouse/deckhouse/pull/5233)
 - **[common]** Add commonName field to Deckhouse X.509 certificates. [#4999](https://github.com/deckhouse/deckhouse/pull/4999)
 - **[control-plane-manager]** Restoring `D8KubernetesVersionIsDeprecated` alert from `1.23` to `1.22`. [#5249](https://github.com/deckhouse/deckhouse/pull/5249)
 - **[go_lib]** Add test that original SANs are not mutated. [#5290](https://github.com/deckhouse/deckhouse/pull/5290)
 - **[helm_lib]** Fix X.509 certificate common name generation. [#5342](https://github.com/deckhouse/deckhouse/pull/5342)
 - **[runtime-audit-engine]** Mount docker and containerd sockets to fetch metadata. [#5110](https://github.com/deckhouse/deckhouse/pull/5110)
 - **[runtime-audit-engine]** Mount falco config to rules-loader to enable plugins for validating webhook. Otherwise, webhook returns an error for valid rules. [#5110](https://github.com/deckhouse/deckhouse/pull/5110)
 - **[runtime-audit-engine]** Output webhook validation error. Without this change, users have to search it in logs among all running falco pods. [#5110](https://github.com/deckhouse/deckhouse/pull/5110)
 - **[runtime-audit-engine]** Fix `FalcoAuditRules` resource name in rules. [#5110](https://github.com/deckhouse/deckhouse/pull/5110)
 - **[user-authn]** Add custom certificate for kubeconfig generator. [#5424](https://github.com/deckhouse/deckhouse/pull/5424)
 - **[user-authn]** Fix kubeconfig certificate generation. [#5273](https://github.com/deckhouse/deckhouse/pull/5273)
 - **[user-authn]** Improve groups migration (run only once and slugify group names). [#5130](https://github.com/deckhouse/deckhouse/pull/5130)
 - **[user-authn]** Disable env expansion to support dollar character in `bindPW` for LDAP connector. [#5106](https://github.com/deckhouse/deckhouse/pull/5106)

## Chore


 - **[candi]** Bump patch versions of Kubernetes images: `v1.27.3` [#5246](https://github.com/deckhouse/deckhouse/pull/5246)
    Kubernetes 1.27 control plane components and kubelet will restart.
 - **[candi]** Move the `altlinux` bundle to EE edition. [#4970](https://github.com/deckhouse/deckhouse/pull/4970)
 - **[candi]** Added the `etcdDiskSizeGb` parameter to Yandex Cloud, GCP, and Azure `ClusterConfiguration`. [#4720](https://github.com/deckhouse/deckhouse/pull/4720)
 - **[candi]** Added the `etcdDiskSizeGb` parameter for OpenStack and migration from `openstack_blockstorage_volume_v2` to `openstack_blockstorage_volume_v3`. [#4716](https://github.com/deckhouse/deckhouse/pull/4716)
 - **[helm_lib]** Update `lib_helm` chart. [#5193](https://github.com/deckhouse/deckhouse/pull/5193)
 - **[openvpn]** Update easyrsa migrator dependencies. [#5177](https://github.com/deckhouse/deckhouse/pull/5177)

