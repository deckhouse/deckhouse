#!/usr/bin/python3

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

import base64
import yaml
from typing import Optional, List
from deckhouse import hook
from dotmap import DotMap

from feature_gates_generated import is_deprecated, is_feature_gate_deprecated_up_to_version

CLUSTER_CONFIG_SNAPSHOT_NAME = "d8-cluster-configuration"
MODULE_CONFIG_SNAPSHOT_NAME = "module-config-control-plane-manager"

config = f"""
configVersion: v1
kubernetesValidating:
- name: cpm-k8s-version-feature-gates.deckhouse.io
  group: cpm-feature-gates-validation
  includeSnapshotsFrom: ["{CLUSTER_CONFIG_SNAPSHOT_NAME}", "{MODULE_CONFIG_SNAPSHOT_NAME}"]
  namespace:
    labelSelector:
      matchLabels:
        kubernetes.io/metadata.name: kube-system
  labelSelector:
    matchLabels:
      name: d8-cluster-configuration
  rules:
  - apiGroups:   [""]
    apiVersions: ["v1"]
    operations:  ["UPDATE"]
    resources:   ["secrets"]
    scope:       "Namespaced"

kubernetes:
- name: {CLUSTER_CONFIG_SNAPSHOT_NAME}
  apiVersion: v1
  kind: Secret
  group: cpm-version-validation
  namespace:
    nameSelector:
      matchNames:
      - kube-system
  nameSelector:
    matchNames:
    - d8-cluster-configuration
  executeHookOnEvent: []
  executeHookOnSynchronization: true
  keepFullObjectsInMemory: true

- name: {MODULE_CONFIG_SNAPSHOT_NAME}
  apiVersion: deckhouse.io/v1alpha1
  kind: ModuleConfig
  group: cpm-version-validation
  nameSelector:
    matchNames:
    - control-plane-manager
  executeHookOnEvent: []
  executeHookOnSynchronization: true
  keepFullObjectsInMemory: true
"""


def main(ctx: hook.Context):
    try:
        binding_context = DotMap(ctx.binding_context)
        error = validate(binding_context)
        if error:
            ctx.output.validations.deny(error)
        else:
            ctx.output.validations.allow()
    except Exception as e:
        ctx.output.validations.deny(str(e))


def get_deckhouse_default_version_from_secret(secret_data) -> Optional[str]:
    encoded_version = secret_data.get('deckhouseDefaultKubernetesVersion')
    if not encoded_version:
        return None
    
    try:
        decoded_version = base64.b64decode(encoded_version).decode('utf-8').strip()
        if decoded_version:
            return decoded_version
    except Exception as e:
        logging.error(f"Failed to decode deckhouse default Kubernetes version from base64: {e}")
    
    return None


def get_k8s_version_from_cluster_config(secret_data) -> Optional[str]:
    encoded_config = secret_data.get('cluster-configuration.yaml')
    if not encoded_config:
        return None
    
    try:
        decoded_config = base64.b64decode(encoded_config).decode('utf-8')
        config_dict = yaml.safe_load(decoded_config)
        if config_dict and isinstance(config_dict, dict):
            kubernetes_version = config_dict.get('kubernetesVersion')
            if kubernetes_version and isinstance(kubernetes_version, str):
                return kubernetes_version
        except Exception as e:
            logging.error(f"Failed to decode Kubernetes version from cluster configuration: {e}")
    
    return None


def get_enabled_feature_gates(ctx: DotMap) -> List[str]:
    snapshot = ctx.snapshots.get(MODULE_CONFIG_SNAPSHOT_NAME, [])
    if not snapshot or len(snapshot) == 0:
        return []
    
    module_config = snapshot[0]
    if not module_config or not hasattr(module_config, 'object'):
        return []
    
    spec = module_config.object.get('spec', {})
    settings = spec.get('settings', {})
    enabled_feature_gates = settings.get('enabledFeatureGates', [])
    
    if not enabled_feature_gates or not isinstance(enabled_feature_gates, list):
        return []
    
    return [fg for fg in enabled_feature_gates if fg]


def normalize_version(version: str) -> str:
    version_parts = version.split('.')
    if len(version_parts) < 2:
        return version
    return f"{version_parts[0]}.{version_parts[1]}"

def validate(ctx: DotMap) -> Optional[str]:
    req = ctx.review.request
    
    old_secret = req.get('oldObject')
    new_secret = req.get('object')
    
    if not old_secret:
        return None
    
    if not new_secret:
        return None
    
    old_data = old_secret.get('data')
    new_data = new_secret.get('data')
    
    if not old_data or not new_data:
        return None
    
    old_config_version = get_k8s_version_from_cluster_config(old_data)
    new_config_version = get_k8s_version_from_cluster_config(new_data)
    
    if old_config_version == new_config_version:
        return None
    
    if not new_config_version:
        return None
    
    target_version = new_config_version
    
    if target_version == "Automatic":
        default_version = get_deckhouse_default_version_from_secret(new_data)
        if not default_version:
            return None
        target_version = default_version
    
    normalized_version = normalize_version(target_version)
    
    enabled_feature_gates = get_enabled_feature_gates(ctx)
    if not enabled_feature_gates:
        return None
    
    deprecated_feature_gates = []
    
    for feature_gate in enabled_feature_gates:
        if not feature_gate:
            continue
        
        try:
            if is_feature_gate_deprecated_up_to_version(feature_gate, normalized_version):
                deprecated_feature_gates.append(feature_gate)
        except Exception:
            continue
    
    if deprecated_feature_gates:
        feature_gates_str = ', '.join(f"'{fg}'" for fg in deprecated_feature_gates)
        return (
            f"Cannot change Kubernetes version to {target_version}:\n"
            f"The following feature gates are deprecated in this version or earlier: {feature_gates_str}\n"
            f"You can remove them from the enabledFeatureGates in the control-plane-manager ModuleConfig."
        )
    
    return None


if __name__ == "__main__":
    hook.run(main, config=config)

