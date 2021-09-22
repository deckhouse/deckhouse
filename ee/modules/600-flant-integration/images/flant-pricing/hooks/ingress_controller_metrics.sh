#!/bin/bash -e

# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

for f in $(find /frameworks/shell/ -type f -iname "*.sh"); do
  source $f
done

function __config__() {
  cat << EOF
    configVersion: v1
    kubernetes:
    - name: controllers
      group: main
      queue: /ingress_controller_metrics
      keepFullObjectsInMemory: false
      waitForSynchronization: false
      apiVersion: deckhouse.io/v1
      kind: IngressNginxController
      jqFilter: |
        {
          "name": .metadata.name,
          "version": (.spec.controllerVersion // "default"),
          "inlet": .spec.inlet
        }
    - name: daemonsets
      group: main
      queue: /ingress_controller_metrics
      apiVersion: apps/v1
      kind: DaemonSet
      keepFullObjectsInMemory: false
      namespace:
        nameSelector:
          matchNames: [d8-ingress-nginx]
      labelSelector:
        matchExpressions:
        - key: heritage
          operator: In
          values: ["deckhouse"]
        - key: module
          operator: In
          values: ["ingress-nginx"]
        - key: app
          operator: In
          values: ["controller"]
        - key: ingress-nginx-failover
          operator: DoesNotExist
      jqFilter: |
        {
          "controllerName": .metadata.labels.name,
          "controllerVersion": .metadata.annotations."ingress-nginx-controller.deckhouse.io/controller-version",
          "controllerInlet": .metadata.annotations."ingress-nginx-controller.deckhouse.io/inlet",
          "readyPodCount": .status.numberReady
        }
    - name: deployments
      group: main
      queue: /ingress_controller_metrics
      apiVersion: apps/v1
      kind: Deployment
      keepFullObjectsInMemory: false
      namespace:
        nameSelector:
          matchNames: [d8-ingress-nginx]
      labelSelector:
        matchExpressions:
        - key: heritage
          operator: In
          values: ["deckhouse"]
        - key: module
          operator: In
          values: ["ingress-nginx"]
        - key: app
          operator: In
          values: ["controller"]
      jqFilter: |
        {
          "controllerName": .metadata.labels.name,
          "controllerVersion": .metadata.annotations."ingress-nginx-controller.deckhouse.io/controller-version",
          "controllerInlet": .metadata.annotations."ingress-nginx-controller.deckhouse.io/inlet",
          "readyPodCount": .status.readyReplicas
        }
EOF
}

function __main__() {
  controllers_count_metric_name="flant_pricing_ingress_nginx_controllers_count"
  pod_count_metric_name="flant_pricing_ingress_nginx_controllers_pod_count"
  group="group_ingress_controller_metrics"
  jq -c --arg group "$group" '.group = $group' <<< '{"action":"expire"}' >> $METRICS_PATH

  default_controller_version=""

  controllers="$(context::jq -c '
    .snapshots.controllers // [] |
    map({
      "name": .filterResult.name,
      "version": .filterResult.version,
      "inlet": .filterResult.inlet,
      "hash": (.filterResult.version + .filterResult.inlet)
    }) |
    group_by(.hash) |
    map({
      "name": .[0].name,
      "version": .[0].version,
      "inlet": .[0].inlet,
      "count": length
    }) | .[]')"
  for controller in $controllers; do
    controller_name="$(jq -r '.name' <<< "$controller")"
    controller_version="$(jq -r '.version' <<< "$controller")"
    controller_inlet="$(jq -r '.inlet' <<< "$controller")"

    deployed_controller_version="$(context::jq -r --arg controller_name "$controller_name" '
      (.snapshots.daemonsets // []) + (.snapshots.deployments // []) | .[] |
      select(.filterResult.controllerName == $controller_name) |
      .filterResult.controllerVersion // ""')"
    if [[ "$controller_version" == "default" ]]; then
      default_controller_version="$deployed_controller_version"
    fi
    jq -c --arg metric_name "$controllers_count_metric_name" --arg group "$group" \
      --arg deployed_controller_version "$deployed_controller_version" '
      {
        "name": $metric_name,
        "group": $group,
        "set": .count,
        "labels":
        {
          "inlet": .inlet,
          "version": (if $deployed_controller_version == "" then .version else $deployed_controller_version end),
          "default": (.version == "default") | tostring
        }
      }
      ' <<< "$controller" >> $METRICS_PATH
  done

  # Set versions from IngressNginxController to figure out
  # which deployed controller uses the default version.
  deployed_controllers_with_default_version="$(context::jq -c '
    .snapshots as $snapshots | ($snapshots.daemonsets // []) + ($snapshots.deployments // []) |
    map({
        "controllerVersion": (
          .filterResult as $deployed_controller |
          [
            $snapshots.controllers // [] | .[] |
            select(.filterResult.name == $deployed_controller.controllerName) |
            .filterResult.version
          ] | first // $deployed_controller.controllerVersion
        ),
        "controllerInlet": .filterResult.controllerInlet,
        "readyPodCount": .filterResult.readyPodCount
    })')"

  deployed_controllers="$(jq -c '
    map({
        "controllerVersion": .controllerVersion,
        "controllerInlet": .controllerInlet,
        "readyPodCount": .readyPodCount,
        "hash": (.controllerVersion + .controllerInlet)
    }) |
    group_by(.hash) |
    map({
      "controllerVersion": .[0].controllerVersion,
      "controllerInlet": .[0].controllerInlet,
      "readyPodCount": [.[].readyPodCount] | add
    }) | .[]' <<< "$deployed_controllers_with_default_version")"
  for deployed_controller in $deployed_controllers; do
    jq -c --arg metric_name "$pod_count_metric_name" --arg group "$group" \
      --arg default_controller_version "$default_controller_version" '
      {
        "name": $metric_name,
        "group": $group,
        "set": .readyPodCount,
        "labels":
        {
          "inlet": .controllerInlet,
          "version": (if .controllerVersion == "default" then $default_controller_version else .controllerVersion end),
          "default": (.controllerVersion == "default") | tostring
        }
      }
      ' <<< "$deployed_controller" >> $METRICS_PATH
  done
}

hook::run "$@"
