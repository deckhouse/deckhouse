#!/usr/bin/env python3

# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

from lib.convert import convert_spec
from deckhouse import hook
from yaml import dump
from os import remove
import subprocess


def main(ctx: hook.Context):
    try:
        request = ctx.binding_context["review"]["request"]
        errmsg = validate(request)
        if errmsg is None:
            ctx.output.validations.allow()
        else:
            ctx.output.validations.deny(errmsg)
    except Exception as e:
        ctx.output.validations.error(str(e))


def validate(request: dict) -> str | None:
    uid = request.get("uid", "uid")
    spec = request.get("object", {}).get("spec", {})
    match request["operation"]:
        case "CREATE":
            return validate_falco_rules(uid, spec)
        case "UPDATE":
            return validate_falco_rules(uid, spec)
        case _:
            raise Exception(f"Unknown operation {request.operation}")


def validate_falco_rules(uid: str, spec: dict) -> str | None:
    rule_file_name = f'/tmp/falco-{uid}.rule'
    with open(rule_file_name, "w") as file:
        res = convert_spec(spec)
        file.write(dump(res))
    try:
        output = subprocess.check_output(
            f'falco -V {rule_file_name}',
            stderr=subprocess.STDOUT,
            shell=True,
            timeout=5,
            universal_newlines=True
        )
    except subprocess.CalledProcessError as exc:
        remove(rule_file_name)
        return "Spec validation error"

    remove(rule_file_name)
    return None


if __name__ == "__main__":
    hook.run(main, configpath="webhook/config.yaml")
