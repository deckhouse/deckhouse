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

# This step is used to set the status of the currently configured registry. The status must be set before executing the tasks:
# - 001_configure_kubernetes_api_proxy.sh.tpl (simultaneously used as a proxy for the registry)
# - 001_configure_containerd_registry.sh.tpl (registry authentication configuration)
# and after:
# - 001_waiting_approval_annotations.sh.tpl (receiving approval from bashible to execute the other tasks)

function create_annotation(){
    local annotation="$1=$2"
    local node="$D8_NODE_HOSTNAME"
    bb-kubectl --request-timeout=60s --kubeconfig=/etc/kubernetes/kubelet.conf annotate node $node --overwrite $annotation
}

if [ "$FIRST_BASHIBLE_RUN" != "yes" ]; then
    create_annotation "registry.deckhouse.io/version" {{ .registry.version | quote }}
fi
