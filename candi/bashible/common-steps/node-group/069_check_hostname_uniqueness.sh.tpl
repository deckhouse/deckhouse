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

{{- if and ( or (eq .nodeGroup.nodeType "Static") (eq .nodeGroup.nodeType "CloudStatic")) (eq .runType "Normal") }}
if [ -f /var/lib/bashible/bootstrap-token ]; then
  failure_count=0
  failure_limit=3
  curl_out=$( mktemp -t curl_out.XXXXX )
  retry=true
  token="$(</var/lib/bashible/bootstrap-token)"
  while [ "$retry" = true ]; do
    for server in {{ .normal.apiserverEndpoints | join " " }}; do
      url="https://$server/api/v1/nodes/$HOSTNAME"
      if d8-curl -sS -f -x "" -X GET "$url" --header "Authorization: Bearer $token" --cacert "$BOOTSTRAP_DIR/ca.crt" > $curl_out 2>&1
      then
        failure_count=$((failure_count + 1))

        if [[ $failure_count -eq $failure_limit ]]; then
          bb-log-error "ERROR: A node with the hostname $HOSTNAME already exists in the cluster\nPlease change the hostname, it should be unique in the cluster.\nThen clean up the server by running the script /var/lib/bashible/cleanup_static_node.sh and try again."
          retry=false
          exit 1
        fi

        bb-log-error "ERROR: A node with the hostname $HOSTNAME already exists in the cluster. ${failure_count} of ${failure_limit} attempts..."
      else
        if cat $curl_out | grep "The requested URL returned error: 404" > /dev/null; then
          exit 0
        else
          curl_error="$(<$curl_out)"
          failure_count=$((failure_count + 1))

          if [[ $failure_count -eq $failure_limit ]]; then
            bb-log-error "ERROR: The request to the $url returned an error: $curl_error"
            retry=false
            exit 1
          fi

          bb-log-error "ERROR: The request the $url returned an error: $curl_error ${failure_count} of ${failure_limit} attempts..."
        fi
      fi
    done
    sleep 10
  done
fi

{{- end }}
