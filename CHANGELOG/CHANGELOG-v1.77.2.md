# Changelog v1.77.2

## [MALFORMED]


 - #23329 unknown section ""
 - #23360 unknown section ""

## Know before update


 - A StaticInstance whose connectivity check fails now keeps its reservation for the whole bootstrap window instead of returning to the pool on every failed attempt, and the bootstrap (20 min) and cleanup (10 min) timeouts, which previously never fired, are now reachable.
    If nothing ever ran on the host, the bootstrap timeout returns the instance to the pool after 20 minutes as before.
    If the host was already bootstrapped in part, the instance returns only through MachineHealthCheck remediation (nodeStartupTimeoutSeconds: 1200) plus the cleanup timeout, so it becomes available again roughly 20 + 10 minutes after the failure, and remediation reboots the host as part of the cleanup.
 - A cluster still on 1.76 whose user-authn ModuleConfig sets idTokenTTL to 6h or more updates to this release without Deckhouse crash-looping, and stays on it until idTokenTTL is lowered below 6h. A cluster already crash-looping on 1.77.0 or 1.77.1 does not recover by itself, since the release controller runs in the pod that is down; set failurePolicy Ignore on the module-configs.deckhouse-webhook.deckhouse.io webhook of the deckhouse-webhook ValidatingWebhookConfiguration, lower idTokenTTL below 6h, and restart the deckhouse pod.
 - Clusters bootstrapped before v1.55 were switched to the `Baseline` default PSS policy in 1.77, which denies privileged workloads in user namespaces.
    After the update, the default policy on these clusters becomes less strict: it is switched back to `Privileged`, and privileged pods in user namespaces pass admission again.
    To keep the `Baseline` policy, set `settings.podSecurityStandards.defaultPolicy: Baseline` explicitly in the `admission-policy-engine` ModuleConfig.

## Features


 - **[admission-policy-engine]** Added the `PodSecurityStandardsDefaultPolicyNotSet` alert that fires when the `install-data` ConfigMap is missing and the default PSS policy must be set explicitly in the ModuleConfig. [#23269](https://github.com/deckhouse/deckhouse/pull/23269)
 - **[deckhouse-controller]** Add the `x-deckhouse-immutable`, `x-deckhouse-ui-group`, and `x-deckhouse-enum-switch-settings` extensions to Application package settings schemas; the Application webhook rejects changes to `x-deckhouse-immutable` fields after the settings were applied. [#23360](https://github.com/deckhouse/deckhouse/pull/23360)
 - **[deckhouse-controller]** Application resources get the `packages.deckhouse.io/package` and `packages.deckhouse.io/instance` labels; the Helm release of every Application is upgraded once to add them. [#23360](https://github.com/deckhouse/deckhouse/pull/23360)

## Fixes


 - **[admission-policy-engine]** Fixed CVEs in the `gatekeeper` and `ratify` images. [#23321](https://github.com/deckhouse/deckhouse/pull/23321)
 - **[admission-policy-engine]** Restored the `Privileged` default PSS policy for clusters bootstrapped before v1.55 (without the `install-data` ConfigMap). [#23259](https://github.com/deckhouse/deckhouse/pull/23259)
    Clusters bootstrapped before v1.55 were switched to the `Baseline` default PSS policy in 1.77, which denies privileged workloads in user namespaces.
    After the update, the default policy on these clusters becomes less strict: it is switched back to `Privileged`, and privileged pods in user namespaces pass admission again.
    To keep the `Baseline` policy, set `settings.podSecurityStandards.defaultPolicy: Baseline` explicitly in the `admission-policy-engine` ModuleConfig.
 - **[cert-manager]** Fixed CVEs in the cert-manager images. [#23333](https://github.com/deckhouse/deckhouse/pull/23333)
 - **[cloud-provider-dvp]** Fixed the `D8CloudProviderDVPMigrationPending` alert that kept firing after the migration resources were applied. [#23248](https://github.com/deckhouse/deckhouse/pull/23248)
 - **[cloud-provider-dvp]** Fixes cluster bootstrap getting stuck when `capdvp-controller-manager` and the validation webhook cannot reach the API server and DNS before the CNI is ready. [#23357](https://github.com/deckhouse/deckhouse/pull/23357)
 - **[cloud-provider-openstack]** Fixes cluster bootstrap getting stuck when `capo-controller-manager` cannot reach the API server and DNS before the CNI is ready. [#23357](https://github.com/deckhouse/deckhouse/pull/23357)
 - **[cloud-provider-vcd]** Batch VM cache refresh on cache miss [#23315](https://github.com/deckhouse/deckhouse/pull/23315)
 - **[cloud-provider-vcd]** Fixes cluster bootstrap getting stuck when `capcd-controller-manager` cannot reach the API server and DNS before the CNI is ready. [#23357](https://github.com/deckhouse/deckhouse/pull/23357)
 - **[cloud-provider-yandex]** fix "Address in use" failures when replacing nodes and when removing external IP addresses [#23256](https://github.com/deckhouse/deckhouse/pull/23256)
    dhctl converge no longer deadlocks on master replacement or externalIPAddresses removal in Yandex Cloud; the terraform-manager provider release is patched to detach one-to-one NAT before deleting a reserved address.
 - **[common]** Fix CVEs in the `kube-rbac-proxy` image. [#23325](https://github.com/deckhouse/deckhouse/pull/23325)
 - **[common]** Fix kubelet podSandbox order [#23215](https://github.com/deckhouse/deckhouse/pull/23215)
 - **[deckhouse-controller]** Fix the rejection of every Application whose package settings schema uses `x-deckhouse-validations`. [#23360](https://github.com/deckhouse/deckhouse/pull/23360)
 - **[dhctl]** Fixed terraform-auto-converger, terraform-state-exporter and `dhctl converge` failing on cloud clusters whose `cloud-provider-<name>` ModuleConfig has no `spec.enabled` field. dhctl now treats `spec.enabled` as optional and a ModuleConfig without it as disabled. [#23317](https://github.com/deckhouse/deckhouse/pull/23317)
 - **[dhctl]** Restoring of handling api_server_url and api_server_token in dhctl [#23293](https://github.com/deckhouse/deckhouse/pull/23293)
 - **[kube-dns]** Set resource limits for the `render-etc-hosts-with-cluster-domain-aliases` init container so that pods can be created in namespaces with a ResourceQuota on `limits.cpu`/`limits.memory`. [#23295](https://github.com/deckhouse/deckhouse/pull/23295)
 - **[kube-proxy]** Fixed kube-proxy failing to start on Kubernetes 1.36 and higher because of the removed `TopologyAwareHints` feature gate. [#23336](https://github.com/deckhouse/deckhouse/pull/23336)
    The `d8-kube-proxy` pods will be restarted on all nodes.
 - **[metallb]** Fixed creation of `BGPPeer` objects on Kubernetes 1.36+, which blocked BGP sessions and LoadBalancer IP announcements in BGP mode. [#23312](https://github.com/deckhouse/deckhouse/pull/23312)
 - **[multitenancy-manager]** Fixed vulnerabilities in the `multitenancy-manager` image. [#23361](https://github.com/deckhouse/deckhouse/pull/23361)
 - **[node-manager]** Increase the capi-controller-manager VPA maximum memory recommendation to prevent health-check failures and restarts under memory pressure. [#23203](https://github.com/deckhouse/deckhouse/pull/23203)
 - **[node-manager]** Stop caps-controller-manager from rewriting StaticInstance objects in a hot loop when a connectivity check fails, which caused a sustained load on etcd. [#23207](https://github.com/deckhouse/deckhouse/pull/23207)
    A StaticInstance whose connectivity check fails now keeps its reservation for the whole bootstrap window instead of returning to the pool on every failed attempt, and the bootstrap (20 min) and cleanup (10 min) timeouts, which previously never fired, are now reachable.
    If nothing ever ran on the host, the bootstrap timeout returns the instance to the pool after 20 minutes as before.
    If the host was already bootstrapped in part, the instance returns only through MachineHealthCheck remediation (nodeStartupTimeoutSeconds: 1200) plus the cleanup timeout, so it becomes available again roughly 20 + 10 minutes after the failure, and remediation reboots the host as part of the cleanup.
 - **[service-with-healthchecks]** Fixed metrics endpoints being reachable without going through kube-rbac-proxy authorization. [#23166](https://github.com/deckhouse/deckhouse/pull/23166)
 - **[user-authn]** An idTokenTTL of 6h or more no longer stops Deckhouse from starting; it raises the D8UserAuthnIDTokenTTLTooLong alert and blocks the update to the next minor release until it is lowered. [#23267](https://github.com/deckhouse/deckhouse/pull/23267)
    A cluster still on 1.76 whose user-authn ModuleConfig sets idTokenTTL to 6h or more updates to this release without Deckhouse crash-looping, and stays on it until idTokenTTL is lowered below 6h. A cluster already crash-looping on 1.77.0 or 1.77.1 does not recover by itself, since the release controller runs in the pod that is down; set failurePolicy Ignore on the module-configs.deckhouse-webhook.deckhouse.io webhook of the deckhouse-webhook ValidatingWebhookConfiguration, lower idTokenTTL below 6h, and restart the deckhouse pod.
 - **[user-authn]** Fixed CVEs in the module images. [#23373](https://github.com/deckhouse/deckhouse/pull/23373)
 - **[user-authz]** Fixed CVEs in the `permission-browser-apiserver` image. [#23324](https://github.com/deckhouse/deckhouse/pull/23324)

## Chore


 - **[cni-cilium]** BPF conntrack, NAT, neighbor and socket reverse NAT map sizes can now be set per node with CiliumNodeConfig. [#23385](https://github.com/deckhouse/deckhouse/pull/23385)
    `cilium-agent` Pods will be restarted.
 - **[deckhouse-controller]** Bump nelm to v1.31.0. The `status.tracking` report of Application now has the operation graph (`id`, `category`, `dependsOn`), the `Canceled` status, and no `waitingFor`. [#23191](https://github.com/deckhouse/deckhouse/pull/23191)
 - **[deckhouse-controller]** The Application status reports the `Deleting` state while the application is being removed. [#23360](https://github.com/deckhouse/deckhouse/pull/23360)
