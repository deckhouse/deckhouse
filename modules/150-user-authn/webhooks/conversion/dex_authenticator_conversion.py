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

# v1 and v1alpha1 describe the first application in spec itself and the rest in
# spec.additionalApplications; v2alpha1 lists every application in spec.applications. The output
# is the one of the jq programs this hook replaced, including the keys they left with a null value.
#
# templates/dex-authenticator-conversion-webhook.yaml embeds this file in the ConversionWebhook of
# dexauthenticators.deckhouse.io. webhook-operator pastes it into the body of a class Conversion, a
# deckhouse.utils.BaseConversionHook: the method named after a conversion converts one object of the
# version it converts from, and the rest are class attributes. The helpers are static methods and
# reach each other through Conversion, the name the template gives the class.

LISTENER_SET_FIELDS = ("Name", "Namespace", "SectionName")


@staticmethod
def _get(value, key):
    """jq's .key: null stays null, anything but an object is an error."""
    if value is None:
        return None
    if not isinstance(value, dict):
        raise TypeError(f"cannot index {type(value).__name__} with {key!r}")
    return value.get(key)


@staticmethod
def _first(value):
    """jq's .[0]."""
    if value is None:
        return None
    if not isinstance(value, list):
        raise TypeError(f"cannot index {type(value).__name__} with 0")
    return value[0] if value else None


@staticmethod
def _truthy(value) -> bool:
    return value is not None and value is not False


@staticmethod
def _or_null(value):
    return value if Conversion._truthy(value) else None


@staticmethod
def _without_nulls(value: dict) -> dict:
    return {k: v for k, v in value.items() if v is not None}


def v2alpha1_to_v1(self, obj: dict) -> tuple[str | None, dict]:
    obj["apiVersion"] = "deckhouse.io/v1"
    spec = obj.get("spec")
    apps = self._get(spec, "applications")
    first = self._first(apps)
    gateway = self._get(first, "gatewayAPI")
    additional = apps[1:] if apps and len(apps) > 1 else None

    spec = dict(spec or {})
    spec.update({
        "applicationDomain": self._get(first, "domain"),
        "applicationIngressClassName": self._get(first, "ingressClassName"),
        "applicationIngressCertificateSecretName": self._or_null(self._get(first, "ingressSecretName")),
        "gatewayAPI": {
            f"applicationHTTPRouteListenerSet{field}": self._or_null(self._get(gateway, f"httpRouteListenerSet{field}"))
            for field in self.LISTENER_SET_FIELDS
        },
        "signOutURL": self._or_null(self._get(first, "signOutURL")),
        "whitelistSourceRanges": self._or_null(self._get(first, "whitelistSourceRanges")),
        "additionalApplications": additional,
    })
    if spec["applicationIngressClassName"] is None:
        del spec["applicationIngressClassName"]
    if gateway is None:
        del spec["gatewayAPI"]
    else:
        spec["gatewayAPI"] = self._without_nulls(spec["gatewayAPI"])
    spec.pop("applications", None)
    if spec["additionalApplications"] is None:
        del spec["additionalApplications"]
    obj["spec"] = spec
    return None, obj


def v1_to_v2alpha1(self, obj: dict) -> tuple[str | None, dict]:
    obj["apiVersion"] = "deckhouse.io/v2alpha1"
    spec = obj.get("spec")
    gateway = self._get(spec, "gatewayAPI")
    additional = self._get(spec, "additionalApplications")
    additional = additional if self._truthy(additional) else []
    if not isinstance(additional, list):
        raise TypeError("spec.additionalApplications is not a list")

    first = {
        "domain": self._get(spec, "applicationDomain"),
        "ingressClassName": self._get(spec, "applicationIngressClassName"),
        "gatewayAPI": {
            f"httpRouteListenerSet{field}": self._get(gateway, f"applicationHTTPRouteListenerSet{field}")
            for field in self.LISTENER_SET_FIELDS
        },
    }
    for source, target in (
        ("applicationIngressCertificateSecretName", "ingressSecretName"),
        ("signOutURL", "signOutURL"),
        ("whitelistSourceRanges", "whitelistSourceRanges"),
    ):
        if self._truthy(spec.get(source) if spec else None):
            first[target] = spec[source]
    if first["ingressClassName"] is None:
        del first["ingressClassName"]
    if first["gatewayAPI"]["httpRouteListenerSetName"] is None:
        del first["gatewayAPI"]
    else:
        first["gatewayAPI"] = self._without_nulls(first["gatewayAPI"])

    spec = dict(spec or {})
    spec.setdefault("sendAuthorizationHeader", None)
    spec["applications"] = [first] + additional
    for key in ("applicationDomain", "applicationIngressClassName", "gatewayAPI",
                "applicationIngressCertificateSecretName", "additionalApplications", "signOutURL",
                "whitelistSourceRanges"):
        spec.pop(key, None)
    obj["spec"] = spec
    return None, obj


def v1_to_v1alpha1(self, obj: dict) -> tuple[str | None, dict]:
    obj["apiVersion"] = "deckhouse.io/v1alpha1"
    return None, obj


def v1alpha1_to_v1(self, obj: dict) -> tuple[str | None, dict]:
    obj["apiVersion"] = "deckhouse.io/v1"
    return None, obj
