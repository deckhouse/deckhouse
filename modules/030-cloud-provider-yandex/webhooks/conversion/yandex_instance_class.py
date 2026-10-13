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

# handler.python of the ConversionWebhook yandexinstanceclasses.deckhouse.io, both steps.
# webhook-operator pastes it into the Conversion class of its template; BaseConversionHook calls the
# method named after the step, for the objects in its source version only.
def v1alpha1_to_v1(self, o: dict) -> tuple[str | None, dict]:
    spec = o.get("spec")
    if spec is not None and not isinstance(spec, dict):
        return "Conversion of yandexinstanceclasses.deckhouse.io failed", o
    converted = {**o, "apiVersion": "deckhouse.io/v1"}
    network_type = (spec or {}).get("networkType")
    network_types = {"STANDARD": "Standard", "SOFTWARE_ACCELERATED": "SoftwareAccelerated"}
    if isinstance(network_type, str) and network_type in network_types:
        converted["spec"] = {**spec, "networkType": network_types[network_type]}
    return None, converted


def v1_to_v1alpha1(self, o: dict) -> tuple[str | None, dict]:
    spec = o.get("spec")
    if spec is not None and not isinstance(spec, dict):
        return "Conversion of yandexinstanceclasses.deckhouse.io failed", o
    converted = {**o, "apiVersion": "deckhouse.io/v1alpha1"}
    network_type = (spec or {}).get("networkType")
    network_types = {"Standard": "STANDARD", "SoftwareAccelerated": "SOFTWARE_ACCELERATED"}
    if isinstance(network_type, str) and network_type in network_types:
        converted["spec"] = {**spec, "networkType": network_types[network_type]}
    return None, converted
