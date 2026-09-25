#!/usr/bin/python3

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


# Loads the hook of a ValidationWebhook manifest so it can be unit tested the same way a hook file is.
#
# A hook described by a ValidationWebhook has no file in the repository: webhook-operator renders
# `handler.python` into the template at
# modules/002-deckhouse/images/webhook-handler/operator/internal/controller/templates/validationwebhook.tpl
# and writes the result inside the container. `load` performs the same assembly in-process and returns a
# module exposing `main`, `validate` and `config`, so a test keeps using hook.testrun unchanged.
#
# The wrapper below is a copy of the template's own wrapper, which means it can drift from it. Call
# assert_operator_template_unchanged() from a test to fail loudly when that happens: a test must exercise
# the code that actually runs in the cluster, not a copy that silently diverged.

import os
import re
import types
from pathlib import Path
from typing import Optional

import yaml
from deckhouse import hook
from dotmap import DotMap

OPERATOR_TEMPLATE = Path(
    "modules/002-deckhouse/images/webhook-handler/operator/internal/controller/templates/validationwebhook.tpl"
)

# Verbatim copy of the wrapper from OPERATOR_TEMPLATE, the part between `def main` and the point where the
# template injects `handler.python`. Kept byte for byte so the comparison in
# assert_operator_template_unchanged() is exact.
_WRAPPER = '''def main(ctx: hook.Context):
    try:
        # DotMap is a dict with dot notation
        binding_context = DotMap(ctx.binding_context)
        message, allowed = validate(binding_context)
        if allowed:
            if message:
                ctx.output.validations.allow(message)  # warning
            else:
                ctx.output.validations.allow()
        else:
            ctx.output.validations.deny(message)
    except Exception as e:
        ctx.output.validations.error(str(e))
'''

# Defaults the template hardcodes on every context binding.
_CONTEXT_DEFAULTS = {
    "group": "main",
    "executeHookOnEvent": [],
    "executeHookOnSynchronization": False,
    "keepFullObjectsInMemory": False,
}

_HELM_ACTION = re.compile(r"{{")


def repo_root() -> Path:
    """Locates the repository root.

    Honours DECKHOUSE_ROOT, then the /src mount used by testing/webhooks/run.sh, then walks up from the
    working directory for the benefit of runs straight from a checkout.
    """
    candidates = []

    env_root = os.environ.get("DECKHOUSE_ROOT")
    if env_root:
        candidates.append(Path(env_root))

    candidates.append(Path("/src"))

    cwd = Path.cwd().resolve()
    candidates.extend([cwd, *cwd.parents])

    for candidate in candidates:
        if (candidate / OPERATOR_TEMPLATE).is_file():
            return candidate

    raise RuntimeError(
        "cannot locate the repository root: none of the candidates contains "
        f"{OPERATOR_TEMPLATE}. Set DECKHOUSE_ROOT to the checkout path."
    )


def _strip_helm(manifest_text: str, manifest_path: Path) -> str:
    """Drops whole-line Helm actions so the manifest can be parsed as plain YAML.

    Only lines that consist of a Helm action are droppable — a `helm_lib_module_labels` include and the
    like. An action anywhere else could change the very value under test, so it is an error instead.
    """
    kept = []
    for number, line in enumerate(manifest_text.splitlines(), start=1):
        stripped = line.strip()
        if stripped.startswith("{{"):
            continue
        if _HELM_ACTION.search(line):
            raise ValueError(
                f"{manifest_path}:{number}: inline Helm action in a ValidationWebhook is not supported by "
                f"the test harness, because the rendered value cannot be reproduced here: {line!r}"
            )
        kept.append(line)

    return "\n".join(kept)


def _render_config(manifest: dict) -> str:
    """Builds the `config` string the way OPERATOR_TEMPLATE does."""
    document = {"configVersion": "v1"}

    validation_object = manifest.get("validationObject")
    contexts = manifest.get("context") or []

    if validation_object:
        validation_object = dict(validation_object)
        if contexts:
            # The operator fills includeSnapshotsFrom in from the context binding names.
            validation_object["includeSnapshotsFrom"] = [binding["name"] for binding in contexts]
        document["kubernetesValidating"] = [validation_object]

    if contexts:
        document["kubernetes"] = [
            {"name": binding["name"], **_CONTEXT_DEFAULTS, **(binding.get("kubernetes") or {})}
            for binding in contexts
        ]

    return "\n" + yaml.safe_dump(document, sort_keys=False, default_flow_style=False)


def load(manifest: str):
    """Returns the hook of a ValidationWebhook manifest as a module.

    `manifest` is a path relative to the repository root, for example
    "modules/140-user-authz/templates/role-binding-validation-webhook.yaml".
    """
    path = repo_root() / manifest
    parsed = yaml.safe_load(_strip_helm(path.read_text(), path))

    if parsed.get("kind") != "ValidationWebhook":
        raise ValueError(f"{path}: expected a ValidationWebhook manifest, got kind={parsed.get('kind')!r}")

    handler = (parsed.get("handler") or {}).get("python")
    if not handler:
        raise ValueError(f"{path}: handler.python is empty")

    module = types.ModuleType(path.stem.replace("-", "_"))
    module.__dict__.update(
        {
            # The names the template has in scope for handler.python.
            "Optional": Optional,
            "hook": hook,
            "DotMap": DotMap,
            "config": _render_config(parsed),
        }
    )

    exec(compile(handler, str(path), "exec"), module.__dict__)
    exec(compile(_WRAPPER, str(OPERATOR_TEMPLATE), "exec"), module.__dict__)

    return module


def assert_operator_template_unchanged(test_case) -> None:
    """Fails when the wrapper copied above no longer matches the operator's template."""
    template = (repo_root() / OPERATOR_TEMPLATE).read_text()

    start = template.find("def main(ctx: hook.Context):")
    end = template.find("{{ .Handler.Python }}")
    test_case.assertNotEqual(start, -1, f"{OPERATOR_TEMPLATE}: no `def main` found")
    test_case.assertNotEqual(end, -1, f"{OPERATOR_TEMPLATE}: no handler injection point found")

    test_case.assertEqual(
        template[start:end].strip(),
        _WRAPPER.strip(),
        f"the wrapper in {OPERATOR_TEMPLATE} changed; update _WRAPPER in testing/webhooks/validation_webhook.py "
        "so tests keep exercising the code that runs in the cluster",
    )
