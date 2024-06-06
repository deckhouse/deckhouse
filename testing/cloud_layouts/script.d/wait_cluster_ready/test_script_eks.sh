# Copyright 2023 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
export PATH="/opt/deckhouse/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export LANG=C
set -Eeuo pipefail

function pause-the-test() {
  while true; do
    if ! { kubectl get configmap pause-the-test -o json | jq -re '.metadata.name == "pause-the-test"' >/dev/null ; }; then
      break
    fi

    >&2 echo 'Waiting until "kubectl delete cm pause-the-test" before destroying cluster'

    sleep 30
  done
}

trap pause-the-test EXIT


sleep 3600


if ! ingress_inlet=$(kubectl get ingressnginxcontrollers.deckhouse.io -o json | jq -re '.items[0] | .spec.inlet // empty'); then
  ingress="ok"
else
  ingress=""
fi

availability=""
attempts=50
# With sleep timeout of 30s, we have 25 minutes period in total to catch the 100% availability from upmeter
for i in $(seq $attempts); do
  # Sleeping at the start for readability. First iterations do not succeed anyway.
  sleep 30

  if upmeter_addr=$(kubectl -n d8-upmeter get ep upmeter -o json | jq -re '.subsets[].addresses[0] | .ip') 2>/dev/null; then
    if upmeter_auth_token="$(kubectl -n d8-upmeter create token upmeter-agent)" 2>/dev/null; then

      # Getting availability data based on last 30 seconds of probe stats, note 'peek=1' query
      # param.
      #
      # Forcing curl error to "null" since empty input is not interpreted as null/false by JQ, and
      # -e flag does not work as expected. See
      # https://github.com/stedolan/jq/pull/1697#issuecomment-1242588319
      #
      if avail_json="$(kubectl -n d8-system exec -t deploy/deckhouse -- curl -k -s -S -m5 -H "Authorization: Bearer $upmeter_auth_token" "https://${upmeter_addr}:8443/public/api/status?peek=1" || echo null | jq -ce)" 2>/dev/null; then
        # Transforming the data to a flat array of the following structure  [{ "probe": "{group}/{probe}", "status": "ok/pending" }]
        avail_report="$(jq -re '
          [
            .rows[]
            | [
                .group as $group
                | .probes[]
                | {
                  probe: ($group + "/" + .probe),
                  status: (if .availability > 0.99   then "up"   else "pending"   end),
                  availability: .availability
                }
              ]
          ]
          | flatten
          ' <<<"$avail_json")"

        # Printing the table of probe statuses
        echo '*'
        echo '====================== AVAILABILITY, STATUS, PROBE ======================'
        # E.g.:  0.626  failure  monitoring-and-autoscaling/prometheus-metrics-adapter
        echo "$(jq -re '.[] | [((.availability*1000|round) / 1000), .status, .probe] | @tsv' <<<"$avail_report")" | column -t
        echo '========================================================================='

        # Overall availability status. We check that all probes are in place because at some point
        # in the start the list can be empty.
        availability="$(jq -r '
          if (
            (. | length > 0) and
            ([ .[] | select(.status != "up") ] | length == 0)
          )
          then "ok"
          else ""
          end '<<<"$avail_report")"

      else
        >&2 echo "Couldn't fetch availability data from upmeter (attempt #${i} of ${attempts})."
      fi
    else
      >&2 echo "Couldn't get upmeter-agent serviceaccount token (attempt #${i} of ${attempts})."
    fi
  else
    >&2 echo "Upmeter endpoint is not ready (attempt #${i} of ${attempts})."
  fi

    cat <<EOF
Availability check: $([ "$availability" == "ok" ] && echo "success" || echo "pending")
EOF

  if [[ -n "$ingress_inlet" ]]; then
    case "$ingress_inlet" in
      LoadBalancer)
        if ingress_service="$(kubectl -n d8-ingress-nginx get svc nginx-load-balancer -ojson 2>/dev/null)"; then
          if ingress_lb_addr="$(jq -re '.status.loadBalancer.ingress | if .[0].hostname then .[0].hostname else .[0].ip end' <<< "$ingress_service")"; then
            if ingress_lb_code="$(d8-curl -o /dev/null -s -w "%{http_code}" "$ingress_lb_addr")"; then
              if [[ "$ingress_lb_code" == "404" ]]; then
                ingress="ok"
              else
                >&2 echo "Got code $ingress_lb_code from LB $ingress_lb_addr, waiting for 404 (attempt #${i} of ${attempts})."
              fi
            else
              >&2 echo "Failed curl request to the LB address: $ingress_lb_addr (attempt #${i} of ${attempts})."
            fi
          else
            >&2 echo "Can't get svc/nginx-load-balancer LB address (attempt #${i} of ${attempts})."
          fi
        else
          >&2 echo "Can't get svc/nginx-load-balancer (attempt #${i} of ${attempts})."
        fi
        ;;
      HostPort|HostWithFailover)
        if master_ip="$(kubectl get node -o json | jq -r '[ .items[] | select(.metadata.labels."node-role.kubernetes.io/master"!=null) | .status.addresses[] | select(.type=="ExternalIP") | .address ] | .[0]')"; then
          if ingress_hp_code="$(d8-curl -o /dev/null -s -w "%{http_code}" "$master_ip")"; then
            if [[ "$ingress_hp_code" == "404" ]]; then
              ingress="ok"
            else
              >&2 echo "Got code $ingress_hp_code from $master_ip, waiting for 404 (attempt #${i} of ${attempts})."
            fi
          else
            >&2 echo "Failed curl request to the master ip address: $master_ip (attempt #${i} of ${attempts})."
          fi
        else
          >&2 echo "Can't get master ip address (attempt #${i} of ${attempts})."
        fi
        ;;
      *)
        >&2 echo "Ingress controller with inlet $ingress_inlet found in the cluster. But I have no instructions how to test it."
        exit 1
        ;;
      esac

    cat <<EOF
Ingress $ingress_inlet check: $([ "$ingress" == "ok" ] && echo "success" || echo "failure")
EOF
  fi

  if [[ "$availability:$ingress" == "ok:ok" ]]; then
    exit 0
  fi
done

>&2 echo 'Timeout waiting for checks to succeed'
exit 1
