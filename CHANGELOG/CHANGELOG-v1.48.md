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
 - **[prometheus]** Improves the `TargetSampleLimitExceeded` alert by adding target labels. [#4795](https://github.com/deckhouse/deckhouse/pull/4795)
 - **[user-authn]** Add `Group` object and migration. The `groups` property of the `User` object becomes read-only.  Migration hook will create groups for all users based on this property. To continue to deploy `User` objects, users must remove groups from the manifest. [#4406](https://github.com/deckhouse/deckhouse/pull/4406)

## Fixes


 - **[candi]** Fix bootstraping master node for AltLinux bundle. [#5099](https://github.com/deckhouse/deckhouse/pull/5099)
 - **[candi]** Add a node approval annotations step. [#5047](https://github.com/deckhouse/deckhouse/pull/5047)
 - **[candi]** Remove the property `etcdDisk` in `nodeGroups` and remove anchor inheritance in the `AWSClusterConfiguration` OpenAPI specification. [#4977](https://github.com/deckhouse/deckhouse/pull/4977)
 - **[cloud-provider-azure]** Fix behavior of the `acceleratedNetworking` variable. [#5000](https://github.com/deckhouse/deckhouse/pull/5000)
 - **[cloud-provider-openstack]** Fix discover volume types hooks incorrect fallback to storage classes in another modules. [#5233](https://github.com/deckhouse/deckhouse/pull/5233)
 - **[common]** Add commonName field to Deckhouse X.509 certificates. [#4999](https://github.com/deckhouse/deckhouse/pull/4999)
 - **[runtime-audit-engine]** Mount docker and containerd sockets to fetch metadata. [#5110](https://github.com/deckhouse/deckhouse/pull/5110)
 - **[runtime-audit-engine]** Mount falco config to rules-loader to enable plugins for validating webhook. Otherwise, webhook returns an error for valid rules. [#5110](https://github.com/deckhouse/deckhouse/pull/5110)
 - **[runtime-audit-engine]** Output webhook validation error. Without this change, users have to search it in logs among all running falco pods. [#5110](https://github.com/deckhouse/deckhouse/pull/5110)
 - **[runtime-audit-engine]** Fix `FalcoAuditRules` resource name in rules. [#5110](https://github.com/deckhouse/deckhouse/pull/5110)
 - **[user-authn]** Improve groups migration (run only once and slugify group names). [#5130](https://github.com/deckhouse/deckhouse/pull/5130)
 - **[user-authn]** Disable env expansion to support dollar character in `bindPW` for LDAP connector. [#5106](https://github.com/deckhouse/deckhouse/pull/5106)

## Chore


 - **[candi]** Move the `altlinux` bundle to EE edition. [#4970](https://github.com/deckhouse/deckhouse/pull/4970)
 - **[candi]** Added the `etcdDiskSizeGb` parameter to Yandex Cloud, GCP, and Azure `ClusterConfiguration`. [#4720](https://github.com/deckhouse/deckhouse/pull/4720)
 - **[candi]** Added the `etcdDiskSizeGb` parameter for OpenStack and migration from `openstack_blockstorage_volume_v2` to `openstack_blockstorage_volume_v3`. [#4716](https://github.com/deckhouse/deckhouse/pull/4716)
 - **[helm_lib]** Update `lib_helm` chart. [#5193](https://github.com/deckhouse/deckhouse/pull/5193)
 - **[openvpn]** Update easyrsa migrator dependencies. [#5177](https://github.com/deckhouse/deckhouse/pull/5177)

