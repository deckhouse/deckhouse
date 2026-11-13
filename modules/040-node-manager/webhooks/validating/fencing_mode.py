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

# Refuses to change the fencing mode of a NodeGroup while a node of the group has a
# FencingFailedNodeState: the deadlines of an open incident come from the mode at its start.
#
# The matchConditions of the ValidationWebhook send here only the updates that change the value of
# spec.fencing.mode. The hook compares the modes after the normalization of the hook node_group_modes
# of the module: a change of the value that keeps the mode reads nothing and is allowed.
#
# The hook reads the states and their Nodes live, as webhooks/validating/heritage.py of the deckhouse
# module does: the states are few, and a mode change is rare. A failed read denies the change.

import json
import subprocess
from urllib.parse import quote

# The chart sets it in Community Edition, where the Critical mode works as Normal.
COMMUNITY_EDITION = False

GROUP_LABEL = "node.deckhouse.io/group"
STATES_PATH = "/apis/node-manager.deckhouse.io/v1alpha1/fencingfailednodestates"

# kubectl ships in the webhook-handler image. No client-config flag may go with `--raw`: with one,
# kubectl does not use its in-cluster config. The subprocess timeout bounds the read.
_KUBECTL = "kubectl"
_API_TIMEOUT_SECONDS = 3


def _get(path: str) -> Optional[dict]:
    """Reads an object or a list by its API path. None when it does not exist, an exception on any
    other failure."""
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


def normalized_mode(node_group: DotMap) -> str:
    """The mode that the group works in. Watchdog is the deprecated name of Slow. Disabled covers the
    old values off and Notify and any value this release does not know."""
    mode = ((node_group.get("spec") or {}).get("fencing") or {}).get("mode") or ""
    if mode == "Critical":
        return "Normal" if COMMUNITY_EDITION else mode
    if mode in ("Normal", "Slow"):
        return mode
    if mode == "Watchdog":
        return "Slow"
    return "Disabled"


def failed_group_nodes(group: str) -> list:
    """The sorted names of the nodes of the group that have a FencingFailedNodeState. The name of a
    state is the name of its node."""
    states = (_get(STATES_PATH) or {}).get("items") or []
    nodes = []
    for state in states:
        name = (state.get("metadata") or {}).get("name") or ""
        node = _get("/api/v1/nodes/" + quote(name, safe="")) if name else None
        labels = ((node or {}).get("metadata") or {}).get("labels") or {}
        if labels.get(GROUP_LABEL) == group:
            nodes.append(name)
    return sorted(nodes)


def validate(ctx: DotMap) -> tuple[Optional[str], bool]:
    request = ctx.review.request
    old_mode = normalized_mode(request.oldObject)
    new_mode = normalized_mode(request.object)
    if old_mode == new_mode:
        return None, True

    nodes = failed_group_nodes(request.name)
    if not nodes:
        return None, True
    return (
        f"it is forbidden to change .spec.fencing.mode from {old_mode} to {new_mode} while nodes of "
        f"the group have a FencingFailedNodeState: {', '.join(nodes)}. "
        "The deadlines of an incident come from the mode at its start. "
        "Wait until the incidents end and these FencingFailedNodeState objects are deleted, then "
        "change the mode. An incident ends when the node comes back after a reboot or when its Node "
        "object is deleted",
        False,
    )
