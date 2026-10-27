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

# With Module v2 (deckhouse.enableModuleV2) v1beta1 is the storage version and carries no
# properties; v1alpha1 is filled from the v1beta1 spec, status and annotations so that its readers
# (kubectl, hooks reading .properties) keep working. v1alpha1 to v1beta1 is trivial: pkgsync
# rewrites every Module in v1beta1, and v1alpha1 has nothing v1beta1 can take.
#
# templates/module-conversion-webhook.yaml embeds this file in the ConversionWebhook of
# modules.deckhouse.io. webhook-operator pastes it into the body of a class Conversion, a
# deckhouse.utils.BaseConversionHook: the method named after a conversion converts one object of the
# version it converts from. The helpers are static methods and reach each other through Conversion.

ANNOTATION_EMBEDDED = "modules.deckhouse.io/embedded"
ANNOTATION_CRITICAL = "modules.deckhouse.io/critical"

# summary.state of v1beta1 to status.phase of v1alpha1; a state not listed leaves the phase unset.
PHASE_BY_STATE = {
    "Ready": "Ready",
    "Degraded": "Error",
    "Failed": "Error",
    "Updating": "Reconciling",
    "Configuring": "Reconciling",
    "Deleting": "Reconciling",
    "Pending": "Installing",
    "Suspended": "Available",
}


@staticmethod
def _phase(status: dict) -> str | None:
    # a disabled module was Available in v1alpha1 whatever it went through before
    if status.get("enabled") is False:
        return "Available"
    state = (status.get("summary") or {}).get("state")
    if not state:
        return "Available"
    return Conversion.PHASE_BY_STATE.get(state)


@staticmethod
def _conditions(status: dict) -> list:
    conditions = []

    enabled = status.get("enabled")
    if enabled is not None:
        condition = {"type": "EnabledByModuleManager", "status": "True" if enabled else "False"}
        if not enabled:
            condition["reason"] = "Disabled"
            condition["message"] = "disabled"
        conditions.append(condition)

    for condition in status.get("conditions") or []:
        if condition.get("type") != "Ready":
            continue
        ready = {"type": "IsReady", "status": condition.get("status", "Unknown")}
        for key in ("reason", "message", "lastTransitionTime"):
            if condition.get(key):
                ready[key] = condition[key]
        conditions.append(ready)

    return conditions


def v1beta1_to_v1alpha1(self, obj: dict) -> tuple[str | None, dict]:
    obj["apiVersion"] = "deckhouse.io/v1alpha1"

    annotations = (obj.get("metadata") or {}).get("annotations") or {}
    spec = obj.get("spec") or {}
    status = obj.get("status") or {}
    current = status.get("currentVersion") or {}

    properties = {}
    if current.get("version"):
        properties["version"] = current["version"]
    release_channel = spec.get("releaseChannel") or current.get("channel")
    if release_channel:
        properties["releaseChannel"] = release_channel
    if annotations.get(self.ANNOTATION_EMBEDDED) == "true":
        properties["source"] = "Embedded"
    elif spec.get("packageRepositoryName"):
        properties["source"] = spec["packageRepositoryName"]
    if annotations.get(self.ANNOTATION_CRITICAL) == "true":
        properties["critical"] = True
    obj["properties"] = properties

    converted = {}
    phase = self._phase(status)
    if phase:
        converted["phase"] = phase
    conditions = self._conditions(status)
    if conditions:
        converted["conditions"] = conditions
    obj["status"] = converted

    return None, obj


def v1alpha1_to_v1beta1(self, obj: dict) -> tuple[str | None, dict]:
    obj["apiVersion"] = "deckhouse.io/v1beta1"
    obj.pop("properties", None)
    obj.pop("status", None)
    return None, obj
