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

# v1alpha1 has one selector, projectSelector, matched against the labels of a namespace. v1alpha2
# splits it: projectSelector is matched against the labels of the Project, namespaceSelector against
# the labels of the namespace. So the v1alpha1 projectSelector is the v1alpha2 namespaceSelector, one
# for one. The v1alpha2 projectSelector has no v1alpha1 field; while the object is served as
# v1alpha1 it is kept, as JSON, in the multitenancy.deckhouse.io/v1alpha2-project-selector
# annotation, so a round trip loses nothing.
#
# The annotation is the only part of a v1alpha1 write that becomes a v1alpha2 field without passing
# the v1alpha2 schema: the API server validates the v1alpha1 request, where it is just a string, and
# stores whatever the conversion returns. So the up conversion accepts it only when it is a label
# selector; anything else is refused, because a projectSelector of the wrong type would make every
# typed list of the policies fail to decode, in the controller and in the grant webhooks alike. The
# down conversion always derives the annotation from spec.projectSelector, so a stale copy written
# through v1alpha2 never comes back as a selector. shell-operator hands the API server its own
# "Conversion ... was not successful" for a refused object, not the failedMessage below.
#
# templates/clusterresourcegrantpolicies-conversion-webhook.yaml embeds this file in the
# ConversionWebhook of clusterresourcegrantpolicies.multitenancy.deckhouse.io. webhook-operator pastes
# it into the body of a class Conversion, a deckhouse.utils.BaseConversionHook: the method named
# after a conversion converts one object of the version it converts from, and the rest are class
# attributes. A refused annotation is the error message of the conversion. The helpers are static
# methods and reach each other through Conversion, the name the template gives the class.

ANNOTATION = "multitenancy.deckhouse.io/v1alpha2-project-selector"
UP_FAILED = (
    "Conversion of clusterresourcegrantpolicies.multitenancy.deckhouse.io failed: the "
    "multitenancy.deckhouse.io/v1alpha2-project-selector annotation must be a JSON label selector "
    "(matchLabels with string values; matchExpressions with a string key, an operator In, NotIn, Exists "
    "or DoesNotExist, and string values). Remove the annotation to drop the projectSelector."
)
OPERATORS = ("In", "NotIn", "Exists", "DoesNotExist")


@staticmethod
def _strings_only(value) -> bool:
    return all(isinstance(v, str) for v in (value.values() if isinstance(value, dict) else value))


@staticmethod
def _null_or(value, kind, check) -> bool:
    return value is None or (isinstance(value, kind) and check(value))


@staticmethod
def _is_label_selector(value) -> bool:
    return (
        isinstance(value, dict)
        and set(value) <= {"matchLabels", "matchExpressions"}
        and Conversion._null_or(value.get("matchLabels"), dict, Conversion._strings_only)
        and Conversion._null_or(value.get("matchExpressions"), list, lambda expressions: all(
            isinstance(e, dict)
            and set(e) <= {"key", "operator", "values"}
            and isinstance(e.get("key"), str)
            and e.get("operator") in Conversion.OPERATORS
            and Conversion._null_or(e.get("values"), list, Conversion._strings_only)
            for e in expressions
        ))
    )


@staticmethod
def _parse(text):
    import json
    try:
        return json.loads(text)
    except ValueError:
        return None


@staticmethod
def _tojson(value) -> str:
    """jq's tojson: compact, UTF-8, with DEL escaped."""
    import json
    return json.dumps(value, separators=(",", ":"), ensure_ascii=False).replace("\x7f", "\\u007f")


@staticmethod
def _drop_annotation(obj: dict):
    metadata = obj.get("metadata") or {}
    annotations = metadata.get("annotations")
    if annotations is None:
        return
    annotations.pop(Conversion.ANNOTATION, None)
    if not annotations:
        del metadata["annotations"]


def v1alpha1_to_v1alpha2(self, obj: dict) -> tuple[str | None, dict]:
    obj["apiVersion"] = "multitenancy.deckhouse.io/v1alpha2"
    kept = ((obj.get("metadata") or {}).get("annotations") or {}).get(self.ANNOTATION)
    spec = obj.get("spec")
    if spec is not None and spec is not False:
        namespace_selector = spec.pop("projectSelector", None)
        spec.pop("namespaceSelector", None)
        if namespace_selector is not None:
            spec["namespaceSelector"] = namespace_selector
        if kept is not None:
            project_selector = self._parse(kept)
            if not self._is_label_selector(project_selector):
                return self.UP_FAILED, obj
            spec["projectSelector"] = project_selector
    self._drop_annotation(obj)
    return None, obj


def v1alpha2_to_v1alpha1(self, obj: dict) -> tuple[str | None, dict]:
    obj["apiVersion"] = "multitenancy.deckhouse.io/v1alpha1"
    self._drop_annotation(obj)
    spec = obj.get("spec")
    if spec is not None and spec is not False:
        project_selector = spec.pop("projectSelector", None)
        namespace_selector = spec.pop("namespaceSelector", None)
        if namespace_selector is not None:
            spec["projectSelector"] = namespace_selector
        if project_selector is not None:
            if obj.get("metadata") is None:
                obj["metadata"] = {}
            metadata = obj["metadata"]
            if metadata.get("annotations") is None:
                metadata["annotations"] = {}
            metadata["annotations"][self.ANNOTATION] = self._tojson(project_selector)
    return None, obj
