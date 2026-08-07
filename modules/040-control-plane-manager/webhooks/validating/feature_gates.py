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
import re
import yaml
from typing import Optional, List
from deckhouse import hook
from dotmap import DotMap

from feature_gates_generated import exists_in_component, is_forbidden, is_deprecated

CLUSTER_CONFIG_SNAPSHOT_NAME = "d8-cluster-configuration"
CLUSTER_KUBERNETES_SNAPSHOT_NAME = "d8-cluster-kubernetes"
# A declared version this webhook is willing to hand onward as a version.
VERSION_RE = re.compile(r'^v?\d+\.\d+(\.\d+)?$')
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


def usable_declared_version(version, source: str) -> Optional[str]:
    """Drop a declared kubernetesVersion that is neither a sentinel nor a version, and say so.

    Mirrors usableDeclaredVersion in global-hooks/discovery/target_kubernetes_version.go. Both
    enums make such a value unwritable, so this is defence in depth for objects that predate the
    current schema — most concretely a ModuleConfig carrying the Automatic alias, which was legal
    there before the alias was dropped from that enum.

    Returning None makes the caller fall through to the next source, which is what already happens
    when the field is absent. Without the log the fall-through is indistinguishable from an unset
    field, and the version silently stops being checked at all.
    """
    if not version:
        return None
    if VERSION_RE.match(version):
        return version

    logging.warning(
        "ignoring the declared kubernetesVersion %r from %s: not a version and not a sentinel this document accepts",
        version, source,
    )
    return None

config = f"""
configVersion: v1
kubernetesValidating:
- name: cpm-moduleconfig-feature-gates.deckhouse.io
  group: main
  includeSnapshotsFrom: ["{CLUSTER_CONFIG_SNAPSHOT_NAME}", "{CLUSTER_KUBERNETES_SNAPSHOT_NAME}"]
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

# status.automaticVersion of this ConfigMap is what "Default" resolves to for the running Deckhouse
# build. It replaces deckhouseDefaultKubernetesVersion in the Secret above, which was only ever
# raised and therefore kept answering with a default that no longer exists after a Deckhouse
# downgrade. The Secret key stays as a fallback for the window before update-observer first writes
# this object. Kept in step with the same snapshot in k8s_version_feature_gates.py.
- name: {CLUSTER_KUBERNETES_SNAPSHOT_NAME}
  apiVersion: v1
  kind: ConfigMap
  namespace:
    nameSelector:
      matchNames:
      - kube-system
  nameSelector:
    matchNames:
    - d8-cluster-kubernetes
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


def get_deckhouse_default_version_from_configmap(ctx: DotMap) -> Optional[str]:
    """Read status.automaticVersion from kube-system/d8-cluster-kubernetes.

    update-observer owns that ConfigMap and writes automaticVersion from the version_map entry
    marked default in the running build, so it always describes the Deckhouse that is actually
    installed. Missing or unparsable means "not published yet" and the caller falls back to the
    Secret key. Kept in step with the same helper in k8s_version_feature_gates.py.
    """
    snapshot = ctx.snapshots.get(CLUSTER_KUBERNETES_SNAPSHOT_NAME, [])
    if not snapshot or len(snapshot) == 0:
        return None

    config_map = snapshot[0]
    if not config_map or not hasattr(config_map, 'object'):
        return None

    raw_status = config_map.object.get('data', {}).get('status')
    if not raw_status:
        return None

    try:
        status = yaml.safe_load(raw_status)
    except Exception as e:
        logging.error(f"Failed to parse d8-cluster-kubernetes data.status: {e}")
        return None

    if not isinstance(status, dict):
        return None

    automatic_version = status.get('automaticVersion')
    if isinstance(automatic_version, str) and automatic_version.strip():
        return automatic_version.strip()

    return None


def get_cluster_configuration_secret_data(ctx: DotMap):
    """Return data of the d8-cluster-configuration Secret, or None when it is absent or empty."""
    snapshot = ctx.snapshots.get(CLUSTER_CONFIG_SNAPSHOT_NAME, [])
    if not snapshot or len(snapshot) == 0:
        return None

    secret = snapshot[0]
    if not secret or not hasattr(secret, 'object'):
        return None

    return secret.object.data or None


def get_k8s_version(ctx: DotMap) -> Optional[str]:
    # Mirrors global-hooks/discovery/target_kubernetes_version.go resolveTargetKubernetesVersion:
    # a present ModuleConfig setting decides on its own, Default/Automatic included (it then means the
    # Deckhouse default, and ClusterConfiguration is not consulted at all).
    settings = ctx.review.request.object.get('spec', {}).get('settings', {})
    mc_version = settings.get('kubernetesVersion')
    if mc_version and not is_module_config_track_default(mc_version):
        mc_pin = usable_declared_version(mc_version, "ModuleConfig control-plane-manager")
        if mc_pin:
            # TODO(E2E-KV): temporary stand Info logs — remove before final PR (`rg E2E-KV`).
            logging.info("E2E-KV python-feature-gates source=mc-pin version=%s", mc_pin)
            return mc_pin

    secret_data = get_cluster_configuration_secret_data(ctx)

    # The Deckhouse default now comes from status.automaticVersion of the cluster ConfigMap, with
    # the Secret key kept only until update-observer has written that object at least once.
    # TODO(kubernetesVersion-deprecation): T+1 remove — drop the Secret fallback.
    def deckhouse_default() -> Optional[str]:
        version = get_deckhouse_default_version_from_configmap(ctx)
        if version:
            return version
        if secret_data:
            return get_deckhouse_default_version_from_secret(secret_data)
        return None

    if is_module_config_track_default(mc_version):
        version = deckhouse_default()
        # TODO(E2E-KV): temporary stand Info logs — remove before final PR (`rg E2E-KV`).
        logging.info("E2E-KV python-feature-gates source=mc-track-default version=%s mc=%s", version, mc_version)
        return version

    if secret_data:
        k8s_version = get_k8s_version_from_cluster_config(secret_data)
        if is_cluster_configuration_pinned(k8s_version):
            cc_pin = usable_declared_version(k8s_version, "ClusterConfiguration")
            if cc_pin:
                # TODO(E2E-KV): temporary stand Info logs — remove before final PR (`rg E2E-KV`).
                logging.info("E2E-KV python-feature-gates source=cc-pin version=%s", cc_pin)
                return cc_pin

    version = deckhouse_default()
    # TODO(E2E-KV): temporary stand Info logs — remove before final PR (`rg E2E-KV`).
    logging.info("E2E-KV python-feature-gates source=%s version=%s mc=%s",
                 "deckhouse-default" if version else "none", version, mc_version)
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
