# Copyright 2021 Flant JSC
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

# The versions differ in the case of spec.github.teamNameField only: name, slug, both in v1alpha1,
# Name, Slug, Both in v1.
#
# templates/dex-provider-conversion-webhook.yaml embeds this file in the ConversionWebhook of
# dexproviders.deckhouse.io. webhook-operator pastes it into the body of a class Conversion, a
# deckhouse.utils.BaseConversionHook: the method named after a conversion converts one object of the
# version it converts from.


@staticmethod
def _recase_team_name_field(obj: dict, upper: bool):
    github = (obj.get("spec") or {}).get("github") or {}
    field = github.get("teamNameField")
    if field is None or field is False:
        return
    first = field[:1]
    if ("a" <= first <= "z") if upper else ("A" <= first <= "Z"):
        github["teamNameField"] = (first.upper() if upper else first.lower()) + field[1:]


def v1alpha1_to_v1(self, obj: dict) -> tuple[str | None, dict]:
    obj["apiVersion"] = "deckhouse.io/v1"
    self._recase_team_name_field(obj, upper=True)
    return None, obj


def v1_to_v1alpha1(self, obj: dict) -> tuple[str | None, dict]:
    obj["apiVersion"] = "deckhouse.io/v1alpha1"
    self._recase_team_name_field(obj, upper=False)
    return None, obj
