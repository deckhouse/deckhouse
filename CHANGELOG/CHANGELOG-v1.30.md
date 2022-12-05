# Changelog v1.30

## [MALFORMED]


 - #394 unknown section "cluster-and-infustructure"
 - #473 unknown section "bashible"
 - #482 unknown section "okmener"
 - #485 unknown section "bashible"
 - #505 unknown section "global"
 - #523 unknown section "global"
 - #527 unknown section "global"
 - #546 unknown section "general"
 - #558 unknown section "global"
 - #562 unknown section "global"
 - #571 unknown section "bashible-apiserver"
 - #599 unknown section "monitoring"
 - #608 unknown section "bashible"
 - #634 unknown section "keepalived"
 - #639 unknown section "bashible"
 - #659 unknown section "documentation"
 - #664 unknown section "chore"
 - #679 unknown section "documentation"
 - #680 unknown section "documentation"
 - #683 unknown section "registry-packages"
 - #688 unknown section "documentation"
 - #709 unknown section "bashible"
 - #710 unknown section "monitoring"
 - #719 unknown section "registry-packages"
 - #729 unknown section "registry-packages"
 - #739 unknown section "bashible"
 - #744 unknown section "global"
 - #809 unknown section "general"

## Release digest


 - Disable enforced namespace passing to a query. If your metric need to be selected with a specific namespace label value, you should set it directly in an HPA's label selector
 - Ingress nginx controller pods now managed by a special hook, not by a Kubernetes Controller.

## Features


 - **[candi]** Support Astra Linux 1.7 [#847](https://github.com/deckhouse/deckhouse/pull/847)
 - **[cert-manager]** Actualize annotation to delete in the orphan secrets alert description [#587](https://github.com/deckhouse/deckhouse/pull/587)
 - **[cert-manager]** Support k8s v1.22 mutating admission for annotations-converter webhook [#554](https://github.com/deckhouse/deckhouse/pull/554)
 - **[chrony]** Disable ntp on nodes by custom bashible step. [#643](https://github.com/deckhouse/deckhouse/pull/643)
 - **[control-plane-manager]** Add basic audit-policy. [#467](https://github.com/deckhouse/deckhouse/pull/467)
    Due to the new basic audit-policy api-server component will be restarted.
 - **[deckhouse]** Check requirements before applying a DeckhouseRelease [#598](https://github.com/deckhouse/deckhouse/pull/598)
 - **[deckhouse]** Different severity level based on pending DeckhouseReleases count [#439](https://github.com/deckhouse/deckhouse/pull/439)
 - **[deckhouse]** Add alert if deckhouse config is broken [#430](https://github.com/deckhouse/deckhouse/pull/430)
 - **[deckhouse]** Add canary deckhouse release update [#429](https://github.com/deckhouse/deckhouse/pull/429)
 - **[dhctl]** Add a templating feature for Kubernetes resources сreated by dhctl. [#776](https://github.com/deckhouse/deckhouse/pull/776)
 - **[extended-monitoring]** Add cert-exporter alerts [#512](https://github.com/deckhouse/deckhouse/pull/512)
    Added alerts to track certificates expiration and cert-exporter health
 - **[extended-monitoring]** Add cert-exporter [#479](https://github.com/deckhouse/deckhouse/pull/479)
    Added cert-exporter to track certificates expiration
 - **[flant-integration]** Add madison-proxy notification channel to send alert from grafana to madison via proxy and show them in Polk [#402](https://github.com/deckhouse/deckhouse/pull/402)
    Add rewrite rule to madison-proxy from /api/v1/alerts url to madison url, because grafana always send notification to this url.
 - **[ingress-nginx]** Add panels to Grafana dashboards with detailed nginx statistic [#689](https://github.com/deckhouse/deckhouse/pull/689)
 - **[ingress-nginx]** Add documentation article "How to enable HorizontalPodAutoscaling for IngressNginxController". [#648](https://github.com/deckhouse/deckhouse/pull/648)
 - **[ingress-nginx]** Add an example of usage LoadBalancer inlet with MetalLB. [#465](https://github.com/deckhouse/deckhouse/pull/465)
 - **[ingress-nginx]** Add ingress-nginx controller version 1.0 [#394](https://github.com/deckhouse/deckhouse/pull/394)
 - **[istio]** nodeSelector and tolerations customization for control-plane [#1034](https://github.com/deckhouse/deckhouse/pull/1034)
 - **[istio]** `alliance.ingressGateway.nodePort.port` option to set a static port for NodePort-type ingressgateway Service. [#575](https://github.com/deckhouse/deckhouse/pull/575)
 - **[local-path-provisioner]** Added reclaimPolicy selector, set default reclaimPolicy to Retain [#561](https://github.com/deckhouse/deckhouse/pull/561)
 - **[monitoring-kubernetes]** Added ebpf-exporter [#387](https://github.com/deckhouse/deckhouse/pull/387)
    ebpf-exporter that monitors global and per-cgroup OOMs. With recording rules and dashboard.
 - **[monitoring-kubernetes-control-plane]** Add sorted tables for kube-apiserver metrics. [#626](https://github.com/deckhouse/deckhouse/pull/626)
 - **[namespace-configurator]** New namespace-configurator module [#435](https://github.com/deckhouse/deckhouse/pull/435)
    namespace-configurator module allows to assign annotations and labels to namespaces automatically
 - **[node-manager]** Update NodeUser resource to support NodeGroup selector and multiple ssh keys. [#595](https://github.com/deckhouse/deckhouse/pull/595)
 - **[node-manager]** Added Early OOM killer [#387](https://github.com/deckhouse/deckhouse/pull/387)
    Primitive early OOM that prevents nodes from getting stuck in out-of-memory conditions. Triggers when MemAvailable becomes less than 500 MiB.
 - **[okmeter]** Okmeter agent image will be checked periodically by tag and used sha256 hash to pin the image for agent. [#556](https://github.com/deckhouse/deckhouse/pull/556)
 - **[prometheus]** Add supporting ServiceMonitors and PodMonitors from user-space [#604](https://github.com/deckhouse/deckhouse/pull/604)
    Prometheus will be restarted
 - **[prometheus]** Provisioning alerts channels from CRD's to grafana via new secret. Migrate to direct datasources. [#402](https://github.com/deckhouse/deckhouse/pull/402)
    Grafana will be restarted.
    Now grafana using direct (proxy) type for deckhouse datasources (main, longterm, uncached), because direct(browse) datasources type is depreated now. And alerts don't work with direct data sources.
    Provisioning datasources from secret instead configmap. Deckhouse datasources need client certificates to connect to  prometheus or trickter. Old cm leave to prevent mount error while terminating.
 - **[prometheus-crd]** Add GrafanaAlertsChannel CRD. [#402](https://github.com/deckhouse/deckhouse/pull/402)
    Support only prometheus alert manager notification channel
 - **[user-authn]** Add an OpenAPI spec to validate Deckhouse configuration parameters for the user-authn module. [#593](https://github.com/deckhouse/deckhouse/pull/593)
 - **[user-authn]** Validation webhook for preventing duplicate DexAuthenticators to be created. [#530](https://github.com/deckhouse/deckhouse/pull/530)
 - **[user-authn]** Update oauth2-proxy to the latest version (7.2.0) [#368](https://github.com/deckhouse/deckhouse/pull/368)
    Dex Authenticators will be restarted

## Fixes


 - **[candi]** Fix update password hash for node user. [#1192](https://github.com/deckhouse/deckhouse/pull/1192)
 - **[candi]** Automatically discover zone for volumes in OpenStack [#1104](https://github.com/deckhouse/deckhouse/pull/1104)
 - **[candi]** Fix docker-stuck-containers-cleaner unit [#1044](https://github.com/deckhouse/deckhouse/pull/1044)
 - **[candi]** Proper work with astra bundle in EE/FE. [#868](https://github.com/deckhouse/deckhouse/pull/868)
 - **[candi]** Fix centos distro version detection [#857](https://github.com/deckhouse/deckhouse/pull/857)
 - **[candi]** Speed up reboot master node on cluster bootstrap. [#800](https://github.com/deckhouse/deckhouse/pull/800)
 - **[candi]** Fix nodeuser script. [#751](https://github.com/deckhouse/deckhouse/pull/751)
 - **[candi]** Fix nodeuser creation script. [#749](https://github.com/deckhouse/deckhouse/pull/749)
 - **[candi]** Fix kubelet slow start on reboot. [#742](https://github.com/deckhouse/deckhouse/pull/742)
 - **[cert-manager]** Disable legacy cert-manager for >= 1.22 kubernetes [#551](https://github.com/deckhouse/deckhouse/pull/551)
    Legacy cert-manager resources (`certmanager.k8s.io`) will not be supported in 1.22+ clusters
 - **[chrony]** Bashible step fix — missed openntpd.service and time-sync.target in list. [#653](https://github.com/deckhouse/deckhouse/pull/653)
 - **[chrony]** Add VPA label `workload-resource-policy` to make it take part in resources requests calculations. [#455](https://github.com/deckhouse/deckhouse/pull/455)
 - **[cloud-provider-aws]** Fixed zone selection for bastion in WithNAT layout [#1021](https://github.com/deckhouse/deckhouse/pull/1021)
 - **[cloud-provider-aws]** Documentation fixes. [#401](https://github.com/deckhouse/deckhouse/pull/401)
 - **[cloud-provider-openstack]** Set volume availability zone in dhctl on bootstrap [#1033](https://github.com/deckhouse/deckhouse/pull/1033)
 - **[cloud-provider-vsphere]** Install latest version of open-vm-tools [#667](https://github.com/deckhouse/deckhouse/pull/667)
 - **[control-plane-manager]** LoadBalancer annotations are able to be set [#567](https://github.com/deckhouse/deckhouse/pull/567)
 - **[deckhouse]** Fix cleanup DeckhouseReleases hook [#1168](https://github.com/deckhouse/deckhouse/pull/1168)
 - **[deckhouse]** The more controlled and transparent release process. [#699](https://github.com/deckhouse/deckhouse/pull/699)
 - **[deckhouse]** Update the description of the release process [#660](https://github.com/deckhouse/deckhouse/pull/660)
 - **[deckhouse]** Fix requirements check semver lib [#658](https://github.com/deckhouse/deckhouse/pull/658)
 - **[deckhouse]** The start and end times of the update window must belong to the same day. [#496](https://github.com/deckhouse/deckhouse/pull/496)
 - **[deckhouse]** Use scrape interval x2 instead of hardcoded value for invalid config values alerting [#493](https://github.com/deckhouse/deckhouse/pull/493)
 - **[deckhouse-controller]** Update addon-operator for working with FeatureGates helm validation [#1169](https://github.com/deckhouse/deckhouse/pull/1169)
 - **[dhctl]** Do not print error about not existing bastion host key for abort command. [#655](https://github.com/deckhouse/deckhouse/pull/655)
 - **[dhctl]** Check deckhouse pod readiness before get logs. It fixes static cluster bootstrap. [#571](https://github.com/deckhouse/deckhouse/pull/571)
 - **[dhctl]** All master nodes will have `control-plane` role in new clusters. [#562](https://github.com/deckhouse/deckhouse/pull/562)
 - **[docs]** Updated the Supported OS versions list format. [#1126](https://github.com/deckhouse/deckhouse/pull/1126)
 - **[docs]** Getting started with Azure minor updates. [#698](https://github.com/deckhouse/deckhouse/pull/698)
 - **[docs]** Fix instructions for switching registry and image copier [#533](https://github.com/deckhouse/deckhouse/pull/533)
 - **[extended-monitoring]** CronJobFailed alert bugfix. [#489](https://github.com/deckhouse/deckhouse/pull/489)
 - **[flant-integration]** Getting rid of deprecated `flantIntegration.kubeall.team` config value spec. [#695](https://github.com/deckhouse/deckhouse/pull/695)
 - **[flant-integration]** Remove "kubeall.team" field from the `deckhouse` ConfigMap. [#673](https://github.com/deckhouse/deckhouse/pull/673)
 - **[flant-integration]** Remove the plan parameter from the OpenAPI specification [#486](https://github.com/deckhouse/deckhouse/pull/486)
 - **[flant-integration]** Implement proper HA remote-write and reduce outgoing traffic amount. [#412](https://github.com/deckhouse/deckhouse/pull/412)
 - **[helm]** Add deprecation guide link to deprecated resources alerts. [#678](https://github.com/deckhouse/deckhouse/pull/678)
 - **[helm]** Provide an actual description for deprecated resources API versions alerts. [#569](https://github.com/deckhouse/deckhouse/pull/569)
 - **[ingress-nginx]** Manual update for ingress controllers [#921](https://github.com/deckhouse/deckhouse/pull/921)
    Ingress nginx controller pods now managed by a special hook, not by a Kubernetes Controller.
 - **[ingress-nginx]** Fix handled request query on a dashboard. [#871](https://github.com/deckhouse/deckhouse/pull/871)
 - **[ingress-nginx]** temporary remove support of 1.0 controller [#782](https://github.com/deckhouse/deckhouse/pull/782)
 - **[ingress-nginx]** Added "pcre_jit on" to nginx.tmpl for controller-0.46 and above [#515](https://github.com/deckhouse/deckhouse/pull/515)
    Ingress Controller >= 0.46 will be restarted
 - **[ingress-nginx]** Set proper version for new ingress-nginx controller 1.0 (drop the patch version). [#480](https://github.com/deckhouse/deckhouse/pull/480)
 - **[ingress-nginx]** Always return auth request cookies (only for controllers >= 0.33) [#368](https://github.com/deckhouse/deckhouse/pull/368)
    Ingress Nginx controllers >=0.33 pods will be restarted
 - **[istio]** The `istio.tracing.kiali.jaegerURLForUsers` parameter bugfix. [#1196](https://github.com/deckhouse/deckhouse/pull/1196)
 - **[istio]** Correct decision to deploy ingressgateway for multiclusters. [#640](https://github.com/deckhouse/deckhouse/pull/640)
 - **[istio]** `globalVersion` option clarification in documentation. [#584](https://github.com/deckhouse/deckhouse/pull/584)
 - **[local-path-provisioner]** Update local-path-provisioner v0.0.21, include fix [#478](https://github.com/deckhouse/deckhouse/pull/478)
    Protect PVs to be reused in case of unmounted storage.
 - **[log-shipper]** Add VPA label `workload-resource-policy` to make it take part in resources requests calculations. [#455](https://github.com/deckhouse/deckhouse/pull/455)
 - **[monitoring-kubernetes]** Add the container="POD" label to all advisor metrics for pause containers. [#960](https://github.com/deckhouse/deckhouse/pull/960)
 - **[monitoring-kubernetes]** Filter VPA by actual controllers to calculate VPA coverage [#459](https://github.com/deckhouse/deckhouse/pull/459)
 - **[monitoring-kubernetes]** Fixed node-exporter apparmor profile. [#457](https://github.com/deckhouse/deckhouse/pull/457)
 - **[node-manager]** Fix event creation for NG when new Machine provisioning process is failed [#757](https://github.com/deckhouse/deckhouse/pull/757)
 - **[node-manager]** Do not deploy VPA for bashible-apiserver if autoscaler is not enabled [#708](https://github.com/deckhouse/deckhouse/pull/708)
 - **[node-manager]** FAQ bootstrap and adopt clarification. [#585](https://github.com/deckhouse/deckhouse/pull/585)
 - **[node-manager]** When calculating maximum instances for particular NodeGroup without zones defined — use global zones count from CloudProvider configuration. [#580](https://github.com/deckhouse/deckhouse/pull/580)
 - **[node-manager]** Fix Static node template annotations updating [#544](https://github.com/deckhouse/deckhouse/pull/544)
 - **[prometheus]** Make Grafana home dashboard queries to only show the top-used versions [#476](https://github.com/deckhouse/deckhouse/pull/476)
 - **[prometheus-metrics-adapter]** Restore HPA external metrics behavior [#1154](https://github.com/deckhouse/deckhouse/pull/1154)
    Disable enforced namespace passing to a query. If your metric need to be selected with a specific namespace label value, you should set it directly in an HPA's label selector
 - **[upmeter]** Fixed floating bug causing false downtime of deckhouse/cluster-configuration probe [#997](https://github.com/deckhouse/deckhouse/pull/997)
 - **[upmeter]** Assigned limited access rights to the agent serviceaccount [#469](https://github.com/deckhouse/deckhouse/pull/469)
 - **[user-authn]** Fixed .spec.ldap.bindPW escaping in DexProvider [#1032](https://github.com/deckhouse/deckhouse/pull/1032)
 - **[user-authn]** Migrate BitbucketCloud connector to utilizing workspaces API. [#738](https://github.com/deckhouse/deckhouse/pull/738)
 - **[user-authn]** Fix values scheme. [#676](https://github.com/deckhouse/deckhouse/pull/676)
 - **[user-authn]** Ignore updating an existing DexAuthenticator [#539](https://github.com/deckhouse/deckhouse/pull/539)
 - **[user-authn]** Delete publish API secrets with not matching names to avoid the orphaned secrets alerts [#472](https://github.com/deckhouse/deckhouse/pull/472)
 - **[user-authz]** Allow empty group and apiVersion requests in user-authz webhook [#526](https://github.com/deckhouse/deckhouse/pull/526)

