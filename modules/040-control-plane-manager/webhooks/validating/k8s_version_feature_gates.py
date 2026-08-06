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

from feature_gates_generated import is_deprecated, is_feature_gate_deprecated_up_to_version

CLUSTER_CONFIG_SNAPSHOT_NAME = "d8-cluster-configuration"
MODULE_CONFIG_SNAPSHOT_NAME = "module-config-control-plane-manager"
CLUSTER_KUBERNETES_SNAPSHOT_NAME = "d8-cluster-kubernetes"
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
- name: cpm-k8s-version-feature-gates.deckhouse.io
  group: cpm-feature-gates-validation
  includeSnapshotsFrom: ["{CLUSTER_CONFIG_SNAPSHOT_NAME}", "{MODULE_CONFIG_SNAPSHOT_NAME}", "{CLUSTER_KUBERNETES_SNAPSHOT_NAME}"]
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
# kubernetesVersion can also be set directly on the ModuleConfig, without ever touching the
# d8-cluster-configuration secret above — this binding covers that path.
- name: cpm-k8s-version-feature-gates-mc.deckhouse.io
  group: cpm-feature-gates-validation
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

# status.automaticVersion of this ConfigMap is what "Default" resolves to for the running Deckhouse
# build. It replaces deckhouseDefaultKubernetesVersion in the Secret above, which was only ever
# raised and therefore kept answering with a default that no longer exists after a Deckhouse
# downgrade. The Secret key stays as a fallback for the window before update-observer first writes
# this object.
- name: {CLUSTER_KUBERNETES_SNAPSHOT_NAME}
  apiVersion: v1
  kind: ConfigMap
  group: cpm-version-validation
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
        error = validate(binding_context)
        if error:
            ctx.output.validations.deny(error)
        else:
            ctx.output.validations.allow()
    except Exception as e:
        # Stays fail-closed, and deliberately not validations.error(): in deckhouse==0.4.11 (the
        # version this image pins) error() builds {"allowed": False, <message>: "..."} — it uses the
        # message as the dict *key*, so the response carries no "message" field at all. It rejects
        # just like deny() but leaves the operator with no explanation.
        #
        # This binding now also covers ModuleConfig, so a bug in this webhook blocks edits to
        # `mc control-plane-manager` — including the edit that would work around it. Keeping the
        # text explicit at least makes the cause obvious in kubectl output.
        ctx.output.validations.deny(f"internal error in the kubernetesVersion feature gates webhook: {e}")


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


def get_deckhouse_default_version_from_configmap(ctx: DotMap) -> Optional[str]:
    """Read status.automaticVersion from kube-system/d8-cluster-kubernetes.

    update-observer owns that ConfigMap and writes automaticVersion from the version_map entry
    marked default in the running build, so it always describes the Deckhouse that is actually
    installed. Missing or unparsable means "not published yet" and the caller falls back to the
    Secret key.
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


# TODO(kubernetesVersion-deprecation): T+1 remove — drop CC fallback helper/branch once kubernetesVersion
# leaves ClusterConfiguration. After T+1 only MC → Default.
#
# NOTE(kubernetesVersion-deprecation): keep — do NOT drop the d8-cluster-configuration Secret snapshot;
# it still carries deckhouseDefaultKubernetesVersion for resolving "Automatic". Only the secret-triggered
# validating rule becomes pointless.
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


def get_module_config_settings(ctx: DotMap) -> dict:
    snapshot = ctx.snapshots.get(MODULE_CONFIG_SNAPSHOT_NAME, [])
    if not snapshot or len(snapshot) == 0:
        return {}
    
    module_config = snapshot[0]
    if not module_config or not hasattr(module_config, 'object'):
        return {}
    
    spec = module_config.object.get('spec', {})
    settings = spec.get('settings', {})
    return settings if isinstance(settings, dict) else {}


def get_enabled_feature_gates(ctx: DotMap) -> List[str]:
    settings = get_module_config_settings(ctx)
    enabled_feature_gates = settings.get('enabledFeatureGates', [])
    
    if not enabled_feature_gates or not isinstance(enabled_feature_gates, list):
        return []
    
    return [fg for fg in enabled_feature_gates if fg]


def normalize_version(version: str) -> str:
    version_parts = version.split('.')
    if len(version_parts) < 2:
        return version
    return f"{version_parts[0]}.{version_parts[1]}"


def get_cluster_configuration_secret_data(ctx: DotMap):
    snapshot = ctx.snapshots.get(CLUSTER_CONFIG_SNAPSHOT_NAME, [])
    if not snapshot or len(snapshot) == 0:
        return None

    secret = snapshot[0]
    if not secret or not hasattr(secret, 'object'):
        return None

    return secret.object.data


def resolve_effective_version(
    mc_kubernetes_version: Optional[str],
    ctx: DotMap,
    secret_data=None,
) -> Optional[str]:
    # Mirrors global-hooks/discovery/cluster_configuration.go resolveTargetKubernetesVersion:
    # a present ModuleConfig setting decides on its own, Default/Automatic included (it then means the
    # Deckhouse default, and ClusterConfiguration is not consulted at all). Only an absent setting
    # falls back to ClusterConfiguration, where "Automatic" is not a pin either.
    if mc_kubernetes_version and not is_module_config_track_default(mc_kubernetes_version):
        # TODO(E2E-KV): temporary stand Info logs — remove before final PR (`rg E2E-KV`).
        logging.info("E2E-KV python-k8s-fg source=mc-pin version=%s", mc_kubernetes_version)
        return mc_kubernetes_version

    if secret_data is None:
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

    if is_module_config_track_default(mc_kubernetes_version):
        version = deckhouse_default()
        # TODO(E2E-KV): temporary stand Info logs — remove before final PR (`rg E2E-KV`).
        logging.info("E2E-KV python-k8s-fg source=mc-track-default version=%s mc=%s", version, mc_kubernetes_version)
        return version

    if secret_data:
        cc_version = get_k8s_version_from_cluster_config(secret_data)
        if is_cluster_configuration_pinned(cc_version):
            # TODO(E2E-KV): temporary stand Info logs — remove before final PR (`rg E2E-KV`).
            logging.info("E2E-KV python-k8s-fg source=cc-pin version=%s", cc_version)
            return cc_version

    version = deckhouse_default()
    # TODO(E2E-KV): temporary stand Info logs — remove before final PR (`rg E2E-KV`).
    logging.info("E2E-KV python-k8s-fg source=deckhouse-default version=%s", version)
    return version


def build_deprecated_feature_gates_error(target_version: str, enabled_feature_gates: List[str]) -> Optional[str]:
    normalized_version = normalize_version(target_version)

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


def validate_cluster_configuration_change(ctx: DotMap) -> Optional[str]:
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

    enabled_feature_gates = get_enabled_feature_gates(ctx)
    if not enabled_feature_gates:
        return None

    mc_version = get_module_config_settings(ctx).get('kubernetesVersion')

    # Whenever ModuleConfig carries a value at all, this edit cannot move the effective version:
    # resolve_effective_version returns the MC pin outright, and for Default/Automatic it reads the
    # deckhouseDefaultKubernetesVersion key — never ClusterConfiguration.kubernetesVersion. Denying
    # here blocked exactly the documented migration step (dropping the deprecated field from
    # ClusterConfiguration) that the D8UnsetKubernetesVersionInModuleConfig alert asks operators to
    # perform. The ModuleConfig branch guards against this class of error explicitly; this one did not.
    if mc_version:
        return None

    target_version = resolve_effective_version(mc_version, ctx, new_data)
    if not target_version:
        return None

    return build_deprecated_feature_gates_error(target_version, enabled_feature_gates)


def get_guarded_settings(obj) -> tuple:
    # The pair this webhook is about. Anything else in the ModuleConfig is none of its business.
    if not obj:
        return (None, None)

    settings = obj.get('spec', {}).get('settings', {})

    return (settings.get('kubernetesVersion'), settings.get('enabledFeatureGates'))


def validate_module_config_change(ctx: DotMap) -> Optional[str]:
    req = ctx.review.request

    new_object = req.get('object')
    if not new_object:
        return None

    # Mirror the ClusterConfiguration branch above and bail out when nothing relevant changed.
    # Without this the check re-runs on every edit of the ModuleConfig, so a config that already
    # carries a deprecated feature gate becomes uneditable as a whole: changing an unrelated
    # setting gets rejected with a message about a kubernetesVersion nobody touched. Worse, a
    # Deckhouse upgrade that adds entries to the deprecation table would put existing clusters in
    # that state without any user action. On CREATE there is no old object, so the check runs.
    old_object = req.get('oldObject')
    if old_object and get_guarded_settings(old_object) == get_guarded_settings(new_object):
        return None

    settings = new_object.get('spec', {}).get('settings', {})

    enabled_feature_gates = settings.get('enabledFeatureGates', [])
    if not enabled_feature_gates or not isinstance(enabled_feature_gates, list):
        return None
    enabled_feature_gates = [fg for fg in enabled_feature_gates if fg]
    if not enabled_feature_gates:
        return None

    target_version = resolve_effective_version(settings.get('kubernetesVersion'), ctx)
    if not target_version:
        return None

    return build_deprecated_feature_gates_error(target_version, enabled_feature_gates)


def validate(ctx: DotMap) -> Optional[str]:
    req = ctx.review.request
    # DotMap returns an empty DotMap for missing keys; calling .lower() on it raises
    # AttributeError, which main() turns into deny — with failurePolicy:Fail that would
    # lock ModuleConfig edits. Resolve kind defensively.
    kind = str(req.get('kind', {}).get('kind', '') or '').lower()

    if kind == "secret":
        return validate_cluster_configuration_change(ctx)

    if kind == "moduleconfig":
        return validate_module_config_change(ctx)

    return None


if __name__ == "__main__":
    hook.run(main, config=config)
