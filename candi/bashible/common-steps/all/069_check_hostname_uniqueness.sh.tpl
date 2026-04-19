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
{{- if eq .runType "Normal" }}
  {{- if or (eq .nodeGroup.nodeType "Static") (eq .nodeGroup.nodeType "CloudStatic") }}
if [ "$FIRST_BASHIBLE_RUN" == "no" ]; then
  exit 0
fi

if [ ! -f /var/lib/bashible/bootstrap-token ]; then
  exit 0
fi
token="$(</var/lib/bashible/bootstrap-token)"

counter=0
limit=3
while true; do
  if [[ $counter -eq $limit ]]; then
    bb-log-error "ERROR: Retry limit reached"
    exit 1
  fi
  for server in {{ .normal.apiserverEndpoints | join " " }}; do
    url="https://$server/api/v1/nodes/$(bb-d8-node-name)"
    if out="$(d8-curl --connect-timeout 10 -sS -f -x "" -X GET "$url" --header "Authorization: Bearer $token" --cacert "$BOOTSTRAP_DIR/ca.crt" 2>&1)"; then
      # got node info from API, node exists, should fail
      bb-log-error "ERROR: A node with the hostname $(bb-d8-node-name) already exists in the cluster\nPlease change the hostname, it should be unique in the cluster.\nThen clean up the server by running the script /var/lib/bashible/cleanup_static_node.sh and try again."
      exit 1
    fi
    if grep -q "The requested URL returned error: 404" <<< "$out"; then
      # cannot got node info from API, but got valid 404 response from API, node doesn't exists, finish successfully
      exit 0
    fi
    bb-log-error "ERROR: The request the $url returned an error: $out"
  done
  sleep 10
  counter=$((counter + 1))
done
  {{- end }}
{{- end }}
