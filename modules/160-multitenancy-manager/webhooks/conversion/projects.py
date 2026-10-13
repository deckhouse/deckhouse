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

# templates/projects-conversion-webhook.yaml embeds this file in the ConversionWebhook of
# projects.deckhouse.io. webhook-operator pastes it into the body of a class Conversion, a
# deckhouse.utils.BaseConversionHook: the method named after a conversion converts one object of the
# version it converts from, and the rest are class attributes. The helpers are static methods and
# reach each other through Conversion, the name the template gives the class.

import re

# A quantity is a number or a string in a subset of the syntax ParseQuantity reads. The exponent has
# at most three digits and the string at most 64 characters: a longer exponent can keep
# ParseQuantity busy for seconds, and a longer mantissa the canonical form the controller computes
# each time it writes the project, which it does on every reconcile.
QUANTITY = re.compile(r"[+-]?([0-9]+(\.[0-9]*)?|\.[0-9]+)([KMGTPE]i|[numkMGTPE]|[eE][+-]?[0-9]{1,3})?")


@staticmethod
def _truthy(value) -> bool:
    return value is not None and value is not False


@staticmethod
def _get(value, key):
    """jq's .key: null stays null, anything but an object is an error."""
    if value is None:
        return None
    if not isinstance(value, dict):
        raise TypeError(f"cannot index {type(value).__name__} with {key!r}")
    return value.get(key)


@staticmethod
def _is_number(value) -> bool:
    return isinstance(value, (int, float)) and not isinstance(value, bool)


@staticmethod
def _quota_value(value):
    """A whole number stays a number, anything else becomes a string. spec.quota is
    anyOf[integer, string], so a fractional "cpu: 0.5" is only representable as a string, while
    stringifying "cpu: 1" as well would make the round trip back to v1alpha2 differ from what the
    client wrote and leave kubectl apply with a diff it can never settle."""
    import math
    from decimal import Decimal

    if Conversion._is_number(value) and value == math.floor(value):
        return value
    if isinstance(value, str):
        return value
    # The text jq gives a fraction: the shortest digits, scientific below 1e-6.
    return str(Decimal(repr(value)))


@staticmethod
def _is_quantity(value) -> bool:
    return Conversion._is_number(value) or (
        isinstance(value, str) and len(value) <= 64 and Conversion.QUANTITY.fullmatch(value) is not None
    )


@staticmethod
def _is_liftable_value(value) -> bool:
    return Conversion._is_quantity(value) and not (Conversion._is_number(value) and value == 0)


@staticmethod
def _is_liftable_administrators(value) -> bool:
    return isinstance(value, list) and len(value) > 0 and all(
        isinstance(item, dict)
        and sorted(item) == ["name", "subject"]
        and item["subject"] in ("User", "Group")
        and isinstance(item["name"], str)
        and item["name"] != ""
        and all(ord(char) > 31 and ord(char) != 127 for char in item["name"])
        for item in value
    )


@staticmethod
def _is_liftable_quota(value) -> bool:
    if not isinstance(value, dict) or not value:
        return False
    for key, item in value.items():
        if key in ("requests", "limits") and isinstance(item, dict):
            if not item or not all(Conversion._is_liftable_value(v) for v in item.values()):
                return False
        elif not Conversion._is_liftable_value(item) or key.startswith(("requests.", "limits.")):
            return False
    return True


@staticmethod
def _lift_quota(quota: dict) -> dict:
    lifted = {}
    for prefix in ("requests", "limits"):
        if isinstance(quota.get(prefix), dict):
            lifted.update({f"{prefix}.{key}": Conversion._quota_value(value) for key, value in quota[prefix].items()})
    lifted.update({key: Conversion._quota_value(value) for key, value in quota.items() if not isinstance(value, (dict, list))})
    return lifted


@staticmethod
def _zero_as_string(value):
    return "0" if Conversion._is_number(value) and value == 0 else value


@staticmethod
def _nest_quota(quota: dict) -> dict:
    """Only a "requests."/"limits." prefix means nesting; the rest of the key stays intact, so
    "requests.nvidia.com/gpu" lands under requests as "nvidia.com/gpu". Every other key stays flat.
    The nested form wins over a bare "requests" key whatever the order of the keys."""
    nested = {}
    for key, value in quota.items():
        parts = key.split(".")
        if len(parts) > 1 and parts[0] in ("requests", "limits"):
            group = nested.get(parts[0])
            nested[parts[0]] = {**(group if isinstance(group, dict) else {}), ".".join(parts[1:]): Conversion._zero_as_string(value)}
        elif not isinstance(nested.get(key), dict):
            nested[key] = Conversion._zero_as_string(value)
    return nested


@staticmethod
def _status_namespaces(obj: dict):
    status = obj.get("status")
    return Conversion._get(status if Conversion._truthy(status) else {}, "namespaces")


def v1alpha1_to_v1alpha2(self, obj: dict) -> tuple[str | None, dict]:
    obj["apiVersion"] = "deckhouse.io/v1alpha2"
    self._rename(obj, "projectTypeName", "projectTemplateName")
    self._rename(obj, "template", "parameters")
    return None, obj


def v1alpha2_to_v1alpha1(self, obj: dict) -> tuple[str | None, dict]:
    obj["apiVersion"] = "deckhouse.io/v1alpha1"
    self._rename(obj, "projectTemplateName", "projectTypeName")
    self._rename(obj, "parameters", "template")
    return None, obj


# v1alpha2 -> v1alpha3 lifts administrators and resourceQuota out of spec.parameters into the
# dedicated top-level spec.administrators and spec.quota fields.
#   parameters.administrators[].subject -> administrators[].kind
#   parameters.resourceQuota.{requests,limits}.* -> quota."requests.*"/"limits.*"
#   parameters.resourceQuota.<other>       -> quota."<other>"
# A value is lifted only if v1alpha3 can hold it and v1alpha3_to_v1alpha2 gives it back as it
# was, apart from a fraction, which comes back as a string. Anything else stays in
# spec.parameters untouched, where a template still reads it, and the project gets no standard
# field for it.
#   administrators: a non-empty list whose items hold exactly a subject, User or Group, and a
#     name, a string of at least one character and no control character (the pattern of the
#     v1alpha3 schema).
#   resourceQuota: a non-empty object whose requests/limits, when they are objects, are
#     non-empty and hold quantities, and whose other values are quantities under a key that does
#     not start with "requests." or "limits.". Such a key comes back nested, and next to a nested
#     one it collides with it.
#   One item or value that does not fit keeps the whole list or quota in spec.parameters, and so
#     does an empty one or a quota that holds the number 0 anywhere: the templates of the
#     previous release took the number 0 for no value and set no limit, while in spec.quota it
#     is a hard limit of 0. The string "0" is lifted.
# A null administrators or resourceQuota says nothing and is dropped. status.namespaces turns
# from names into {name, kind} objects, Main for the namespace named after the project.
def v1alpha2_to_v1alpha3(self, obj: dict) -> tuple[str | None, dict]:
    obj["apiVersion"] = "deckhouse.io/v1alpha3"
    spec = obj.get("spec")
    parameters = self._get(spec, "parameters")

    if (not self._truthy(self._get(spec, "administrators"))
            and self._is_liftable_administrators(self._get(parameters, "administrators"))):
        spec["administrators"] = [{"kind": a["subject"], "name": a["name"]} for a in parameters.pop("administrators")]

    if not self._truthy(self._get(spec, "quota")) and self._is_liftable_quota(self._get(parameters, "resourceQuota")):
        spec["quota"] = self._lift_quota(parameters.pop("resourceQuota"))

    if isinstance(parameters, dict):
        for key in ("administrators", "resourceQuota"):
            if parameters.get(key) is None:
                parameters.pop(key, None)

    namespaces = self._status_namespaces(obj)
    if isinstance(namespaces, list):
        main = self._get(obj.get("metadata"), "name")
        obj["status"]["namespaces"] = [
            {"name": ns, "kind": "Main" if ns == main else "Additional"} if isinstance(ns, str) else ns
            for ns in namespaces
        ]
    return None, obj


# v1alpha3 -> v1alpha2 pushes administrators and quota back into spec.parameters so that older
# clients reading v1alpha2 keep seeing the legacy layout. A standard field takes the place of the
# parameter it is lifted from; a parameter the up conversion left in place comes back as it is.
# The number 0 goes back as the string "0", the only 0 the up conversion lifts again.
def v1alpha3_to_v1alpha2(self, obj: dict) -> tuple[str | None, dict]:
    obj["apiVersion"] = "deckhouse.io/v1alpha2"
    spec = obj.get("spec")
    administrators = self._get(spec, "administrators")
    if self._truthy(administrators):
        spec["parameters"] = {**self._parameters(spec), "administrators": [
            {"subject": self._get(a, "kind"), "name": self._get(a, "name")} for a in administrators
        ]}
    quota = self._get(spec, "quota")
    if self._truthy(quota):
        spec["parameters"] = {**self._parameters(spec), "resourceQuota": self._nest_quota(quota)}
    if isinstance(spec, dict):
        spec.pop("administrators", None)
        spec.pop("quota", None)

    namespaces = self._status_namespaces(obj)
    if isinstance(namespaces, list):
        names = [ns.get("name") if isinstance(ns, dict) else ns for ns in namespaces]
        obj["status"]["namespaces"] = [name for name in names if isinstance(name, str)]
    return None, obj


@staticmethod
def _rename(obj: dict, old: str, new: str):
    spec = obj.get("spec")
    value = Conversion._get(spec, old)
    if Conversion._truthy(value):
        spec[new] = value
    if isinstance(spec, dict):
        spec.pop(old, None)


@staticmethod
def _parameters(spec: dict) -> dict:
    parameters = spec.get("parameters")
    return parameters if Conversion._truthy(parameters) else {}
