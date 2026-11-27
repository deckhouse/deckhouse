# Changelog v1.78.2

## Features


 - **[node-manager]** Support manually provisioned Deckhouse Engine nodes in CloudStatic node groups.

## Fixes


 - **[candi]** A joining node finds registry-packages-proxy through the cluster when the proxy has moved since its bootstrap script was rendered, so the installation no longer hangs waiting for the node.
 - **[cloud-provider-openstack]** Fix OpenStackCluster rendering when network lists are missing or empty
 - **[cloud-provider-openstack]** fix cinder-csi self-deadlock on re-auth after token expiry
 - **[control-plane-manager]** Check free disk space against the actual etcd database size before taking a snapshot and preserve the previous backup if archiving fails.
 - **[deckhouse-controller]** Fixed Commander cluster synchronization after upgrading to DKP 1.78 when a template omits the default node subnet prefix or cluster domain.
 - **[dhctl]** Destroy no longer deletes the static CAPI Cluster, which made CAPS wipe masters added from StaticInstances and broke etcd quorum before dhctl cleaned them.
 - **[multitenancy-manager]** Removed alerts that fire on healthy clusters or on Deckhouse internals, and stopped alerting on objects kept by projects of built-in templates.
    Alerts removed: D8ProjectQuotaNotSupported, MultitenancyManagerProjectNamedLikeAdditionalNamespace, MultitenancyManagerProjectHandoverStuck, D8ProjectQuotaLedgerDrift, D8ProjectQuotaLeaderFlapping, D8ProjectQuotaLedgerSlow, D8ProjectQuotaOvershoot, D8ResourceQuotaAdmissionDegraded. Routes and silences that match these names no longer match anything. MultitenancyManagerProjectKeepsHelmTemplateObjects no longer fires for projects of built-in templates.
 - **[node-manager]** Defer OS image updates on Deckhouse Engine nodes until bootstrap finishes and the node becomes Ready, and wait for managed pods before broadcasting shutdown to kubelet (nodelet v0.1.24, Engine image v0.1.21).
    Deckhouse Engine nodes reboot into the new OS image according to their NodeGroup update rules.
 - **[node-manager]** Discover live registry-packages-proxy pods via kube-apiserver in nodelet when derived apiServerEndpoints fail (nodelet v0.1.25).
 - **[node-manager]** Keep master controller pod templates invariant when additional masters join and drop redundant anti-affinity on single-replica node-group-exporter.
    Single-master and non-HA clusters will roll out master controllers once; HA clusters will roll out node-group-exporter once. Rolling node-group-exporter back to the prior manifest on a single monitoring node requires a temporary 0/1 strategy.
 - **[registry-packages-proxy]** Keep registry-packages-proxy pod template invariant when additional masters join so the proxy does not roll out mid-bootstrap.
    Single-master clusters and clusters with highAvailability disabled will roll out registry-packages-proxy once on update.
 - **[registry-packages-proxy]** Run one registry-packages-proxy replica on every master node so each master address serves package proxy ports.
 - **[registry-packages-proxy]** The proxy no longer crashes when the API server refuses its watch, and starts the watch again.
 - **[registry]** Every registry cache replica holds the images of a ModuleSource on the upstream's `modules` path where the nodes pull them, so the cache keeps serving them after a leader change while the upstream is unreachable.
 - **[registry]** In the Managed mode pods reach the in-cluster registry address `registry.d8-system.svc:5001` through the Service `d8-system/registry`, with a read-only account for the platform's image set and the store's certificate authority in the global registry values, so modules that fetch their own artifacts, such as operator-trivy, work again.
    Deckhouse and multitenancy-manager restart once. Without the cache, registry-storage-proxy pods run on the master nodes. The registry-agent static pods on all nodes, the registry-storage pods on master nodes and registry-controller restart. Rolling back to a version without this change requires deleting the Service `d8-system/registry` by hand.
 - **[registry]** Keep registry master workload pod templates invariant across master count transitions.
    Single-master clusters will roll out registry-controller, registry-storage and registry-storage-image-holder once on update.
 - **[registry]** Leaving the Managed mode for Unmanaged no longer leaves a node with the agent's containerd drop-in pointing at the stopped agent, or without the registry configuration the agent removed.
 - **[registry]** The registry cache keeps and fills the images of modules run through a ModulePullOverride, so garbage collection no longer deletes images their pods run on.
    The registry-storage pods on master nodes restart one by one.
 - **[registry]** registry-agent passes the request query on to the registry, so paginated tag listings, including the module source listing, return every page.
    The registry-agent static pod restarts on every node.
 - **[registrypackages]** rpp-get tries every registry-packages-proxy endpoint before an attempt fails and retries connection timeouts, so a node that reaches only the master of its own zone during bootstrap joins the cluster.
 - **[user-authn]** Removed alerts about the internal state of user-authn-controller and dex-authenticator, and stopped alerting on the DexProvider of Deckhouse Commander and on the DexAuthenticators in the d8-* namespaces.
    Alerts removed: D8UserAuthnIdentityMigrationIncomplete, D8UserAuthnRevocationFeedLagging, D8UserAuthnControllerReconcileErrorsHigh. Routes and silences that match these names no longer match anything.
 - **[user-authz]** Removed alerts a cluster administrator cannot act on, and stopped alerting on the ClusterRoles of modules in the foreign aggregation label and legacy custom role alerts.
    Alerts removed: D8UserAuthzRulePropagationLag, D8UserAuthzReleaseSlow, D8UserAuthzRenderSlow, D8UserAuthzControllerReconcileErrorsHigh. Routes and silences that match these names no longer match anything. The ClusterRoles of modules no longer hold an update on the legacyRBACv2CustomRolesCount requirement.

## Chore


 - **[candi]** Kubernetes patch versions are updated to 1.35.11, 1.36.7 and 1.37.3.
    Control plane components, kubelet on all nodes and kube-proxy will restart.
 - **[control-plane-manager]** etcd is updated to 3.6.17.
    etcd on all master nodes will be restarted.
 - **[node-manager]** Add kernel version guidance to the containerd v2 migration alert for nodes with 100 or more CPUs.
