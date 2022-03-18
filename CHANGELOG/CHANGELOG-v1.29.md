# Changelog v1.29

## [MALFORMED]


 - #354 missing section, missing summary, missing type, unknown section ""
 - #371 unknown section "documentation"
 - #393 unknown section "‎ingress-nginx‎"
 - #422 unknown section "bashible"
 - #463 unknown section "docs,control-plane-manager"
 - #521 unknown section "bashible"
 - #568 unknown section "bashible"
 - #617 unknown section "documentation"

## Features


 - **[cert-manager]** Upgrade cert-manager to v1.6.1 [#398](https://github.com/deckhouse/deckhouse/pull/398)
    The cert-manager controller will be restarted. CRD with version  is no longer supported.
 - **[cert-manager]** Instructions for connecting Vault to the . [#374](https://github.com/deckhouse/deckhouse/pull/374)
 - **[deckhouse]** Cleanup deckhouse Outdated releases (> 10) [#573](https://github.com/deckhouse/deckhouse/pull/573)
 - **[docs]** Add documentation on using Harbor as a third-party registry. [#565](https://github.com/deckhouse/deckhouse/pull/565)
 - **[istio]** Great module refactoring [#357](https://github.com/deckhouse/deckhouse/pull/357)
 - **[log-shipper]** Support storing data in Elasticsearch datastreams. [#372](https://github.com/deckhouse/deckhouse/pull/372)
 - **[node-manager]** Add Pods deletion from a node that requests disruption updates, when pod eviction fails. [#367](https://github.com/deckhouse/deckhouse/pull/367)
 - **[prometheus]** Improve Prometheus FAQ about Lens access. [#406](https://github.com/deckhouse/deckhouse/pull/406)
 - **[secret-copier]** Implement create–or–update logic for proper reconcile. [#411](https://github.com/deckhouse/deckhouse/pull/411)
    Add support of namespace label-selector in  annotation value.
 - **[upmeter]** Assign more specific nodes for the server pod [#351](https://github.com/deckhouse/deckhouse/pull/351)
 - **[user-authn]** Add the doc about Dex rate limit [#352](https://github.com/deckhouse/deckhouse/pull/352)
 - **[user-authz]** Add the doc about how cluster authorization rules are combined [#342](https://github.com/deckhouse/deckhouse/pull/342)

## Fixes


 - **[chrony]** Fix rollout restart time of chrony daemonset. [#364](https://github.com/deckhouse/deckhouse/pull/364)
    The module will be restarted.
 - **[deckhouse]** Clear values cache when a module is disabled. [#416](https://github.com/deckhouse/deckhouse/pull/416)
 - **[deckhouse]** Move context generation into a bashible-apiserver. [#375](https://github.com/deckhouse/deckhouse/pull/375)
    A bashible-apiserver will be restarted.
 - **[deckhouse]** Fix Deckhouse Manual update mode. [#362](https://github.com/deckhouse/deckhouse/pull/362)
 - **[deckhouse-web]** Add missing 'ca.crt' field to internal values schema. [#518](https://github.com/deckhouse/deckhouse/pull/518)
 - **[istio]** Missing customCertificateData in openapi fix. [#563](https://github.com/deckhouse/deckhouse/pull/563)
 - **[log-shipper]** Fix default CRD values. [#520](https://github.com/deckhouse/deckhouse/pull/520)
    CR , created in , should be recreated.
 - **[monitoring-kubernetes]** Fix description for alert . [#456](https://github.com/deckhouse/deckhouse/pull/456)
    We only use the Deckhouse chrony module, so a description about another NTP daemons is not needed.
 - **[prometheus]** Migrate Grafana old tables to new and replace from __cell variable to __value and add a time interval to URL. [#532](https://github.com/deckhouse/deckhouse/pull/532)
 - **[prometheus]** Bump Grafana version to fix zero-day path traversal bug (CVE-2021-43798). [#421](https://github.com/deckhouse/deckhouse/pull/421)
 - **[upmeter]** Re-create pods which change their availability zones by re-creating corresponding StatefulSets [#350](https://github.com/deckhouse/deckhouse/pull/350)
    Accidentally, fix PVC re-creation by avoiding a race with kube-controller-manager. Fixes #281
 - **[upmeter]** HTTP probe status is "down" when it cannot connect to endpoint, instead of "unknown" [#349](https://github.com/deckhouse/deckhouse/pull/349)
    Unavailable Prometheus is not considered "up" anymore, like everything else that depends on it
 - **[user-authn]** Fixed secret name in crowd-proxy deployment. [#559](https://github.com/deckhouse/deckhouse/pull/559)
    Fixed bug when kubernetes-api certificate had differrent name from crowd-proxy certificate name.

