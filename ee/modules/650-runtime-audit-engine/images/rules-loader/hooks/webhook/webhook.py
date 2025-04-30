#!/usr/bin/env python3

# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

import subprocess
import typing
from os import remove

from deckhouse import hook
from yaml import dump

from lib.convert import convert_spec


def main(ctx: hook.Context):
    try:
        request = ctx.binding_context["review"]["request"]
        validate(request)
        ctx.output.validations.allow()
    except subprocess.CalledProcessError as e:
        print(e.output)
        ctx.output.validations.deny(f"spec validation error: {e.output}")
    except Exception as e:
        ctx.output.validations.error(str(e))


def validate(request: dict) -> typing.Union[str,  None]:
    uid = request.get("uid", "uid")
    spec = request.get("object", {}).get("spec", {})
    if request["operation"] == "CREATE":
        return validate_falco_rules(uid, spec)
    elif request["operation"] == "UPDATE":
        return validate_falco_rules(uid, spec)
    else:
        raise Exception(f"Unknown operation {request.operation}")


def validate_falco_rules(uid: str, spec: dict):
    rule_file_name = f'/tmp/falco-{uid}.rule'
    with open(rule_file_name, "w") as file:
        res = convert_spec(spec)
        file.write(dump(res))

    subprocess.check_output(
        f'falco -V {rule_file_name}',
        stderr=subprocess.STDOUT,
        shell=True,
        timeout=5,
        universal_newlines=True
    )

    remove(rule_file_name)


if __name__ == "__main__":
    hook.run(main, configpath="webhook/config.yaml")
