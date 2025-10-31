#!/usr/bin/python3

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

import os
from deckhouse import hook
from dotmap import DotMap
from feature_gates_generated import get_feature_gate_info

config = """
configVersion: v1
kubernetesValidating:
- name: cpm-moduleconfig-feature-gates.deckhouse.io
  group: main
  matchConditions:
  - name: "only-control-plane-manager-module"
    expression: 'request.name == "control-plane-manager"'
  rules:
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["*"]
    operations:  ["CREATE", "UPDATE"]
    resources:   ["moduleconfigs"]
    scope:       "Cluster"
"""


def main(ctx: hook.Context):
    try:
        binding_context = DotMap(ctx.binding_context)
        warnings = validate(binding_context)
        ctx.output.validations.allow(*warnings)
    except Exception as e:
        ctx.output.validations.error(str(e))


def validate(ctx: DotMap) -> list[str]:
    req = ctx.review.request
    
    k8s_version_file = "/tmp/kubectl_version"
    if not os.path.exists(k8s_version_file) or os.path.getsize(k8s_version_file) == 0:
        return []
    
    with open(k8s_version_file, 'r') as f:
        k8s_version = f.read().strip()
    
    version_parts = k8s_version.split('.')
    if len(version_parts) < 2:
        return []
    normalized_version = f"{version_parts[0]}.{version_parts[1]}"
    
    enabled_feature_gates = req.object.spec.settings.get('enabledFeatureGates', [])
    if not enabled_feature_gates or not isinstance(enabled_feature_gates, list):
        return []
    
    warnings = []
    
    components = ['apiserver', 'kubelet', 'kubeControllerManager', 'kubeScheduler']
    
    for feature_gate in enabled_feature_gates:
        if not feature_gate:
            continue
        
        found_in_any_component = False
        is_forbidden = False
        is_deprecated = False

        # deprecated and forbidden are global for the version, so we check them once
        try:
            info_check = get_feature_gate_info(normalized_version, components[0], feature_gate)
            is_forbidden = info_check.is_forbidden
            is_deprecated = info_check.is_deprecated
        except Exception:
            pass
        
        for component_name in components:
            try:
                info = get_feature_gate_info(normalized_version, component_name, feature_gate)
                if info.exists:
                    found_in_any_component = True
                    break
            except Exception:
                continue
        
        if found_in_any_component:
            if is_forbidden:
                warning_msg = f"'{feature_gate}' is forbidden for Kubernetes version {normalized_version} and will not be applied"
                warnings.append(warning_msg)
            elif is_deprecated:
                warning_msg = f"'{feature_gate}' is deprecated for Kubernetes version {normalized_version} and will not be applied"
                warnings.append(warning_msg)
        else:
            warning_msg = f"'{feature_gate}' is unknown or enabled by default FeatureGate for Kubernetes version {normalized_version} and will not be applied"
            warnings.append(warning_msg)
    
    return warnings


if __name__ == "__main__":
    hook.run(main, config=config)

