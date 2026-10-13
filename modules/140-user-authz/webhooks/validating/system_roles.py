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

# `heritage: deckhouse` marks a ClusterRole as installed by the platform, and that marker is
# load-bearing beyond bookkeeping: identity_assign.py reads a level out of a role's name and
# annotations only for roles carrying it (is_platform_owned), because a platform role name such as
# `d8:manage:<subsystem>:admin` is otherwise just a string anyone with `create` on clusterroles can
# pick. So the marker has to stay out of reach of everyone but the platform.
#
# templates/system-roles-validation-webhook.yaml embeds this file in a ValidationWebhook;
# webhook-operator renders it into a hook that calls validate().

# The operator template imports these as well; they make the file importable by the tests.
from typing import Optional
from dotmap import DotMap

DECKHOUSE_SA = "system:serviceaccount:d8-system:deckhouse"


def _text(value) -> str:
    """A value the way the old hook read it out of jq: null as "null", no trailing line breaks."""
    return ("null" if value is None else str(value)).rstrip("\n")


def _labels(obj) -> dict:
    return ((obj or {}).get("metadata") or {}).get("labels") or {}


def check(request: dict) -> Optional[str]:
    """Returns None to allow the request, or the message to deny it with."""
    if _text((request.get("userInfo") or {}).get("username")) == DECKHOUSE_SA:
        return None

    operation = _text(request.get("operation"))
    if operation == "UPDATE":
        # objectSelector matches when either the old or the new object carries the label, so updates
        # of a role that is already marked arrive here too -- including every rewrite of .rules that
        # clusterrole-aggregation-controller performs on the aggregated platform roles, and the
        # removal of the label. Only the transition into the label turns a self-made role into one
        # that claims to be platform-installed, so that is the only update worth refusing. Reading
        # the label off oldObject also fails closed: an unreadable previous value is not "deckhouse".
        if _text(_labels(request.get("oldObject")).get("heritage") or "") == "deckhouse":
            return None
    elif operation != "CREATE":
        return None

    name = _text(((request.get("object") or {}).get("metadata") or {}).get("name"))
    return f'ClusterRole "{name}" has forbidden label "heritage:deckhouse"'


def validate(ctx: DotMap) -> tuple[Optional[str], bool]:
    message = check(ctx.toDict()["review"]["request"])
    return message, message is None
