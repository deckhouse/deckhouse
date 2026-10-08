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

# Assembles the hook of a ValidationWebhook, so a unit test can run it with hook.testrun.
#
# The ValidationWebhooks of the module take handler.python from the files next to this one, and
# webhook-operator renders it into the template
# modules/002-deckhouse/images/webhook-handler/operator/internal/controller/templates/validationwebhook.tpl.
# load() runs a copy of the template's `main` and then the same files, in the order the
# ValidationWebhook joins them, in one module.
#
# The template tests of the module check that the ValidationWebhooks render these files, that
# WRAPPER below matches the template and that this file is identical in every module that has such
# hooks, so a test cannot exercise code that differs from the cluster. The name keeps the file out of
# the webhook-handler image.

import copy
import sys
import types
from pathlib import Path
from typing import Optional

from deckhouse import hook
from dotmap import DotMap

# Verbatim copy of the template's wrapper: the part between `def main` and the point where the
# template injects handler.python.
WRAPPER = '''def main(ctx: hook.Context):
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


def load(*files: str) -> types.ModuleType:
    """Returns the hook made of `files`, names relative to this directory, as a module."""
    here = Path(__file__).resolve().parent
    name = "_".join(Path(file).stem for file in files) + "_hook"

    module = types.ModuleType(name)
    # The names the template has in scope for handler.python.
    module.__dict__.update({"Optional": Optional, "hook": hook, "DotMap": DotMap})
    sys.modules[name] = module

    # The template defines `main` first and pastes handler.python after it, so a handler could
    # override it; keep the same order.
    exec(compile(WRAPPER, "validationwebhook.tpl", "exec"), module.__dict__)
    for file in files:
        path = here / file
        exec(compile(path.read_text(), str(path), "exec"), module.__dict__)

    template_main = module.main

    def main(ctx: hook.Context):
        # The template sets keepFullObjectsInMemory: false on every context binding, so the hook
        # gets the jqFilter result only. A snapshot item a test hands over with the full object must
        # not let a handler that reads `object` pass here and fail in the cluster.
        ctx.binding_context = _without_full_objects(ctx.binding_context)
        return template_main(ctx)

    module.main = main
    return module


def _without_full_objects(binding_context):
    plain = copy.deepcopy(binding_context.toDict() if isinstance(binding_context, DotMap) else binding_context)
    for snapshot in (plain.get("snapshots") or {}).values():
        for item in snapshot or []:
            if isinstance(item, dict):
                item.pop("object", None)
    return plain
