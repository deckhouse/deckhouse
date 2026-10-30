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

# The operator template imports these as well; they make the file importable by the tests.
from typing import Optional

from dotmap import DotMap

# Closes the two ways around the heritage protection of label-objects.deckhouse.io that the policy
# itself cannot see:
#
#   1. CONNECT on pods/exec, pods/attach and pods/portforward of a Pod labeled `heritage: deckhouse`.
#      Port-forward enters the network namespace of the pod, so a service that trusts localhost
#      trusts whoever may forward a port to it. Denied in every namespace, superadmins included.
#
#   2. UPDATE of the scale subresource of a Deployment, StatefulSet, ReplicaSet or
#      ReplicationController labeled `heritage: deckhouse`. The policy matches `*`, which leaves
#      subresources out, and its binding selects by the object labels, which the Scale object does
#      not carry. Denied unless the parent is under maintenance, which is what the heritage binding
#      of the policy exempts as well. The rules route these kinds only, and the webhook-handler may
#      read only these; the handler itself reads whatever parent the request names.
#
# Neither admission object carries the labels the decision is made of: a CONNECT reviews a
# PodExecOptions / PodAttachOptions / PodPortForwardOptions, and a scale reviews a Scale, whose
# metadata holds the name, namespace and uid of the parent and nothing else. So the hook reads the
# target live, one object by name, the way the user-authz exec webhook on the same handler does: no
# standing watch on every Pod and every scalable kind, and nothing to wait for before the hook can
# answer.
#
# Who may do all of this is decided by the matchConditions of the ValidationWebhook, which repeat the
# exemptions of label-objects.deckhouse.io and add `system:masters`; a request that reaches the hook
# comes from someone the
# heritage protection applies to.

import json
import subprocess
from urllib.parse import quote

HERITAGE_LABEL = "heritage"
HERITAGE_VALUE = "deckhouse"

# An object under maintenance is left to the user by the heritage-label-objects binding: Deckhouse
# no longer reconciles it, and its replicas are the user's to set. The values are the ones that
# binding matches.
MAINTENANCE_LABEL = "maintenance.deckhouse.io/no-resource-reconciliation"
MAINTENANCE_VALUES = {"", "true"}

CONNECT_SUBRESOURCES = {"exec", "attach", "portforward"}

# kubectl ships in the webhook-handler image; see webhooks/validating/system_resources.py of
# user-authz for why the reads do not go through Python's own HTTP client. `--raw` takes the path as
# it is, so a read costs no discovery round trip.
#
# No other flag may go with it. kubectl uses its in-cluster config only while no client-config flag
# is set; with `--request-timeout=3s` the kubectl of the image dialed localhost:8080 instead, every
# read failed, and the handler, failing closed, denied exec and scale on every target, labeled or
# not. The same goes for `--server`, `--kubeconfig` and the other client-config flags.
_KUBECTL = "kubectl"
# The read is bounded by the subprocess instead, below the timeoutSeconds of the ValidationWebhook,
# so a slow API server ends in this hook's own message instead of an apiserver timeout.
_API_TIMEOUT_SECONDS = 3


def _get(path: str) -> Optional[dict]:
    """Reads one object by its API path. None when it does not exist, an exception on any other
    failure: the handler fails closed, and an unreadable target is denied."""
    proc = subprocess.run(
        [_KUBECTL, "get", "--raw", path],
        capture_output=True, text=True, timeout=_API_TIMEOUT_SECONDS, check=False,
    )
    if proc.returncode != 0:
        stderr = (proc.stderr or "").strip()
        if "NotFound" in stderr:
            return None
        raise RuntimeError(f"cannot read {path}: {stderr}")
    return json.loads(proc.stdout)


def _segment(value: str) -> str:
    return quote(value, safe="")


def object_path(group: str, version: str, resource: str, namespace: str, name: str) -> str:
    """The API path of a named object, without the subresource."""
    prefix = f"/apis/{_segment(group)}/{_segment(version)}" if group else f"/api/{_segment(version)}"
    if namespace:
        prefix += f"/namespaces/{_segment(namespace)}"
    return f"{prefix}/{_segment(resource)}/{_segment(name)}"


def _labels(obj: Optional[dict]) -> dict:
    return ((obj or {}).get("metadata") or {}).get("labels") or {}


def is_heritage(labels: dict) -> bool:
    return labels.get(HERITAGE_LABEL) == HERITAGE_VALUE


def under_maintenance(labels: dict) -> bool:
    return MAINTENANCE_LABEL in labels and labels[MAINTENANCE_LABEL] in MAINTENANCE_VALUES


def validate(ctx: DotMap) -> tuple[Optional[str], bool]:
    request = ctx.review.request
    subresource = request.subResource or ""

    if request.operation == "CONNECT" and request.resource.resource == "pods" \
            and subresource in CONNECT_SUBRESOURCES:
        return validate_connect(request, subresource)
    if request.operation == "UPDATE" and subresource == "scale":
        return validate_scale(request)
    return None, True


def validate_connect(request: DotMap, subresource: str) -> tuple[Optional[str], bool]:
    namespace = request.namespace or ""
    name = request.name or ""

    pod = _get(object_path("", "v1", "pods", namespace, name))
    # A pod that is gone has nothing to protect; the API server answers NotFound on its own.
    if not is_heritage(_labels(pod)):
        return None, True

    return (
        f'{subresource} into Pod "{name}" in namespace "{namespace}" is forbidden: the Pod is '
        f"labeled `{HERITAGE_LABEL}: {HERITAGE_VALUE}`, and only Deckhouse system users may "
        "exec, attach or port-forward into the platform's own Pods.",
        False,
    )


def validate_scale(request: DotMap) -> tuple[Optional[str], bool]:
    resource = request.resource
    namespace = request.namespace or ""
    name = request.name or ""

    parent = _get(object_path(resource.group or "", resource.version or "", resource.resource or "",
                              namespace, name))
    labels = _labels(parent)
    if not is_heritage(labels) or under_maintenance(labels):
        return None, True

    # request.kind is Scale here; the kind of the parent comes with the parent.
    kind = (parent or {}).get("kind") or "object"
    where = f' in namespace "{namespace}"' if namespace else ""
    return (
        f'Scaling {resource.resource} "{name}"{where} is forbidden: the {kind} is labeled '
        f"`{HERITAGE_LABEL}: {HERITAGE_VALUE}`, and only Deckhouse system users may change "
        "the objects the platform manages.",
        False,
    )
