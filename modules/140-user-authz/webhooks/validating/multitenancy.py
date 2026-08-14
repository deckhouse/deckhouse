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
# The effective value of enableMultiTenancy (the ModuleConfig user setting
# merged with config-schema defaults) is not something an admission webhook
# can see directly — webhooks only see real Kubernetes objects, not module
# Values. This is necessary because in some editions (e.g. CSE)
# enableMultiTenancy defaults to true, and when the user has not set it
# explicitly, the field is simply absent from ModuleConfig.spec.settings.
#
# The discoverMultitenancyState hook (see hooks/discover_multitenancy_state.go)
# bridges this gap: it runs on every module reconcile, reads the already
# defaults-merged input.Values.Get("userAuthz.enableMultiTenancy"), and keeps
# it mirrored into the "d8-user-authz-multitenancy-state" ConfigMap. This hook
# reads that ConfigMap instead of ModuleConfig or the Module CR.
#
# (Do not go back to reading status.lastAppliedConfiguration from a Module CR
# here — deckhouse.io/v1alpha2 Module is not a real, served API version for
# built-in modules like this one; it belongs to the unrelated, in-progress
# packages/Application controller. Doing so silently makes this check always
# treat MultiTenancy as disabled.)
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
#   If the request doesn't touch enableMultiTenancy at all (the field is absent from the submitted
#   spec.settings, e.g. an unrelated ModuleConfig edit), we fall back to the ConfigMap-mirrored
#   effective value instead of treating it as "disabled".


from deckhouse import hook
from dotmap import DotMap

SEPARATOR = "; "
MULTITENANCY_STATE_SNAPSHOT_NAME = "d8-user-authz-multitenancy-state"
CLUSTER_AUTH_RULES_SNAPSHOT_NAME = "d8-user-authz-cars"
CONFIG = f"""
configVersion: v1
kubernetesValidating:
- name: d8-user-authz-car-multitenancy-related-options.deckhouse.io
  includeSnapshotsFrom: ["{MULTITENANCY_STATE_SNAPSHOT_NAME}"]
  rules:
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["*"]
    operations:  ["CREATE", "UPDATE"]
    resources:   ["clusterauthorizationrules"]
    scope:       "Cluster"
- name: d8-user-authz-module-multitenancy-related-options.deckhouse.io
  includeSnapshotsFrom: ["{CLUSTER_AUTH_RULES_SNAPSHOT_NAME}", "{MULTITENANCY_STATE_SNAPSHOT_NAME}"]
  matchConditions:
  - name: "only-user-authz-module"
    expression: 'request.name == "user-authz"'
  rules:
  - apiGroups: ["deckhouse.io"]
    apiVersions: ["*"]
    resources: ["moduleconfigs"]
    operations: ["CREATE", "UPDATE"]
    scope:       "Cluster"

kubernetes:
- name: {MULTITENANCY_STATE_SNAPSHOT_NAME}
  apiVersion: v1
  kind: ConfigMap
  executeHookOnEvent: []
  executeHookOnSynchronization: true
  keepFullObjectsInMemory: false
  jqFilter: |
    {{
      "enableMultiTenancy": (.data.enableMultiTenancy == "true")
    }}
  namespace:
    nameSelector:
      matchNames:
      - d8-user-authz
  nameSelector:
    matchNames:
    - d8-user-authz-multitenancy-state
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


def is_multitenancy_enabled(ctx: DotMap) -> bool:
    snapshot = ctx.snapshots[MULTITENANCY_STATE_SNAPSHOT_NAME]
    return len(snapshot) != 0 and snapshot[0].filterResult.enableMultiTenancy is True


def validate(ctx: DotMap) -> tuple[list[str], list[str]]:
    req = ctx.review.request
    kind = req.kind.kind.lower()

    if kind == "clusterauthorizationrule":
        # don't check ClusterAuthorizationRule if user-authz MultiTenancy option is enabled
        if is_multitenancy_enabled(ctx):
            return [], []

        return validate_car_multitenancy_related_fields(req.object)
    elif kind == "moduleconfig":
        settings = req.object.spec.settings
        field_present = "enableMultiTenancy" in settings

        # don't check existing CARs if the request explicitly enables MultiTenancy...
        if field_present and settings.enableMultiTenancy is True:
            return [], []
        # ...or if the request doesn't touch enableMultiTenancy at all (it's absent from
        # spec.settings, e.g. an unrelated ModuleConfig edit) and it's still effectively
        # enabled — explicitly, or via the edition's config-schema default.
        if not field_present and is_multitenancy_enabled(ctx):
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
                f"in ClusterAuthorizationRule '{resource_name}'"
            )

    return errors, []


if __name__ == "__main__":
    hook.run(main, config=CONFIG)
