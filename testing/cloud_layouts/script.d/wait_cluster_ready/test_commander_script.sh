# Copyright 2025 Flant JSC
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
    pause_check=$( { kubectl get configmap pause-the-test; } 2>&1 ) || true

    if [[ $pause_check = *NotFound* ]]; then
      break
    elif [[ $pause_check = *pause-the-test* ]]; then
      echo 'Waiting until "kubectl delete cm pause-the-test" before destroying cluster'
    else
      >&2 echo "$pause_check"
      echo 'Unable to connect to Kubernetes API, waiting'
    fi

    sleep 30
  done
}

trap pause-the-test EXIT

if ! ingress_inlet=$(kubectl get ingressnginxcontrollers.deckhouse.io -o json | jq -re '.items[0] | .spec.inlet // empty'); then
  ingress="ok"
else
  ingress=""
fi

attempts=50
# With sleep timeout of 30s, we have 25 minutes period in total to catch the 100% availability from upmeter
for i in $(seq $attempts); do
  # Sleeping at the start for readability. First iterations do not succeed anyway.
  sleep 30
  if [[ -n "$ingress_inlet" ]]; then
    echo "Ingress inlet: ${ingress_inlet}"
    case "$ingress_inlet" in
      LoadBalancer)
        if ingress_service="$(kubectl -n d8-ingress-nginx get svc nginx-load-balancer -ojson 2>/dev/null)"; then
          echo "Ingress service: ${ingress_service}"
          if ingress_lb_addr="$(jq -re '.status.loadBalancer.ingress | if .[0].hostname then .[0].hostname else .[0].ip end' <<< "$ingress_service")"; then
            echo "Ingress LB address: ${ingress_lb_addr}"
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
        if master_ip="$(kubectl get node -o json | jq -r '[ .items[] | select(.metadata.labels."node-role.kubernetes.io/master"!=null) | .status.addresses[] ] | map(select(.type == "ExternalIP").address) + map(select(.type == "InternalIP").address) | first')"; then
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

  if kubectl -n d8-istio get po | grep istiod | grep -q Running; then
    istio="ok"
  else
    istio=""
  fi

  cat <<EOF
Istio operator check: $([ "$istio" == "ok" ] && echo "success" || echo "failed")
EOF

  if [[ "$ingress:$istio" == "ok:ok" ]]; then
    exit 0
  fi
done

>&2 echo 'Timeout waiting for checks to succeed'
exit 1
