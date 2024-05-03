# Copyright 2024 Flant JSC
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

# $1 - username $2 - request data
function bb-nodeuser-patch() {
  local username="$1"
  local data="$2"

  # Skip this step after multiple failures.
  # This step puts information "how to get bootstrap logs" into Instance resource.
  # It's not critical, and waiting for it indefinitely, breaking bootstrap, is not reasonable.
  local failure_count=0
  local failure_limit=3

  if type kubectl >/dev/null 2>&1 && test -f /etc/kubernetes/kubelet.conf ; then
    json_file=$( mktemp -t patch_json.XXXXX )
    echo "${data}" > $json_file

    until bb-kubectl --kubeconfig=/etc/kubernetes/kubelet.conf patch nodeusers.deckhouse.io "${username}" --type=json --patch-file="${json_file}" --subresource=status; do
      failure_count=$((failure_count + 1))
      if [[ $failure_count -eq $failure_limit ]]; then
        bb-log-error "ERROR: Failed to patch NodeUser with kubectl --kubeconfig=/etc/kubernetes/kubelet.conf"
        break
      fi
      bb-log-error "failed to NodeUser with kubectl --kubeconfig=/etc/kubernetes/kubelet.conf"
      sleep 10
    done
    rm $json_file
  elif [ -f /var/lib/bashible/bootstrap-token ]; then
    local patch_pending=true
    while [ "$patch_pending" = true ] ; do
      for server in {{ .normal.apiserverEndpoints | join " " }} ; do
        local server_addr=$(echo $server | cut -f1 -d":")
        until local tcp_endpoint="$(ip ro get ${server_addr} | grep -Po '(?<=src )([0-9\.]+)')"; do
          bb-log-info "The network is not ready for connecting to apiserver yet, waiting..."
          sleep 1
        done

        if curl -sS --fail -x "" \
          --max-time 10 \
          -XPATCH \
          -H "Authorization: Bearer $(</var/lib/bashible/bootstrap-token)" \
          -H "Accept: application/json" \
          -H "Content-Type: application/json-patch+json" \
          --cacert "$BOOTSTRAP_DIR/ca.crt" \
          --data "${data}" \
          "https://$server/apis/deckhouse.io/v1/nodeusers/${username}/status" ; then

          bb-log-info "Successfully patched NodeUser."
          patch_pending=false

          break
        else
          failure_count=$((failure_count + 1))

          if [[ $failure_count -eq $failure_limit ]]; then
            bb-log-error "Failed to patch NodeUser. Number of attempts exceeded. NodeUser patch will be skipped."
            patch_pending=false
            break
          fi

          bb-log-error "Failed to patch NodeUser. ${failure_count} of ${failure_limit} attempts..."
          sleep 10
          continue
        fi
      done
    done
  else
    bb-log-error "failed to patch NodeUser can't find kubelet.conf or bootstrap-token"
    exit 1
  fi
}
