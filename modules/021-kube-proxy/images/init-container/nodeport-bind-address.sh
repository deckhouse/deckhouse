#!/bin/bash

# Copyright 2021 Flant JSC
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

set -Eeuo pipefail

node_object="$(curl -sS -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" -k https://127.0.0.1:6445/api/v1/nodes/$(hostname -s))"
nodeport_bind_internal_ip="$(jq -re '.metadata.annotations."node.deckhouse.io/nodeport-bind-internal-ip" // true' <<< "$node_object")"

internalip="$(jq -re '[.status.addresses[] | select(.type == "InternalIP").address] | (first | "\(.)/32") // ""' <<< "$node_object")"

if [ -z "$internalip" ]; then
  >&2 echo "ERROR: Node $(hostname) doesn't have InternalIP in .status.addresses"
  exit 1
fi

if [ "$nodeport_bind_internal_ip" == "false" ]; then
  internalip="0.0.0.0/0"
fi

sed "s#__node_address__#$internalip#" /var/lib/kube-proxy-cm/config.conf > /var/lib/kube-proxy/config.conf
