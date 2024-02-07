# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

from stringcase import snakecase

# Converts FalcoAuditRules CRD format to the native Falco rules


def convert_spec(spec: dict) -> list:
    result = []

    required_engine_version = spec.get("requiredEngineVersion")
    if required_engine_version is not None:
        result.append({
            "required_engine_version": required_engine_version,
        })

    required_k8saudit_plugin_version = spec.get(
        "requiredK8sAuditPluginVersion")
    if required_k8saudit_plugin_version is not None:
        result.append({
            "required_plugin_versions": [
                {
                    "name": "k8saudit",
                    "version": required_k8saudit_plugin_version,
                },
            ],
        })

    for item in spec["rules"]:
        # `item.get('key')` is not None is used instead of `'key' in item` to avoid exceptions if the value equals null.
        # According to FalcoAuditRules CRD value cannot be null, yet it is not bulletproof from all perspectives.\
        if item.get("rule") is not None:
            converted_item = {**item["rule"]}
            converted_item["rule"] = converted_item.pop("name")

            source = item["rule"].get("source")
            if source is not None:
                converted_item["source"] = snakecase(source)

            result.append(converted_item)
            continue
        if item.get("macro") is not None:
            result.append({
                "macro": item["macro"]["name"],
                "condition": item["macro"]["condition"],
            })
            continue
        if item.get("list") is not None:
            result.append({
                "list": item["list"]["name"],
                "items": item["list"]["items"],
            })
            continue

    return result
