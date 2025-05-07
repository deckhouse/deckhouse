#!/usr/bin/env python3

# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

from os import remove, walk

from yaml import dump
from deckhouse import hook

from lib.convert import convert_spec

_FALCO_RULES_DIR = '/etc/falco/rules.d'


def main(ctx: hook.Context):
    filenames = set()

    # Create and update files with rules
    for s in ctx.snapshots["rules"]:
        filtered = s["filterResult"]
        if filtered is None:
            # Should not happen
            continue

        filename = f'{filtered["name"]}.yaml'
        filenames.add(filename)

        with open(f'{_FALCO_RULES_DIR}/{filename}', "w") as file:
            spec = convert_spec(filtered["spec"])
            file.write(dump(spec))

    # Delete missing rules
    for (_, _, existing_rules) in walk(_FALCO_RULES_DIR):
        for rule_name in existing_rules:
            if rule_name not in filenames:
                remove(f'{_FALCO_RULES_DIR}/{rule_name}')
        break  # depth 1

    with open('/tmp/ready', "w") as file:
        file.write("ok")


if __name__ == "__main__":
    hook.run(main, configpath="rules/config.yaml")
