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

{{- if or (eq .nodeGroup.nodeType "Static") (eq .nodeGroup.nodeType "CloudStatic") }}
if [[ "$FIRST_BASHIBLE_RUN" != "yes" ]]; then
  exit 0
fi

if [ -f /var/lib/bashible/bootstrap-token ]; then
  token="$(</var/lib/bashible/bootstrap-token)"
  while true; do
    for server in {{ .normal.apiserverEndpoints | join " " }}; do
      url="https://$server/api/v1/nodes/$HOSTNAME"
      if curl -sS -f -x "" -X GET "$url" --header "Authorization: Bearer $token" --cacert "$BOOTSTRAP_DIR/ca.crt"
      then
        bb-log-error "ERROR: A node with the hostname $HOSTNAME already exists in the cluster\nPlease change the hostname, it should be unique in the cluster.\nThen clean up the server by running the script /var/lib/bashible/cleanup_static_node.sh and try again."
        exit 1
      else
        exit 0
      fi
    done
    sleep 10
  done
else
  bb-log-error "failed to get node $HOSTNAME: can't find bootstrap-token"
  exit 1
fi

{{- end }}

