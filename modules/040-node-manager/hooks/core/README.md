# core

Hooks of the node-manager team: NodeGroup lifecycle, bashible, CAPI and MCM plumbing, certificates of the module's own components.

- `get_crds` reads every NodeGroup and publishes the fields the helm chart still needs (node type, engine, zones, min and max per zone, GPU, fencing, static instances) as one values key. Templates and a few hooks read that key instead of the cluster. The per-group logic (kubernetes version, capacity, labels and taints) already lives in node-controller. The hook is scheduled for removal once its last template readers are gone.
- `deployment_required` decides whether machine-controller-manager (MCM) has to run in this cluster by looking at the engine the NodeGroups use. The chart renders MCM only when a group needs it.
- `discover_cloud_provider` copies the cloud provider settings from the provider secret into values, so the chart and other hooks know the provider type, its zones and the instance class kind without reading the secret themselves.
- `discover_apiserver_endpoints` collects the addresses of the kube-apiserver instances. bashible on a new node and the CAPI cluster templates need them before cluster DNS is available.
- `enable_cluster_api_cloud_and_static` turns on the CAPI controller for cloud groups and the CAPS controller for static groups only when a NodeGroup actually needs them, so a cluster without static nodes does not run CAPS.
- `ensure_internal_crds` installs the internal CAPI bootstrap CRDs from `crds/internal`. They are kept apart from the public CRDs because users never create these objects by hand.
- `set_instance_prefix` resolves the prefix of machine names: the module setting, then the global prefix, then the legacy cluster configuration. The chart puts it into the cluster-autoscaler arguments. node-controller resolves the same prefix on its own when it names MachineDeployments.
- `gen_bashible_apiserver_certs`, `generate_node_webhook_certs` and `generate_capi_webhook_certs` issue self-signed TLS for bashible-apiserver, the node-controller webhook and the CAPI webhook and renew them before expiry. The chart needs the CA to wire the APIService and the webhook configurations, so the components cannot issue these certificates themselves.
- `generate_capi_kubeconfig` issues a client certificate for capi-controller-manager through a CSR and writes the kubeconfig secret in the form Cluster API expects, one for the cloud cluster and one for the static cluster.
- `set_keep_policy_on_capi_resources` marks the CAPI, MCM and bootstrap objects that helm rendered before the node-controller migration, so a release that no longer renders them does not delete them. Scheduled for removal in 1.81, when an upgrade from a pre-migration release is impossible.
- `lock_bashible_apiserver` locks the bashible context while the running bashible-apiserver image differs from the one in the release, so nodes do not receive a configuration rendered by another version.
- `minimal_node_os_version` records the oldest Ubuntu and Debian versions among the nodes for the release requirements check, which holds a Deckhouse upgrade that would drop support for them.
- `trim_machine_set_revision_history` keeps the revision history annotation of MCM MachineSets short, because the vendored MCM never trims it.
- `chaos_monkey` deletes a random machine of a NodeGroup with chaos mode enabled, once per period, to exercise recovery. It knows MCM machines only.
- `change_host_ip` restarts bashible-apiserver pods whose host IP changed. It is the shared platform hook from `go_lib`, instantiated here.
- `util.go` holds the helpers shared by the hooks of this package.
