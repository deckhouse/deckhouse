# Changelog v1.75

## Know before update


 - Will restart all d8 pods on dkp release with this changes.
 - mode auto is deprecated and will be removed in a future API version. Use explicit modes like "Recreate", "Initial", or "InPlaceOrRecreate" instead.

## Features


 - **[admission-policy-engine]** add policy to deny exec/attach to pods with heritage deckhouse label [#16749](https://github.com/deckhouse/deckhouse/pull/16749)
 - **[candi]** Add annotation for node by creating converger user. [#16734](https://github.com/deckhouse/deckhouse/pull/16734)
 - **[cloud-provider-dvp]** clarify CSI errors [#16434](https://github.com/deckhouse/deckhouse/pull/16434)
 - **[control-plane-manager]** Implement etcd-arbiter mode for HA capability with less resources. This will allow to bootstrap only etcd node without control-plane components. [#16716](https://github.com/deckhouse/deckhouse/pull/16716)
    Will restart all d8 pods on dkp release with this changes.
 - **[deckhouse-controller]** add Application statistics logic [#16809](https://github.com/deckhouse/deckhouse/pull/16809)
 - **[descheduler]** update descheduler to the 1.34 version. 
    Descheduler evicts pods with a larger restart count first it should make workload balancing in the cluster more stable.
    Descheduler respects DRA resources. [#16846](https://github.com/deckhouse/deckhouse/pull/16846)
 - **[dhctl]** Wait for converger user will be presented on all master nodes. [#16734](https://github.com/deckhouse/deckhouse/pull/16734)
 - **[vertical-pod-autoscaler]** update vpa module to 1.5.1 [#16814](https://github.com/deckhouse/deckhouse/pull/16814)
    mode auto is deprecated and will be removed in a future API version. Use explicit modes like "Recreate", "Initial", or "InPlaceOrRecreate" instead.

## Fixes


 - **[candi]** bashible events generateName [#16768](https://github.com/deckhouse/deckhouse/pull/16768)
 - **[deckhouse]** validate deckhouse-registry secret for spaces or newlines. [#16101](https://github.com/deckhouse/deckhouse/pull/16101)
 - **[deckhouse-controller]** Rollback nelm version [#16770](https://github.com/deckhouse/deckhouse/pull/16770)
 - **[deckhouse-controller]** Remove track-termination-mode notation [#16612](https://github.com/deckhouse/deckhouse/pull/16612)
 - **[dhctl]** Fix dhctl bootstrap-phase abort running after dhctl bootstrap-phasebase-infra. [#16829](https://github.com/deckhouse/deckhouse/pull/16829)
 - **[dhctl]** fix kube token handling [#16735](https://github.com/deckhouse/deckhouse/pull/16735)
 - **[docs]** Fix registry-modules-watcher deleting all documentation when registry returns an error [#16771](https://github.com/deckhouse/deckhouse/pull/16771)
 - **[node-manager]** CAPS logs noise reduction [#16805](https://github.com/deckhouse/deckhouse/pull/16805)

## Chore


 - **[candi]** enrich static nodes with topology labels via /var/lib/node_labels [#16816](https://github.com/deckhouse/deckhouse/pull/16816)
 - **[prometheus]** Add new rules for grafana dashboards [#15865](https://github.com/deckhouse/deckhouse/pull/15865)

