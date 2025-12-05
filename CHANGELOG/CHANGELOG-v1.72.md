# Changelog v1.72

## Know before update


 - Fixes a bug where creating new User resources failed due to missing oldObject in validation; password immutability is still enforced on update.
 - If wireguard interface is present on nodes, then cilium-agent upgrade will stuck. Upgrading the linux kernel to 6.8 is required.
 - Kube-apiserver will be restarted to migrate to the AuthenticationConfiguration configuration file.

## Features


 - **[cert-manager]** Added abitity for specifying recursive DNS servers list (recursiveSettings parameters section), which are used to check the existence of a DNS record, before initiating the domain ownership verification process using the [ACME DNS-01](https://cert-manager.io/docs/configuration/acme/dns01/) method. [#14669](https://github.com/deckhouse/deckhouse/pull/14669)
 - **[cloud-provider-aws]** Maked the creation of default security groups optional. [#14604](https://github.com/deckhouse/deckhouse/pull/14604)
 - **[cloud-provider-huaweicloud]** Added discovery logic so Cluster Autoscaler can create nodes starts with zero replicas. [#14835](https://github.com/deckhouse/deckhouse/pull/14835)
 - **[cloud-provider-vcd]** Added WithNAT layout at VCD cloud-provider. [#13954](https://github.com/deckhouse/deckhouse/pull/13954)
 - **[cloud-provider-vsphere]** Maked mainNetwork optional in Vsphere InstanceClass. [#14372](https://github.com/deckhouse/deckhouse/pull/14372)
 - **[cni-cilium]** Added hook for disable cni-exclusive when sdn agent daemonset was discovered. [#14627](https://github.com/deckhouse/deckhouse/pull/14627)
 - **[control-plane-manager]** Migrated to AuthenticationConfiguration config file. [#14788](https://github.com/deckhouse/deckhouse/pull/14788)
    Kube-apiserver will be restarted to migrate to the AuthenticationConfiguration configuration file.
 - **[deckhouse]** Added d8 config validation webhook. [#14726](https://github.com/deckhouse/deckhouse/pull/14726)
 - **[deckhouse]** Downloaded modules can be enabled by bundle. [#14652](https://github.com/deckhouse/deckhouse/pull/14652)
 - **[deckhouse]** Added experimental flag for modules. [#14630](https://github.com/deckhouse/deckhouse/pull/14630)
 - **[deckhouse]** Added moduleConfig properties for registry. [#14327](https://github.com/deckhouse/deckhouse/pull/14327)
 - **[deckhouse]** Added the edition available and enabled extenders. [#14310](https://github.com/deckhouse/deckhouse/pull/14310)
 - **[deckhouse]** Separated queues for critical and functional modules. [#13906](https://github.com/deckhouse/deckhouse/pull/13906)
 - **[deckhouse-controller]** ModuleRelease supports from/to update constraints to skip step-by-step upgrades and jump to a target. release [#15298](https://github.com/deckhouse/deckhouse/pull/15298)
    Pending releases between the deployed version and the target endpoint are automatically marked Skipped; the endpoint is processed as a minor update (respects module readiness and update windows)
 - **[deckhouse-controller]** ignore migrate module if disabled [#15297](https://github.com/deckhouse/deckhouse/pull/15297)
 - **[deckhouse-controller]** Added task queue performance improvements with linked list implementation. [#14848](https://github.com/deckhouse/deckhouse/pull/14848)
 - **[deckhouse-controller]** Added alerts for modules that are outdated by N minor versions. [#14833](https://github.com/deckhouse/deckhouse/pull/14833)
    New alerts will be triggered when modules in manual update mode fall behind the latest available version by 1, 2, or 3+ minor versions. This helps users maintain module compatibility with modules updates.
 - **[deckhouse-controller]** Added implement major version update restrictions. [#14684](https://github.com/deckhouse/deckhouse/pull/14684)
 - **[deckhouse-controller]** Added implement metrics collector library. [#14472](https://github.com/deckhouse/deckhouse/pull/14472)
 - **[deckhouse-controller]** Disabled a module will delete its Pending ModuleReleases. [#14463](https://github.com/deckhouse/deckhouse/pull/14463)
 - **[deckhouse-controller]** Added a validating webhook for DeckhouseRelease to prevent approval if requirements are not met. [#14365](https://github.com/deckhouse/deckhouse/pull/14365)
 - **[deckhouse-controller]** Upgraded Deckhouse deployment now use Patch instead of Update. [#14311](https://github.com/deckhouse/deckhouse/pull/14311)
 - **[dhctl]** Added password authentication support to dhctl. [#13240](https://github.com/deckhouse/deckhouse/pull/13240)
 - **[docs]** Added the registry module docs. [#14327](https://github.com/deckhouse/deckhouse/pull/14327)
 - **[docs]** Updated CAPS resources API version. [#13445](https://github.com/deckhouse/deckhouse/pull/13445)
 - **[node-local-dns]** The `enableLogs' option has been added to the ModuleConfig, which logs all DNS queries when enabled. [#14672](https://github.com/deckhouse/deckhouse/pull/14672)
 - **[node-manager]** Add and increase lease duration timeouts to CAPS. [#15349](https://github.com/deckhouse/deckhouse/pull/15349)
 - **[node-manager]** Added SSH password auth to CAPS controller manager. [#14811](https://github.com/deckhouse/deckhouse/pull/14811)
 - **[node-manager]** Bumped Cluster API from 1.7.5 to 1.10.4. [#14603](https://github.com/deckhouse/deckhouse/pull/14603)
 - **[node-manager]** Updated go.mod dependencies. [#14327](https://github.com/deckhouse/deckhouse/pull/14327)
 - **[registry]** Added a relax registry check mode for switching between deckhouse editions. [#14860](https://github.com/deckhouse/deckhouse/pull/14860)
 - **[registry]** Added the registry module. [#14327](https://github.com/deckhouse/deckhouse/pull/14327)
 - **[user-authn]** Add documentation examples for PasswordPolicy and 2FA [#15270](https://github.com/deckhouse/deckhouse/pull/15270)
    Provides administrators with clear examples for configuring password policies, user creation, group management, and enabling two-factor authentication.
 - **[user-authn]** Added implement password policy logic for local user accounts. Now it is possible to set complexity level of passwords, failed attempts number to block the user, keep password history and force renewing the password after specified amount of time. [#14993](https://github.com/deckhouse/deckhouse/pull/14993)

## Fixes


 - **[candi]** Switch node hostname in discover_node_ip.sh to locked hostname [#15799](https://github.com/deckhouse/deckhouse/pull/15799)
 - **[candi]** Fixed deletion of NodeUsers. [#13240](https://github.com/deckhouse/deckhouse/pull/13240)
 - **[candi]** Fixed default AWS subnets tags for LB controller autodiscovery. [#10138](https://github.com/deckhouse/deckhouse/pull/10138)
 - **[cert-manager]** Mark module as critical for cluster [#16078](https://github.com/deckhouse/deckhouse/pull/16078)
 - **[cloud-provider-aws]** fix ssh access sg creation with disableDefaultSecurityGroup passed [#16081](https://github.com/deckhouse/deckhouse/pull/16081)
 - **[cloud-provider-aws]** Fixed incorrect template id for AWS e2e cluster. [#14605](https://github.com/deckhouse/deckhouse/pull/14605)
 - **[cloud-provider-dvp]** fix single-node LoadBalancer bug and add multi-LB per node support [#14883](https://github.com/deckhouse/deckhouse/pull/14883)
 - **[cloud-provider-dvp]** Fixed logic of work with disks and coreFraction validation. [#14284](https://github.com/deckhouse/deckhouse/pull/14284)
 - **[cloud-provider-huaweicloud]** add rule to d8:cloud-provider-huaweicloud:cloud-controller-manager account for list pods as needed [#15678](https://github.com/deckhouse/deckhouse/pull/15678)
 - **[cloud-provider-openstack]** fix openstack ccm deployment [#15601](https://github.com/deckhouse/deckhouse/pull/15601)
 - **[cloud-provider-openstack]** Loadbalancers in Openstack clouds will be renamed to match cloud prefix if it is set. [#12180](https://github.com/deckhouse/deckhouse/pull/12180)
 - **[cloud-provider-vsphere]** fix hook logick [#16028](https://github.com/deckhouse/deckhouse/pull/16028)
 - **[cloud-provider-vsphere]** cloud-data-discoverer fixes [#15775](https://github.com/deckhouse/deckhouse/pull/15775)
 - **[cloud-provider-vsphere]** cloud-data-discoverer fixes [#15507](https://github.com/deckhouse/deckhouse/pull/15507)
 - **[cloud-provider-vsphere]** Moved datastore discovery (via vSphere SDK calls) from hook to cloud-data-discovery. [#14519](https://github.com/deckhouse/deckhouse/pull/14519)
 - **[cloud-provider-vsphere]** Fixed main network escaping for names with special symbols. [#14328](https://github.com/deckhouse/deckhouse/pull/14328)
 - **[cloud-provider-vsphere]** Made internalNetworkCIDR optional. [#14317](https://github.com/deckhouse/deckhouse/pull/14317)
 - **[cloud-provider-zvirt]** Replaced virtio instead of virtio-scsi. [#13984](https://github.com/deckhouse/deckhouse/pull/13984)
 - **[cni-cilium]** improved migration to 1.17 logic [#15602](https://github.com/deckhouse/deckhouse/pull/15602)
 - **[cni-cilium]** Add a compatibility check for the Cilium version and the kernel version, if WireGuard is installed on the node. [#15228](https://github.com/deckhouse/deckhouse/pull/15228)
    If wireguard interface is present on nodes, then cilium-agent upgrade will stuck. Upgrading the linux kernel to 6.8 is required.
 - **[cni-cilium]** Fixed conversion type bug in discovery cni exclusive hook. [#14995](https://github.com/deckhouse/deckhouse/pull/14995)
 - **[cni-cilium]** Enabled vlan-bpf-bypass feature to fix extra vlan interfaces issues. [#14606](https://github.com/deckhouse/deckhouse/pull/14606)
 - **[control-plane-manager]** Promoted etcd member if needed. [#14661](https://github.com/deckhouse/deckhouse/pull/14661)
 - **[control-plane-manager]** Made etcd client ignore HTTPS_PROXY settings. [#14504](https://github.com/deckhouse/deckhouse/pull/14504)
 - **[deckhouse]** Fixes bug when module`s config values unchanged after enabling. [#15887](https://github.com/deckhouse/deckhouse/pull/15887)
 - **[deckhouse]** Fixed a helm issue with patching arrays in deckhouse deployment. [#14599](https://github.com/deckhouse/deckhouse/pull/14599)
 - **[deckhouse-controller]** Fix conversions for external modules [#16891](https://github.com/deckhouse/deckhouse/pull/16891)
 - **[deckhouse-controller]** Fixed verifying migrated modules [#16873](https://github.com/deckhouse/deckhouse/pull/16873)
 - **[deckhouse-controller]** Fixed a crash during external module updates with conversions that caused ModuleRelease to fail validation due to a forbidden property error. [#16849](https://github.com/deckhouse/deckhouse/pull/16849)
 - **[deckhouse-controller]** fix deckhouse update accidentally minor skip [#16095](https://github.com/deckhouse/deckhouse/pull/16095)
 - **[deckhouse-controller]** Ensure `afterDeleteHelm` hooks receive Kubernetes snapshots by stopping monitors after hook execution [#15724](https://github.com/deckhouse/deckhouse/pull/15724)
 - **[deckhouse-controller]** Updated CEL rules, add x-deckhouse-validations documentation. [#14428](https://github.com/deckhouse/deckhouse/pull/14428)
 - **[dhctl]** Change default ssh mode [#15773](https://github.com/deckhouse/deckhouse/pull/15773)
 - **[dhctl]** Fixing bashible steps when it does not respond to the limit on the number of attempts [#15633](https://github.com/deckhouse/deckhouse/pull/15633)
 - **[dhctl]** Fix start and keep-alive behavior, do not change host if any kube-proxies has been started. [#15566](https://github.com/deckhouse/deckhouse/pull/15566)
 - **[dhctl]** Add timeout to ssh client dial and fix stop. [#15539](https://github.com/deckhouse/deckhouse/pull/15539)
 - **[dhctl]** Fix deadlock while reading dhctl singlethreaded server logs [#15512](https://github.com/deckhouse/deckhouse/pull/15512)
 - **[dhctl]** Fixed the behaviour of the attach operation when there are no keys in SSHConfig, only a password [#15478](https://github.com/deckhouse/deckhouse/pull/15478)
 - **[dhctl]** Fix ssh client initialising in Commander Attach and Commander Detach operations [#15380](https://github.com/deckhouse/deckhouse/pull/15380)
 - **[docs]** Added documentation for the new registry configuration in Containerd. [#14790](https://github.com/deckhouse/deckhouse/pull/14790)
 - **[extended-monitoring]** Exclude PVCs with block volume mode from space and inodes monitoring. [#14859](https://github.com/deckhouse/deckhouse/pull/14859)
    free space monitoring for the PVCs in the Block volumeMode is meaningless and will be disabled
 - **[ingress-nginx]** Enable ExecuteHookOnEvents on hook set_annotation_validation_suspended.go [#15839](https://github.com/deckhouse/deckhouse/pull/15839)
 - **[ingress-nginx]** Fixed nginx segfaults when opentelemetry is enabled. [#15466](https://github.com/deckhouse/deckhouse/pull/15466)
    The ingress-nginx pods of 1.10 will be restarted.
 - **[ingress-nginx]** Fixed the missing opentelemetry libraries issue. [#14965](https://github.com/deckhouse/deckhouse/pull/14965)
    The pods of Ingress Nginx controllers of 1.10 and 1.12 versions will be restated.
 - **[ingress-nginx]** Re-enabled validation and updated documentation. [#14368](https://github.com/deckhouse/deckhouse/pull/14368)
 - **[istio]** Fixed AuthorizationPolicy CRD insufficiency for Istio 1.25. [#16605](https://github.com/deckhouse/deckhouse/pull/16605)
 - **[istio]** Erroneous option in 1.25 control-plane helm template fixed. [#16412](https://github.com/deckhouse/deckhouse/pull/16412)
 - **[istio]** Reduce RAM for regenerate multicluster JWT token [#15328](https://github.com/deckhouse/deckhouse/pull/15328)
 - **[istio]** The metrics-exporter's template is fixed, it blocked the main queue if  `controlPlane.nodeSelector` setting was configured. [#15236](https://github.com/deckhouse/deckhouse/pull/15236)
 - **[istio]** Added api-proxy support for short-lived ServiceAccount tokens. [#14137](https://github.com/deckhouse/deckhouse/pull/14137)
 - **[loki]** Change stage to Preview [#15553](https://github.com/deckhouse/deckhouse/pull/15553)
 - **[metallb]** Fixed the Deckhouse controller queue freezing issue that occurs when Service was deleted, but child resource L2LBService wasn't. [#14966](https://github.com/deckhouse/deckhouse/pull/14966)
 - **[metallb]** Fixed IP pool exhaustion on LoadBalancer deletion. [#14315](https://github.com/deckhouse/deckhouse/pull/14315)
 - **[monitoring-deckhouse]** Fixed ModuleRelease alerts by eliminating many-to-many joins; observability alert now evaluates correctly. [#14908](https://github.com/deckhouse/deckhouse/pull/14908)
    Prometheus rules only; no component restarts. Alerts for module manual approval and unmet requirements will start firing as expected.
 - **[node-manager]** Enable SSHLegacyMode if SSHPrivateKey is not empty and fix gossh panic. [#15763](https://github.com/deckhouse/deckhouse/pull/15763)
 - **[node-manager]** CAPI crd served version fix [#15731](https://github.com/deckhouse/deckhouse/pull/15731)
 - **[node-manager]** shutdown inhibitor use three-state GracefulShutdownPostpone condition so kubelet waits for the final decision before ending pods [#15575](https://github.com/deckhouse/deckhouse/pull/15575)
 - **[node-manager]** CAPI crd served version fix [#15706](https://github.com/deckhouse/deckhouse/pull/15706)
 - **[node-manager]** Add rbac permission for getting DaemonSets for CAPI. [#15377](https://github.com/deckhouse/deckhouse/pull/15377)
 - **[node-manager]** Upgrade CAPI version and increase lease duration timeouts. [#15349](https://github.com/deckhouse/deckhouse/pull/15349)
 - **[node-manager]** Fixed calculation of memory for standby holder. [#14522](https://github.com/deckhouse/deckhouse/pull/14522)
 - **[node-manager]** Correct processing of the NodeUser in the bootstrap configuration. [#14151](https://github.com/deckhouse/deckhouse/pull/14151)
 - **[prometheus]** Fix securityContext indentation in the Prometheus main and longterm resources. [#15102](https://github.com/deckhouse/deckhouse/pull/15102)
    main and longterm Prometheuses will be rollout-restarted
 - **[prometheus]** Suppress Grafana-related alerts when the Grafana is disabled in the ModuleConfig. [#14981](https://github.com/deckhouse/deckhouse/pull/14981)
    default
 - **[prometheus]** Fix remote write dropping valid samples after restart due to missing series from snapshot. [#14849](https://github.com/deckhouse/deckhouse/pull/14849)
 - **[registrypackages]** Fixed permissions for directory with cni-plugins in PCI-DSS clusters [#16409](https://github.com/deckhouse/deckhouse/pull/16409)
 - **[user-authn]** Fix critical bug in password connector that caused login failures for LDAP, Crowd, and Keystone connectors. [#15370](https://github.com/deckhouse/deckhouse/pull/15370)
 - **[user-authn]** Fix critical bug in password connector that caused login failures for LDAP, Crowd, and Keystone connectors. [#15359](https://github.com/deckhouse/deckhouse/pull/15359)
 - **[user-authn]** Fix `ValidatingAdmissionPolicy` for `User` CR to skip password check on create. [#15269](https://github.com/deckhouse/deckhouse/pull/15269)
    Fixes a bug where creating new User resources failed due to missing oldObject in validation; password immutability is still enforced on update.
 - **[user-authz]** Rewrited user-authz module access hook from bash to Python. [#14695](https://github.com/deckhouse/deckhouse/pull/14695)
 - **[vertical-pod-autoscaler]** Disable vpa for k8s-metacollector in module runtime-audit-engine [#15659](https://github.com/deckhouse/deckhouse/pull/15659)

## Chore


 - **[candi]** Bump patch versions of Kubernetes images. [#14979](https://github.com/deckhouse/deckhouse/pull/14979)
    Kubernetes control-plane components will restart, kubelet will restart
 - **[candi]** Added support for new patch versions of Kubernetes. [#14834](https://github.com/deckhouse/deckhouse/pull/14834)
    Kubernetes control-plane components will restart, kubelet will restart
 - **[deckhouse]** Remove module weight constraints. [#15131](https://github.com/deckhouse/deckhouse/pull/15131)
 - **[deckhouse]** Made keepalived and network-policy-engine modules functional. [#14521](https://github.com/deckhouse/deckhouse/pull/14521)
 - **[deckhouse-controller]** Removed embedded pod-reloader module. The module was migrated and available as module from the `deckhouse` ModuleSource. [#14343](https://github.com/deckhouse/deckhouse/pull/14343)
 - **[dhctl]** Added native ssh client support to dhctl. [#13240](https://github.com/deckhouse/deckhouse/pull/13240)
 - **[ingress-nginx]** Removed mtls requirement from validating webhook. [#14862](https://github.com/deckhouse/deckhouse/pull/14862)
    All ingress-nginx controller pods will be restarted.
 - **[ingress-nginx]** Added a hook that add a finalizer on the IngressNginxController. [#13595](https://github.com/deckhouse/deckhouse/pull/13595)
 - **[istio]** Fixed public services metadata formatting. [#14887](https://github.com/deckhouse/deckhouse/pull/14887)
 - **[multitenancy-manager]** Added warning for namespace override. [#14874](https://github.com/deckhouse/deckhouse/pull/14874)
 - **[node-local-dns]** Disabled caching servfail responses. [#14836](https://github.com/deckhouse/deckhouse/pull/14836)
 - **[node-local-dns]** Updated the maximum and minimum TTL values for the success and denial parameters in the core dns cache settings. [#14345](https://github.com/deckhouse/deckhouse/pull/14345)

