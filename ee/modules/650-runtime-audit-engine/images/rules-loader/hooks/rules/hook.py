#!/usr/bin/env python3

# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

from shell_operator import hook
from yaml import dump


# Converts FalcoAuditRules CRD format to the native Falco rules
def convert_spec(spec: dict) -> list:
    result = []

    if spec["requiredEngineVersion"] is not None:
        result.append({
            "required_engine_version": spec["requiredEngineVersion"],
        })

    if spec["requiredK8sAuditPluginVersion"] is not None:
        result.append({
            "required_plugin_versions": [
                {
                    "name": "k8saudit",
                    "version": spec["requiredK8sAuditPluginVersion"],
                },
            ],
        })

    for item in spec["rules"]:
        if item["rule"] is not None:
            converted_item = {
                **item["rule"],
                "source": item["rule"]["source"].lower(),
            }
            converted_item["rule"] = converted_item.pop("name")

            result.append(converted_item)
            continue
        if item["macros"] is not None:
            result.append({
                "macro": item["macros"]["name"],
                "condition": item["macros"]["condition"],
            })
            continue
        if item["list"] is not None:
            result.append({
                "list": item["list"]["name"],
                "items": item["list"]["items"],
            })
            continue

    return result


def main(ctx: hook.Context):
    for s in ctx.snapshots["rules"]:
        filtered = s["filterResult"]
        if filtered is None:
            # Should not happen
            continue

        filename = f'{filtered["name"]}.yaml'

        with open(f'/etc/falco/rules.d/{filename}', "w") as file:
            spec = convert_spec(filtered["spec"])
            file.write(dump(spec))

    with open('/tmp/ready', "w") as file:
        file.write("ok")


if __name__ == "__main__":
    hook.run(main, configpath="rules/config.yaml")
