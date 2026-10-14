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

# handler.python of the ValidationWebhook in
# templates/d8-secret-encryption-key-validation-webhook.yaml; the operator template provides the
# imports and main.

DECKHOUSE_SA = "system:serviceaccount:d8-system:deckhouse"
# Metadata fields that may change on UPDATE.
MUTABLE_METADATA = ("labels", "annotations", "resourceVersion", "managedFields")


def get(obj, *path):
    """jq-like path lookup: null propagates, indexing a non-object fails."""
    for key in path:
        if obj is None:
            return None
        if not isinstance(obj, dict):
            raise ValueError(f"cannot index {type(obj).__name__} with {key!r}")
        obj = obj.get(key)
    return obj


def as_shell_string(value):
    """What `$(jq -r ...)` leaves in a bash variable, for string values: NUL bytes and
    trailing newlines are dropped. Other types can never equal the strings compared with."""
    if isinstance(value, str):
        return value.replace("\0", "").rstrip("\n")
    return None


def without_mutable_metadata(obj):
    if obj is None:
        return None
    metadata = get(obj, "metadata")
    if metadata is None:
        return obj
    if not isinstance(metadata, dict):
        raise ValueError(f"cannot delete fields of {type(metadata).__name__} metadata")
    stripped = {k: v for k, v in metadata.items() if k not in MUTABLE_METADATA}
    return {**obj, "metadata": stripped}


def json_equal(a, b):
    """JSON equality as in jq: booleans never equal numbers (unlike Python's True == 1)."""
    if isinstance(a, bool) or isinstance(b, bool):
        return type(a) is type(b) and a == b
    if isinstance(a, (int, float)) and isinstance(b, (int, float)):
        return a == b
    if type(a) is not type(b):
        return False
    if isinstance(a, dict):
        return a.keys() == b.keys() and all(json_equal(a[k], b[k]) for k in a)
    if isinstance(a, list):
        return len(a) == len(b) and all(json_equal(x, y) for x, y in zip(a, b))
    return a == b


# Called by the main of the ValidationWebhook template, which wraps the binding context in DotMap.
def validate(ctx: DotMap) -> tuple[Optional[str], bool]:
    # The checks below compare plain JSON values, so work on the dict shell-operator sent.
    binding_context = ctx.toDict()
    request = get(binding_context, "review", "request")
    operation = as_shell_string(get(request, "operation"))
    user = as_shell_string(get(request, "userInfo", "username"))

    if operation == "DELETE":
        return "it is forbidden to delete secret d8-secret-encryption-key", False

    if operation == "UPDATE":
        new = without_mutable_metadata(get(request, "object"))
        old = without_mutable_metadata(get(request, "oldObject"))
        if not json_equal(new, old):
            return (
                "it is forbidden to modify secret d8-secret-encryption-key "
                "(only labels and annotations are allowed to be changed)"
            ), False

        if user != DECKHOUSE_SA:
            return (
                f"only {DECKHOUSE_SA} is allowed to modify labels and annotations "
                "of secret d8-secret-encryption-key"
            ), False

    return None, True
