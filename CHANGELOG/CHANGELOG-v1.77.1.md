# Changelog v1.77.1

## Know before update


 - A StaticInstance whose connectivity check fails now keeps its reservation for the whole bootstrap window instead of returning to the pool on every failed attempt, and the bootstrap (20 min) and cleanup (10 min) timeouts, which previously never fired, are now reachable.
    If nothing ever ran on the host, the bootstrap timeout returns the instance to the pool after 20 minutes as before.
    If the host was already bootstrapped in part, the instance returns only through MachineHealthCheck remediation (nodeStartupTimeoutSeconds: 1200) plus the cleanup timeout, so it becomes available again roughly 20 + 10 minutes after the failure, and remediation reboots the host as part of the cleanup.
 - Clusters bootstrapped before v1.55 were switched to the `Baseline` default PSS policy in 1.77, which denies privileged workloads in user namespaces.
    After the update, the default policy on these clusters becomes less strict: it is switched back to `Privileged`, and privileged pods in user namespaces pass admission again.
    To keep the `Baseline` policy, set `settings.podSecurityStandards.defaultPolicy: Baseline` explicitly in the `admission-policy-engine` ModuleConfig.

## Fixes


 - **[admission-policy-engine]** Restored the `Privileged` default PSS policy for clusters bootstrapped before v1.55 (without the `install-data` ConfigMap). [#23259](https://github.com/deckhouse/deckhouse/pull/23259)
    Clusters bootstrapped before v1.55 were switched to the `Baseline` default PSS policy in 1.77, which denies privileged workloads in user namespaces.
    After the update, the default policy on these clusters becomes less strict: it is switched back to `Privileged`, and privileged pods in user namespaces pass admission again.
    To keep the `Baseline` policy, set `settings.podSecurityStandards.defaultPolicy: Baseline` explicitly in the `admission-policy-engine` ModuleConfig.
 - **[cloud-provider-dvp]** Fixed the `D8CloudProviderDVPMigrationPending` alert that kept firing after the migration resources were applied. [#23248](https://github.com/deckhouse/deckhouse/pull/23248)
 - **[common]** Fix kubelet podSandbox order [#23215](https://github.com/deckhouse/deckhouse/pull/23215)
 - **[node-manager]** Increase the capi-controller-manager VPA maximum memory recommendation to prevent health-check failures and restarts under memory pressure. [#23203](https://github.com/deckhouse/deckhouse/pull/23203)
 - **[node-manager]** Stop caps-controller-manager from rewriting StaticInstance objects in a hot loop when a connectivity check fails, which caused a sustained load on etcd. [#23207](https://github.com/deckhouse/deckhouse/pull/23207)
    A StaticInstance whose connectivity check fails now keeps its reservation for the whole bootstrap window instead of returning to the pool on every failed attempt, and the bootstrap (20 min) and cleanup (10 min) timeouts, which previously never fired, are now reachable.
    If nothing ever ran on the host, the bootstrap timeout returns the instance to the pool after 20 minutes as before.
    If the host was already bootstrapped in part, the instance returns only through MachineHealthCheck remediation (nodeStartupTimeoutSeconds: 1200) plus the cleanup timeout, so it becomes available again roughly 20 + 10 minutes after the failure, and remediation reboots the host as part of the cleanup.
 - **[service-with-healthchecks]** Fixed metrics endpoints being reachable without going through kube-rbac-proxy authorization. [#23166](https://github.com/deckhouse/deckhouse/pull/23166)
