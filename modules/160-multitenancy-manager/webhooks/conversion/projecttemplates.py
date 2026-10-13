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

# v1alpha1 is served and deprecated, v1alpha2 is the storage version, and the CRD declares a
# conversion webhook, so the apiserver asks for a conversion whenever a client uses the version that
# is not stored and whenever it reads an object still stored at the other one. A CRD pointing at a
# webhook that cannot convert is a CRD whose objects can stop being readable. Removing the hook made
# every such request fail: a live cluster answered about one a second, all of them counted as
# conversion failures.
#
# The conversion is the version bump and nothing else. Both versions name description,
# parametersSchema and resourcesTemplate identically, so a Helm template survives the round trip.
# The structured fields v1alpha2 added exist only there: they are pruned on the way down and cannot
# come back, so a write through v1alpha1 would erase them, and the template webhook refuses one to a
# template that has them.

# templates/projecttemplates-conversion-webhook.yaml embeds this file in the ConversionWebhook of
# projecttemplates.deckhouse.io. webhook-operator pastes it into the body of a class Conversion, a
# deckhouse.utils.BaseConversionHook: the method named after a conversion converts one object of the
# version it converts from.


@staticmethod
def _or(value, default):
    """jq's `value // default`."""
    return default if value is None or value is False else value


def v1alpha1_to_v1alpha2(self, obj: dict) -> tuple[str | None, dict]:
    obj["apiVersion"] = "deckhouse.io/v1alpha2"
    return None, obj


# The two fields the v1alpha1 schema requires are backfilled when a structured template omits
# them. The apiserver checks only the metadata of what a conversion returns, but a client that
# reads a template through v1alpha1 and writes it back is validated against the v1alpha1 schema,
# which requires both, so a template that never had a resourcesTemplate would read back as an
# object that cannot be written.
def v1alpha2_to_v1alpha1(self, obj: dict) -> tuple[str | None, dict]:
    obj["apiVersion"] = "deckhouse.io/v1alpha1"
    spec = self._or(obj.get("spec"), {})
    spec["resourcesTemplate"] = self._or(spec.get("resourcesTemplate"), "")
    schema = spec["parametersSchema"] = self._or(spec.get("parametersSchema"), {"openAPIV3Schema": {}})
    schema["openAPIV3Schema"] = self._or(schema.get("openAPIV3Schema"), {})
    obj["spec"] = spec
    return None, obj
