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

# handler.python of the ValidationWebhook cpm-moduleconfig-feature-gates in
# templates/feature-gates-validation-webhooks.yaml. The operator template provides the imports and
# main; the ValidationWebhook puts feature_gates_generated.py (is_forbidden, is_deprecated,
# exists_in_component) and k8s_version_common.py (the kubernetesVersion helpers) before this file.

from typing import List, Optional

from dotmap import DotMap


def get_k8s_version(ctx: DotMap) -> Optional[str]:
    # Mirrors global-hooks/discovery/target_kubernetes_version.go resolveTargetKubernetesVersion:
    # a present ModuleConfig setting decides on its own, Default/Automatic included (it then means the
    # Deckhouse default, and ClusterConfiguration is not consulted at all).
    settings = ctx.review.request.object.get('spec', {}).get('settings', {})
    mc_version = settings.get('kubernetesVersion')
    if mc_version and not is_module_config_track_default(mc_version):
        mc_pin = usable_declared_version(mc_version, "ModuleConfig control-plane-manager")
        if mc_pin:
            return mc_pin

    secret_data = get_cluster_configuration_secret_data(ctx)

    # The Deckhouse default now comes from spec.automaticVersion of the cluster ConfigMap, with
    # the Secret key kept only until update-observer has written that object at least once.
    def deckhouse_default() -> Optional[str]:
        version = get_deckhouse_default_version_from_configmap(ctx)
        if version:
            return version
        if secret_data:
            return get_deckhouse_default_version_from_secret(secret_data)
        return None

    if is_module_config_track_default(mc_version):
        version = deckhouse_default()
        return version

    if secret_data:
        k8s_version = get_k8s_version_from_cluster_config(secret_data)
        if is_cluster_configuration_pinned(k8s_version):
            cc_pin = usable_declared_version(k8s_version, "ClusterConfiguration")
            if cc_pin:
                return cc_pin

    return deckhouse_default()


def validate(ctx: DotMap) -> tuple[List[str], bool]:
    # Allowed always; each warning stays a separate warning of the admission response.
    return collect_warnings(ctx), True


def collect_warnings(ctx: DotMap) -> List[str]:
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

