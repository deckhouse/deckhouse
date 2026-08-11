#!/usr/bin/env python3
#
# Copyright 2026 Flant JSC
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

"""Shared kubernetesVersion resolution helpers for the control-plane-manager admission webhooks.

Both feature_gates.py and k8s_version_feature_gates.py have to answer the same question — what
Kubernetes version does this cluster declare — and they used to answer it with verbatim copies of
these helpers. Two copies of one resolution rule in one directory is the shape that drifts, and it
already had: one copy trimmed where the other did not.

Not a hook. This module carries no shell-operator config and is deliberately left non-executable,
so shell-operator does not pick it up as one — the same arrangement as feature_gates_generated.py,
which both webhooks already import from this directory.

Deliberately imported by name rather than with a star import, again following
feature_gates_generated.py: a reader of either webhook should be able to see where every symbol
comes from without opening this file.
"""

import base64
import logging
import re
from typing import Optional

import yaml
from dotmap import DotMap

# Snapshot names, shared because the two webhooks bind the same objects and their handlers read the
# snapshots by name.
CLUSTER_CONFIG_SNAPSHOT_NAME = "d8-cluster-configuration"
CLUSTER_KUBERNETES_SNAPSHOT_NAME = "d8-cluster-kubernetes"

# A declared version these webhooks are willing to hand onward as a version.
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

    Load-bearing in a way it is not in the Go hook: an unusable value used to travel all the way
    into is_feature_gate_deprecated_up_to_version, whose exception is swallowed per feature gate
    (see build_deprecated_feature_gates_error), so the whole deprecation check silently became a
    no-op and the webhook allowed everything. Returning None makes the caller fall through to the
    next source instead, which is what already happens when the field is absent.
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


def get_cluster_configuration_secret_data(ctx: DotMap):
    """Return data of the d8-cluster-configuration Secret, or None when it is absent or empty."""
    snapshot = ctx.snapshots.get(CLUSTER_CONFIG_SNAPSHOT_NAME, [])
    if not snapshot or len(snapshot) == 0:
        return None

    secret = snapshot[0]
    if not secret or not hasattr(secret, 'object'):
        return None

    return secret.object.data


def get_deckhouse_default_version_from_secret(secret_data) -> Optional[str]:
    """Read deckhouseDefaultKubernetesVersion from the d8-cluster-configuration Secret.

    TODO(kubernetesVersion-deprecation): T+1 remove — migration fallback for
    get_deckhouse_default_version_from_configmap below; nothing writes this key any more.
    """
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


# TODO(kubernetesVersion-deprecation): T+1 remove — drop the CC fallback helper once
# kubernetesVersion leaves ClusterConfiguration. After T+1 only MC → Default.
#
# NOTE(kubernetesVersion-deprecation): keep — do NOT drop the d8-cluster-configuration Secret
# snapshot; it still carries deckhouseDefaultKubernetesVersion for resolving the sentinel. Only the
# secret-triggered validating rule becomes pointless.
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
