#!/usr/bin/env python3

# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

from sys import path
path.append('../rules')

from dotmap import DotMap
from deckhouse import hook
import hook as rules


def main(ctx: hook.Context):
    try:
        # DotMap is a dict with dot notation
        request = DotMap(ctx.binding_context).review.request

        # print("request", request.pprint(pformat="json"))  # debug printing

        errmsg = validate(request)
        if errmsg is None:
            ctx.output.validations.allow()
        else:
            ctx.output.validations.deny(errmsg)
    except Exception as e:
        # print("validating error", str(e))  # debug printing
        ctx.output.validations.error(str(e))


def validate(request: DotMap) -> str | None:
    match request.operation:
        case "CREATE":
            return validate_falco_rules(request.object)
        case "UPDATE":
            return validate_falco_rules(request.object)
        case _:
            raise Exception(f"Unknown operation {request.operation}")


def validate_falco_rules(obj: DotMap) -> str | None:
    # Validate name
    print("Validate_falco_rules func")
    res = rules.convert_spec(obj.spec)
    print(res)

    return None


if __name__ == "__main__":
    hook.run(main, configpath="webhook/config.yaml")
