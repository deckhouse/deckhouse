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
import logging
import yaml
from typing import Optional, List
from deckhouse import hook
from dotmap import DotMap

from feature_gates_generated import exists_in_component, is_forbidden, is_deprecated

CLUSTER_CONFIG_SNAPSHOT_NAME = "d8-cluster-configuration"
# Sentinels meaning "let Deckhouse pick the version". The two documents do not accept the same
# word: ModuleConfig takes Default only, while ClusterConfiguration keeps Automatic, which predates
# Default there and cannot be removed without breaking existing documents.
AUTOMATIC_VERSION = "Automatic"
DEFAULT_VERSION = "Default"


def is_module_config_track_default(version) -> bool:
    return version == DEFAULT_VERSION


def is_cluster_configuration_pinned(version) -> bool:
    """A concrete minor pin in ClusterConfiguration.

    Default is excluded here too: the schema does not accept it there, and a value that is
    obviously not a version must never be handed onward as one.
    """
    return bool(version) and version not in (AUTOMATIC_VERSION, DEFAULT_VERSION)

config = f"""
configVersion: v1
kubernetesValidating:
- name: cpm-moduleconfig-feature-gates.deckhouse.io
  group: main
  includeSnapshotsFrom: ["{CLUSTER_CONFIG_SNAPSHOT_NAME}"]
  matchConditions:
  - name: "only-control-plane-manager-module"
    expression: 'request.name == "control-plane-manager"'
  rules:
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["*"]
    operations:  ["CREATE", "UPDATE"]
    resources:   ["moduleconfigs"]
    scope:       "Cluster"

kubernetes:
- name: {CLUSTER_CONFIG_SNAPSHOT_NAME}
  apiVersion: v1
  kind: Secret
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
"""


def main(ctx: hook.Context):
    try:
        binding_context = DotMap(ctx.binding_context)
        warnings = validate(binding_context)
        ctx.output.validations.allow(*warnings)
    except Exception as e:
        ctx.output.validations.error(str(e))


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


# TODO(kubernetesVersion-deprecation): T+1 remove — drop CC fallback helper/branch once kubernetesVersion
# leaves ClusterConfiguration. After T+1 only MC → Default.
#
# NOTE(kubernetesVersion-deprecation): keep — do NOT drop the d8-cluster-configuration Secret snapshot;
# it still carries deckhouseDefaultKubernetesVersion for resolving "Automatic".
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


def get_k8s_version(ctx: DotMap) -> Optional[str]:
    # Mirrors global-hooks/discovery/target_kubernetes_version.go resolveTargetKubernetesVersion:
    # a present ModuleConfig setting decides on its own, Default included (it then means the
    # Deckhouse default, and ClusterConfiguration is not consulted at all).
    settings = ctx.review.request.object.get('spec', {}).get('settings', {})
    mc_version = settings.get('kubernetesVersion')
    if mc_version and not is_module_config_track_default(mc_version):
        return mc_version

    snapshot = ctx.snapshots.get(CLUSTER_CONFIG_SNAPSHOT_NAME, [])
    if not snapshot or len(snapshot) == 0:
        return None

    secret = snapshot[0]
    if not secret or not hasattr(secret, 'object'):
        return None

    data = secret.object.data
    if not data:
        return None

    if is_module_config_track_default(mc_version):
        version = get_deckhouse_default_version_from_secret(data)
        return version

    k8s_version = get_k8s_version_from_cluster_config(data)
    if is_cluster_configuration_pinned(k8s_version):
        return k8s_version

    version = get_deckhouse_default_version_from_secret(data)
    return version


def validate(ctx: DotMap) -> List[str]:
    req = ctx.review.request
    
    k8s_version = get_k8s_version(ctx)
    if not k8s_version:
        return []
    
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
        if is_forbidden(normalized_version, feature_gate):
            warning_msg = f"'{feature_gate}' is forbidden for Kubernetes version {normalized_version} and will not be applied"
            warnings.append(warning_msg)
        elif is_deprecated(normalized_version, feature_gate):
            warning_msg = f"'{feature_gate}' is deprecated for Kubernetes version {normalized_version} and will not be applied"
            warnings.append(warning_msg)
        else:
            found_in_any_component = any(
                exists_in_component(normalized_version, component, feature_gate)
                for component in components
            )
            if not found_in_any_component:
                warning_msg = f"'{feature_gate}' is unknown or enabled by default FeatureGate for Kubernetes version {normalized_version} and will not be applied"
                warnings.append(warning_msg)
    
    return warnings


if __name__ == "__main__":
    hook.run(main, config=config)
