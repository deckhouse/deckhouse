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

# Checks the subjects of a ClusterAuthorizationRule. It replaces a Bash hook and keeps its
# decisions and messages, including two of its quirks, marked below. The API server validates the
# object against the CRD schema first, so every subject has a kind and a name, both strings.
#
# templates/cluster-authorization-rule-validation-webhook.yaml embeds this file in a
# ValidationWebhook; webhook-operator renders it into a hook that calls validate().

import json
import re

# The operator template imports these as well; they make the file importable by the tests.
from typing import Optional
from dotmap import DotMap

# A DNS subdomain. Like in jq, $ also matches before a trailing line break.
SERVICE_ACCOUNT_NAME = re.compile(r"^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$")


def looks_like_glob(word: str) -> bool:
    """Tells whether Bash takes a word for a file name pattern.

    Quirk: the Bash hook iterated over the subjects as unquoted words, one JSON document each, under
    failglob, so a subject whose JSON is a pattern, such as a name with "*", "?" or "[...]", matched
    no file and failed the hook. It still does: check() raises SystemExit, which the `except
    Exception` of the template's main lets through, so the hook exits with 1 as before instead of
    answering with an error.
    """
    bracket = False
    chars = iter(word)
    for char in chars:
        if char in "*?":
            return True
        if char == "[":
            bracket = True
        elif char == "]" and bracket:
            return True
        elif char == "/":
            bracket = False
        elif char == "\\":
            next(chars, None)
    return False


def check(request: dict) -> Optional[str]:
    """Returns None to allow the request, or the message to deny it with."""
    subjects = ((request.get("object") or {}).get("spec") or {}).get("subjects") or []

    for subject in subjects:
        if looks_like_glob(json.dumps(subject, separators=(",", ":"), ensure_ascii=False)):
            raise SystemExit(f"no match: {json.dumps(subject)}")

    message = "error:"
    for subject in subjects:
        kind, namespace = subject.get("kind"), subject.get("namespace")
        # The old hook read names off the output of jq, which drops trailing line breaks.
        name = subject.get("name", "").rstrip("\n")

        if kind == "ServiceAccount":
            if not SERVICE_ACCOUNT_NAME.search(subject.get("name", "")) and name != "false":
                message += f" ServiceAccount name is invalid - {name} ;"
            # Quirk: a ServiceAccount named "false" read as "no failure" to the old hook.
            if namespace is None and name != "false":
                message += f" Namespace for the ServiceAccount {name} is not provided ;"
        elif namespace is not None:
            message += f" Namespace for the {kind} {name} must not be provided ;"

    if not subjects:
        message += " Field <.spec.subjects> must not be empty"

    return None if message == "error:" else message


def validate(ctx: DotMap) -> tuple[Optional[str], bool]:
    message = check(ctx.toDict()["review"]["request"])
    return message, message is None
