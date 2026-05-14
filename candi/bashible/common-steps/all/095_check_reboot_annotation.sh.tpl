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

{{ if eq .runType "Normal" }}
REBOOT_ANNOTATION="$( bb-curl-kube "/api/v1/nodes/$D8_NODE_HOSTNAME" |jq -r '.metadata.annotations."update.node.deckhouse.io/reboot"' )"

if [[ $REBOOT_ANNOTATION != "null" ]]
  then
    attempts=30
    while true
      do
        if [[ attempts == 0 ]]
          then
            >&2 echo "out of attempts. exiting..."
            exit 1
        fi
        DRAINING_ANNOTATION="$( bb-curl-kube "/api/v1/nodes/$D8_NODE_HOSTNAME" |jq -r '.metadata.annotations."update.node.deckhouse.io/draining"' )"
        DRAINED_ANNOTATION="$( bb-curl-kube "/api/v1/nodes/$D8_NODE_HOSTNAME" |jq -r '.metadata.annotations."update.node.deckhouse.io/drained"' )"
        if [[ $DRAINED_ANNOTATION != "null" ]]
          then
            # node is drained, could be rebooted asap
            bb-flag-set reboot
            bb-curl-helper-patch-node-metadata "${D8_NODE_HOSTNAME}" "annotations" "update.node.deckhouse.io/reboot-"
            break
          else
            # node should be drained first
            if [[ $DRAINING_ANNOTATION == "null" ]]
              then
                # draining annotation didn't set, removing reboot annotation, drain node and set reboot flag after that
                bb-curl-helper-patch-node-metadata "${D8_NODE_HOSTNAME}" "annotations" "update.node.deckhouse.io/draining=bashible"
            fi
        fi
        sleep 20
        attempts=$(( attempts - 1 ))
    done
fi
{{- end }}
