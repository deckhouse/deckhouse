#!/usr/bin/env python3

# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

from sys import path
path.append('/hooks/rules')
from rules import convert_spec

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
        # print("validating error", str(e))  # debug printing
        ctx.output.validations.error(str(e))


def validate(request: dict) -> str | None:
    match request["operation"]:
        case "CREATE":
            return validate_falco_rules(request["object"]["spec"])
        case "UPDATE":
            return validate_falco_rules(request["object"]["spec"])
        case _:
            raise Exception(f"Unknown operation {request['operation']}")


def validate_falco_rules(spec: dict) -> str | None:

    with open('/tmp/test.rule', "w") as file:
        res = convert_spec(spec)
        file.write(dump(res))

    try:
        output = subprocess.check_output(
        "cat /tmp/test.rule", stderr=subprocess.STDOUT, shell=True, timeout=3,
        universal_newlines=True)
    except subprocess.CalledProcessError as exc:
        print("Status : FAIL", exc.returncode, exc.output)
    else:
        print("Output: \n{}\n".format(output))


    remove('/tmp/test.rule')

    return None


if __name__ == "__main__":
    hook.run(main, configpath="webhook/config.yaml")
