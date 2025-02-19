#!/usr/bin/env python3

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

from typing import Tuple, Optional, Any
import base64
import yaml
from dotmap import DotMap
from deckhouse import hook

PROVIDERS_WITH_REGISTRY_DATA_DEVICE_SUPPORT = {
    "aws",
    "gcp",
    "yandex",
    "azure",
    "openstack",
    "huaweicloud",
    # "vsphere",
    # "vcd",
    # "zvirt",
    # "dynamix",
}
REGISTRY_MODES_WITHOUT_REGISTRY_DATA_DEVICE_SUPPORT = {"Direct"}

HOOK_CONFIG = """
configVersion: v1
kubernetes:
  - name: cluster_cfg
    apiVersion: v1
    kind: Secret
    group: main
    executeHookOnEvent: []
    executeHookOnSynchronization: false
    keepFullObjectsInMemory: false
    nameSelector:
      matchNames:
      - d8-cluster-configuration
    namespace:
      nameSelector:
        matchNames: ["kube-system"]
    jqFilter: '.data."cluster-configuration.yaml" // ""'
kubernetesValidating:
- name: d8-provider-cluster-configuration-secret.deckhouse.io
  group: main
  labelSelector:
    matchLabels:
      name: d8-provider-cluster-configuration
  namespace:
    labelSelector
      matchLabels:
        kubernetes.io/metadata.name: kube-system
  rules:
  - apiGroups:   [""]
    apiVersions: ["v1"]
    operations:  ["*"]
    resources:   ["secrets"]
    scope:       "Namespaced"
"""


def main(ctx: hook.Context):
    try:
        # DotMap is a dict with dot notation
        binding_context = DotMap(ctx.binding_context)
        validate(binding_context, ctx.output.validations)
    except Exception as e:
        ctx.output.validations.error(str(e))


def validate(ctx: DotMap, output: hook.ValidationsCollector):
    operation = ctx.review.request.operation
    if operation in ["CREATE", "UPDATE"]:
        return validate_creation_or_update(ctx, output)
    elif operation == "DELETE":
        return validate_delete(ctx, output)
    else:
        raise Exception(f"Unknown operation {operation}")


def validate_delete(ctx: DotMap, output: hook.ValidationsCollector):
    output.allow()
    return


def validate_creation_or_update(ctx: DotMap, output: hook.ValidationsCollector):
    cluster_cfg, err = get_cluster_cfg(ctx)
    if err is not None:
        output.deny(err)
        return
    provider_secondary_devices_cfg, err = get_provider_secondary_devices_cfg(ctx)
    if err is not None:
        output.deny(err)
        return
    err = validate_registry_data_device(
        provider_secondary_devices_cfg=provider_secondary_devices_cfg,
        cluster_cfg=cluster_cfg,
    )
    if err is not None:
        output.deny(err)
        return
    output.allow()


def validate_registry_data_device(
    provider_secondary_devices_cfg: Optional[dict], cluster_cfg: Optional[dict]
) -> Optional[str]:
    # Ensure both configuration dicts are available
    if provider_secondary_devices_cfg is not None and cluster_cfg is None:
        return "Cannot get provider cluster configuration"
    if provider_secondary_devices_cfg is None or cluster_cfg is None:
        return None

    # Check field presence in provider config
    registry_data_device_enable_field, is_exist_registry_data_device_enable_field = (
        get_nested_value(
            data=provider_secondary_devices_cfg, keys=["RegistryDataDeviceEnable"]
        )
    )

    # Skip check if the registry data device is not enabled
    if not is_exist_registry_data_device_enable_field:
        return None
    registry_data_device_enable_field = bool(registry_data_device_enable_field)
    if not registry_data_device_enable_field:
        return None

    # Validate the cloud provider support
    cloud_provider_field, is_exist_cloud_provider_field = get_nested_value(
        data=cluster_cfg, keys=["cloud", "provider"]
    )
    if is_exist_cloud_provider_field:
        cloud_provider_field = str(cloud_provider_field)
        if (
            cloud_provider_field.lower()
            not in PROVIDERS_WITH_REGISTRY_DATA_DEVICE_SUPPORT
        ):
            return f'The registry data device is not supported with the cloud provider \
                "{cloud_provider_field}". Please select a registry mode that does \
                not require the registry data device. Available modes: \
                {", ".join(REGISTRY_MODES_WITHOUT_REGISTRY_DATA_DEVICE_SUPPORT)}'
    return None


def get_cluster_cfg(ctx: DotMap) -> Tuple[Optional[dict], Optional[str]]:
    # Fetch data from snapshots and decode it
    try:
        snapshots = ctx.snapshots["cluster_cfg"]
        if len(snapshots) == 0:
            return None, None
        cfg_data = snapshots[0].filterResult
        if len(cfg_data) == 0:
            return None, None
        cfg_yaml = base64.standard_b64decode(cfg_data)
        # Unmarshal to YAML
        cfg = yaml.safe_load(cfg_yaml)
        return cfg, None
    except Exception as e:
        return None, f"Cannot process cluster configuration: {str(e)}"


def get_provider_secondary_devices_cfg(
    ctx: DotMap,
) -> Tuple[Optional[dict], Optional[str]]:
    try:
        cfg_data = ctx.review.request.object.data.get(
            "cloud-provider-secondary-devices-configuration.yaml", None
        )
        if cfg_data is None:
            return None, None
        cfg_yaml = base64.standard_b64decode(cfg_data)
        # Unmarshal to YAML
        cfg = yaml.safe_load(cfg_yaml)
        return cfg, None
    except Exception as e:
        return (
            None,
            f"Cannot process cloud provider secondary devices configuration: {str(e)}",
        )


def get_nested_value(data: dict, keys: list) -> Tuple[Any, bool]:
    current = data
    for key in keys:
        if isinstance(current, dict) and key in current:
            current = current[key]
        else:
            return None, False
    return current, True


if __name__ == "__main__":
    hook.run(main, config=HOOK_CONFIG)
