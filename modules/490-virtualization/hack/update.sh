#!/bin/bash

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

if [ $# -ne 1 ]; then
  echo "Usage: hack/update.sh v1.54.0"
  exit 1
fi

manifest=$(mktemp)
trap "rm -f \"$manifest\"" EXIT

curl -LfsS "https://github.com/kubevirt/kubevirt/releases/download/$1/kubevirt-operator.yaml" -o "$manifest"
awk -v RS="\n---\n" '/\nkind: CustomResourceDefinition\n/ {print "---\n" $0}' "$manifest"  > crds/kubevirt.yaml

{
  awk -v RS='\n---\n' '/\nkind: ServiceAccount\n/ {print "---\n" $0}' "$manifest"
  printf "%s\n" "imagePullSecrets:" "- name: deckhouse-registry"
  awk -v RS='\n---\n' '/\nkind: Role\n/ {print "---\n" $0}' "$manifest" | \
    sed -z 's/\(\nmetadata:\n\([- ] [^\n]*\n\)\+  name:\) [^\n]*/\1 kubevirt-operator/'
  awk -v RS='\n---\n' '/\nkind: RoleBinding\n/ {print "---\n" $0}' "$manifest" | \
    sed -z 's/\(\nmetadata:\n\([- ] [^\n]*\n\)\+  name:\) [^\n]*/\1 kubevirt-operator/'
  awk -v RS='\n---\n' '/\nkind: ClusterRole\n.*\n  name: kubevirt-operator\n/ {print "---\n" $0}' "$manifest" | \
    sed -z 's/\(\nmetadata:\n\([- ] [^\n]*\n\)\+  name:\) [^\n]*/\1 d8:virtualization:kubevirt-operator/'
  awk -v RS='\n---\n' '/\nkind: ClusterRoleBinding\n/ {print "---\n" $0}' "$manifest" | \
    sed -z 's/\(\nmetadata:\n\([- ] [^\n]*\n\)\+  name:\) [^\n]*/\1 d8:virtualization:kubevirt-operator/' | \
    sed -z 's/\(\nroleRef:\n\([- ] [^\n]*\n\)\+  name:\) [^\n]*/\1 d8:virtualization:kubevirt-operator/'
} > templates/kubevirt-operator/rbac-for-us.yaml

sed -i 's/namespace: kubevirt/namespace: d8-virtualization/g' \
  templates/kubevirt-operator/rbac-for-us.yaml
sed -zi 's/  labels:\n\(    [^\n]*\n\)\+/  {{- include "helm_lib_module_labels" (list .) | nindent 2 }}\n/g' \
  templates/kubevirt-operator/rbac-for-us.yaml

builder_image=$(curl -LfsS "https://raw.githubusercontent.com/kubevirt/kubevirt/$1/hack/dockerized" | sed -n '/^KUBEVIRT_BUILDER_IMAGE=/ s/.*="\(.*\)"/\1/p')
sed -i images/artifact/werf.inc.yaml \
  -e '/{{- $version := ".*" }}/ s/".*"/"'"${1#*v}"'"/g' \
  -e '/{{- $builderImage := ".*" }}/ s|".*"|"'"${builder_image}"'"|g'
