#!/bin/bash -e

for f in $(find /frameworks/shell/ -type f -iname "*.sh"); do
  source $f
done

function __config__() {
  # NOTE: If you are changing crontab frequency - please change a time duration
  # in PromQL "ingress_nginx_overall_requests_total[20m]" in `rps_metrics()` below.
  cat << EOF
    configVersion: v1
    onStartup: 100
    schedule:
    - group: main
      queue: /modules/$(module::name::kebab_case)
      crontab: "*/20 * * * *"
EOF
}

# Makes query to prometheus and returns resulting json
# $1 - promql
function prometheus_query() {
  curl_args=(-s --connect-timeout 10 --max-time 10 -k -XGET -G -k --cert /etc/ssl/prometheus-api-client-tls/tls.crt --key /etc/ssl/prometheus-api-client-tls/tls.key)
  prom_url="https://prometheus.d8-monitoring:9090/api/v1/query"
  if ! prom_result="$(curl "${curl_args[@]}" "${prom_url}" --data-urlencode "${1}")"; then
    prom_result=""
  fi
  echo "$prom_result"
}

function get_cluster_status() {
  if [[ $1 == "" ]]; then
    echo "error"
    return 0
  fi
  echo "$1" | jq -r '[.data.result // [] | .[] | .metric.status] | sort | first // "missing"'
}

function get_node_group_status() {
  if [[ $2 == "" || $3 == ""  || $4 == "" ]]; then
    echo "error"
    return 0
  fi

  node_group_name="$1"
  node_group_statuses="$2"
  node_statuses="$3"
  node_template_statuses="$4"

  node_group_status=$(jq -r --arg node_group_name "$node_group_name" '
    [.data.result // [] | .[] | select(.metric.name == $node_group_name) | .metric.status] | sort | first // ""
    ' <<< "$node_group_statuses")

  node_status=$(jq -r --arg node_group_name "$node_group_name" '
    [.data.result // [] | .[] | select(.metric.node_group == $node_group_name) | .metric.status] | sort | first // ""
    ' <<< "$node_statuses")

  node_template_status=$(jq -r --arg node_group_name "$node_group_name" '
    [.data.result // [] | .[] | select(.metric.name == $node_group_name) | .metric.status] | sort | first // ""
    ' <<< "$node_template_statuses")

  status="changed"
  if [[ "$node_group_status" == "ok" && "$node_status" == "ok"  && "$node_template_status" == "ok" ]]; then
    status="ok"
  else
    if [[ -z "$node_template_status" ]]; then
      status="missing"
    else
      if jq -e --arg node_group_name "$node_group_name" '[.data.result // [] | .[] | select(.metric.node_group == $node_group_name and .metric.status == "destructively_changed")] | any' <<< "$node_statuses" > /dev/null; then
        status="destructively_changed"
      fi
    fi
  fi
  echo $status
}

function terraform_state_metrics() {
  summarized_metric_name="flant_pricing_terraform_state"
  node_group_metric_name="flant_pricing_terraform_state_node_group"
  group="group_terraform_state_metrics"
  jq -c --arg group "$group" '.group = $group' <<< '{"action":"expire"}' >> $METRICS_PATH

  tf_state_cluster="none"
  tf_state_master="none"
  tf_state_terranode="none"

  if [[ "${FP_TERRAFORM_MANAGER_EBABLED}" == "true" ]]; then
    prom_result=$(prometheus_query 'query=max(candi_converge_cluster_status) by (status) == 1')
    tf_state_cluster=$(get_cluster_status "$prom_result")

    node_group_statuses=$(prometheus_query 'query=max(candi_converge_node_group_status) by (name,status) == 1')
    node_statuses=$(prometheus_query 'query=max(candi_converge_node_status) by (name,node_group,status) == 1')
    node_template_statuses=$(prometheus_query 'query=max(candi_converge_node_template_status) by (name,status) == 1')

    tf_state_master="missing"
    for node_group_name in $(jq -r '.data.result[] | .metric.name' <<< "$node_group_statuses"); do
      status=$(get_node_group_status "$node_group_name" "$node_group_statuses" "$node_statuses" "$node_template_statuses")
      if [[ "$node_group_name" == "master" ]]; then
        tf_state_master="$status"
      else
        jq -nc --arg metric_name $node_group_metric_name --arg group "$group" \
          --arg node_group_name "$node_group_name" \
          --arg status "$status" '
          {
            "name": $metric_name,
            "group": $group,
            "set": '$(date +%s)',
            "labels": {
              "name": $node_group_name,
              "status": $status
            }
          }
          ' >> $METRICS_PATH

        if [[ "$tf_state_terranode" == "none" ]]; then
          tf_state_terranode="$status"
        elif [[ "$status" != "ok" && "$tf_state_terranode" != "destructively_changed" ]]; then
          tf_state_terranode="$status"
        fi
      fi
    done
  fi

  jq -nc --arg metric_name $summarized_metric_name --arg group "$group" \
    --arg tf_state_cluster "$tf_state_cluster" \
    --arg tf_state_master "$tf_state_master" \
    --arg tf_state_terranode "$tf_state_terranode" '
    {
      "name": $metric_name,
      "group": $group,
      "set": '$(date +%s)',
      "labels": {
        "cluster": $tf_state_cluster,
        "master": $tf_state_master,
        "terranode": $tf_state_terranode
      }
    }
    ' >> $METRICS_PATH
}

function helm_releases_metrics() {
  helm_releases_metric_name="flant_pricing_helm_releases_count"
  group="group_helm_releases_metrics"
  jq -c --arg group "$group" '.group = $group' <<< '{"action":"expire"}' >> $METRICS_PATH

  prom_result=$(prometheus_query 'query=helm_releases_count')
  if [[ ! -z "$prom_result" ]]; then
    jq --arg metric_name $helm_releases_metric_name --arg group "$group" '
      .data.result[] |
      {
        "name": $metric_name,
        "group": $group,
        "set": (.value[1] | tonumber),
        "labels": {
          "helm_version": .metric.helm_version
        }
      }
      ' <<< "$prom_result" >> $METRICS_PATH
  fi
}

function resources_metrics() {
  resources_metric_name="flant_pricing_resources_count"
  group="group_resources_metrics"
  jq -c --arg group "$group" '.group = $group' <<< '{"action":"expire"}' >> $METRICS_PATH

  metrics=""

  prom_result=$(prometheus_query 'query=count(sum(kube_pod_container_status_ready) by (pod))')
  if [[ ! -z "$prom_result" ]]; then
    metrics="$metrics\n$(jq --arg metric_name $resources_metric_name --arg group "$group" '.data.result[] |
    {
      "name": $metric_name,
      "group": $group,
      "set": (.value[1] | tonumber),
      "labels": {
        "kind": "Pod"
      }
    }
    ' <<< "$prom_result")"
  fi

  prom_result=$(prometheus_query 'query=count(kube_namespace_created)')
  if [[ ! -z "$prom_result" ]]; then
    metrics="$metrics\n$(jq --arg metric_name $resources_metric_name --arg group "$group" '.data.result[] |
    {
      "name": $metric_name,
      "group": $group,
      "set": (.value[1] | tonumber),
      "labels": {
        "kind": "Namespace"
      }
    }
    ' <<< "$prom_result")"
  fi

  prom_result=$(prometheus_query 'query=count(kube_service_created)')
  if [[ ! -z "$prom_result" ]]; then
    metrics="$metrics\n$(jq --arg metric_name $resources_metric_name --arg group "$group" '.data.result[] |
    {
      "name": $metric_name,
      "group": $group,
      "set": (.value[1] | tonumber),
      "labels": {
        "kind": "Service"
      }
    }
    ' <<< "$prom_result")"
  fi

  prom_result=$(prometheus_query 'query=count(kube_ingress_created)')
  if [[ ! -z "$prom_result" ]]; then
    metrics="$metrics\n$(jq --arg metric_name $resources_metric_name --arg group "$group" '.data.result[] |
    {
      "name": $metric_name,
      "group": $group,
      "set": (.value[1] | tonumber),
      "labels": {
        "kind": "Ingress"
      }
    }
    ' <<< "$prom_result")"
  fi

  prom_result=$(prometheus_query 'query=count(kube_controller_replicas{controller_type!="ReplicaSet"}) by (controller_type)')
  if [[ ! -z "$prom_result" ]]; then
    metrics="$metrics\n$(jq --arg metric_name $resources_metric_name --arg group "$group" '.data.result[] |
    {
      "name": $metric_name,
      "group": $group,
      "set": (.value[1] | tonumber),
      "labels": {
        "kind": .metric.controller_type
      }
    }
    ' <<< "$prom_result")"
  fi

  echo -e "$metrics" >> $METRICS_PATH
}

function rps_metrics() {
  rps_metric_name="flant_pricing_ingress_nginx_controllers_rps"
  group="group_helm_rps_metrics"
  jq -c --arg group "$group" '.group = $group' <<< '{"action":"expire"}' >> $METRICS_PATH

  prom_result=$(prometheus_query 'query=sum(rate(ingress_nginx_overall_requests_total[20m])) or vector(0)')
  if [[ ! -z "$prom_result" ]]; then
    jq --arg metric_name $rps_metric_name --arg group "$group" '.data.result[] |
      {
        "name": $metric_name,
        "group": $group,
        "set": (.value[1] | tonumber),
        "labels": {}
      }
      ' <<< "$prom_result" >> $METRICS_PATH
  fi
}

function __main__() {
  terraform_state_metrics
  helm_releases_metrics
  resources_metrics
  rps_metrics
}

hook::run "$@"
