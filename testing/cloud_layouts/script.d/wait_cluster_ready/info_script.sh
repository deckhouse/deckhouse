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

kubectl -n d8-system get deploy/deckhouse -o jsonpath='{.kind}/{.metadata.name}:{"\n"}Image: {.spec.template.spec.containers[0].image} {"\n"}Config: {.spec.template.spec.containers[0].env[?(@.name=="ADDON_OPERATOR_CONFIG_MAP")]}{"\n"}'
echo "Deployment/deckhouse"
kubectl -n d8-system get deploy/deckhouse -o wide
echo "Pod/deckhouse-*"
kubectl -n d8-system get po -o wide | grep ^deckhouse
echo "Enabled modules:"
kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module list -o yaml | grep -v enabledModules: | sort
echo "ConfigMap/generated"
kubectl -n d8-system get configmap/deckhouse-generated-config-do-not-edit -o yaml
echo "ModuleConfigs"
kubectl get moduleconfigs
echo "Errors:"
kubectl -n d8-system logs deploy/deckhouse | grep '"error"'
