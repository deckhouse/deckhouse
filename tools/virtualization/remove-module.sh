#!/usr/bin/env bash
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

red=$(tput setaf 1)
bold=$(tput bold)
sgr0=$(tput sgr0)

color_echo(){
  echo "$red$bold$@ $sgr0"
}

remove_webhooks() {
  color_echo "Remove kubevirt webhooks"
  kubectl delete validatingwebhookconfigurations -l app.kubernetes.io/component=kubevirt
  kubectl delete validatingwebhookconfigurations -l cdi.kubevirt.io=cdi-api

  kubectl delete mutatingwebhookconfigurations -l app.kubernetes.io/component=kubevirt
  kubectl delete mutatingwebhookconfigurations -l cdi.kubevirt.io=cdi-api
}

remove_cdi() {
  color_echo "Remove CDI resource"
  # delete CDI and wait for it to be deleted
  kubectl patch cdi cdi --type=json -p '[{ "op": "remove", "path": "/metadata/finalizers" }]' 2>/dev/null
  kubectl delete cdi cdi --force --grace-period 0 2>/dev/null
  kubectl wait --for=delete -n cdi cdi --timeout=300s
}

remove_kubevirt() {
  color_echo "Remove kubevirt resource"
  # delete kubevirt and wait for it to be deleted
  kubectl -n d8-virtualization patch kubevirt kubevirt --type=json -p '[{ "op": "remove", "path": "/metadata/finalizers" }]' 2>/dev/null
  kubectl -n d8-virtualization delete kubevirt kubevirt --force --grace-period=0 2>/dev/null
  kubectl wait --for=delete -n d8-virtualization kubevirt kubevirt --timeout=300s
}

disable_virtualization_module() {
  color_echo "Disable virtualization module"
  # disable virtualization module
  kubectl patch mc virtualization --type='merge' --patch '{"spec":{"enabled":false}}'
  kubectl wait mc virtualization --for="jsonpath={.status.state}=Disabled" --timeout=320s
}

remove_crds() {
  color_echo "Remove kubvirt and deckhouse virtualizatiun CRDs"
  # remove deckhouse virtualization CRDs
  kubectl get crd -o name | grep -E 'virtualmachine.+deckhouse.io' | xargs kubectl delete --force --grace-period=0 2>/dev/null
  # remove kubevirt CRDs
  kubectl get crd -o name | grep kubevirt | xargs kubectl delete --force --grace-period=0 2>/dev/null
}

remove_apiservices() {
  color_echo "Remove kubevirt apiservice"
  kubectl get apiservices -o name | grep -E "(kubevirt|cdi)" | xargs kubectl delete --force --grace-period=0 2>/dev/null
}

remove_rbac() {
  color_echo "Remove virtualization module rbac"
  kubectl get clusterrole -o name | grep -E "(kubevirt|cdi)" | xargs kubectl delete --force --grace-period=0 2>/dev/null
  kubectl get clusterrolebindings -o name | grep -E "(kubevirt|cdi)" | xargs kubectl delete --force --grace-period=0 2>/dev/null
}

main(){
  remove_webhooks
  remove_cdi
  remove_kubevirt

  disable_virtualization_module

  remove_apiservices
  remove_rbac
  remove_crds

  # Let's delete webhooks again just in case, because the controller might have time to put them back in place.
  remove_webhooks
}

main
