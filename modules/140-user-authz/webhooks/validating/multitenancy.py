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


# This hook checks the MultiTenancy flag for the user-authz module.
#
# - If the flag is enabled — we just exit.
#
# - If the flag is disabled — we check the ClusterAuthorizationRule (CAR) resource being created or updated
#   for the presence of the following fields:
#   - allowAccessToSystemNamespaces
#   - limitNamespaces
#   - namespaceSelector
#
#   If any of those fields are present — creation is denied due to MultiTenancy restrictions.
#
# - Additionally, if the user attempts to disable the `enableMultiTenancy` flag in the user-authz ModuleConfig,
#   the hook validates all existing ClusterAuthorizationRule resources for the presence of the same fields.
#   If any CAR uses those fields — disabling MultiTenancy is denied.


from deckhouse import hook
from dotmap import DotMap

SEPARATOR = "; "
MODULE_CONFIG_SNAPSHOT_NAME = "d8-user-authz-moduleconfig" 
CLUSTER_AUTH_RULES_SNAPSHOT_NAME = "d8-user-authz-cars"
CONFIG = f"""
configVersion: v1
kubernetesValidating:
- name: d8-user-authz-car-multitenancy-related-options.deckhouse.io
  includeSnapshotsFrom: ["{MODULE_CONFIG_SNAPSHOT_NAME}"]
  rules:
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["*"]
    operations:  ["CREATE", "UPDATE"]
    resources:   ["clusterauthorizationrules"]
    scope:       "Cluster"
- name: d8-user-authz-module-multitenancy-related-options.deckhouse.io
  includeSnapshotsFrom: ["{CLUSTER_AUTH_RULES_SNAPSHOT_NAME}"]
  matchConditions:
  - name: "only-user-authz-module"
    expression: 'request.name == "user-authz"'
  rules:
  - apiGroups: ["deckhouse.io"]
    apiVersions: ["*"]
    resources: ["moduleconfigs"]
    operations: ["CREATE", "UPDATE"]
    scope: "Cluster"

kubernetes:
- name: {MODULE_CONFIG_SNAPSHOT_NAME}
  apiVersion: deckhouse.io/v1alpha1
  kind: ModuleConfig
  executeHookOnEvent: []
  executeHookOnSynchronization: true
  keepFullObjectsInMemory: true
  nameSelector:
    matchNames:
    - user-authz
- name: {CLUSTER_AUTH_RULES_SNAPSHOT_NAME}
  apiVersion: deckhouse.io/v1alpha1
  kind: ClusterAuthorizationRule
  keepFullObjectsInMemory: true
  executeHookOnEvent: []
  executeHookOnSynchronization: false
"""


def main(ctx: hook.Context):
    try:
        binding_context = DotMap(ctx.binding_context)
        error_messages, warnings = validate(binding_context)
        if error_messages:
            ctx.output.validations.deny(SEPARATOR.join(error_messages))
        else:
            ctx.output.validations.allow(*warnings)
    except Exception as e:
        ctx.output.validations.error(str(e))


def validate(ctx: DotMap) -> tuple[list[str], list[str]]:
    req = ctx.review.request
    kind = req.kind.kind.lower()

    if kind == "clusterauthorizationrule":
        # don't check ClusterAuthorizationRule if user-authz MultiTenancy option is enabled
        moduleconfig_snapshot = ctx.snapshots[MODULE_CONFIG_SNAPSHOT_NAME]
        if len(moduleconfig_snapshot) != 0 and moduleconfig_snapshot[0].object.spec.settings.enableMultiTenancy is True:
            return [], []

        return validate_car_multitenancy_related_fields(req.object)
    elif kind == "moduleconfig":
        # don't check ClusterAuthorizationRule if user-authz MultiTenancy option is enabled
        if req.object.spec.settings.enableMultiTenancy is True:
            return [], []

        errors, warnings = [], []
        for cluster_authorization_rule in ctx.snapshots.get(CLUSTER_AUTH_RULES_SNAPSHOT_NAME, []):
            error_messages, warning_messages = validate_car_multitenancy_related_fields(cluster_authorization_rule.object)
            if warning_messages:
              warnings.append(SEPARATOR.join(warning_messages))
            if error_messages:
              errors.append(SEPARATOR.join(error_messages))

        return errors, []

    return [], []


MULTITENANCY_RESTRICTED_FIELDS = {
    'allowAccessToSystemNamespaces': "allowAccessToSystemNamespaces flag",
    'namespaceSelector': "namespaceSelector option",
    'limitNamespaces': "limitNamespaces option"
}

def validate_car_multitenancy_related_fields(obj: DotMap) -> tuple[list[str], list[str]]:
    errors = []
    resource_name = obj.metadata.name

    for field, description in MULTITENANCY_RESTRICTED_FIELDS.items():
        if field in obj.spec:
            errors.append(
                f"You must enable userAuthz.enableMultiTenancy to use the {description} "
                f"in ClusterAuthorizationRule '{resource_name}' (EE Only)"
            )

    return errors, []


if __name__ == "__main__":
    hook.run(main, config=CONFIG)
