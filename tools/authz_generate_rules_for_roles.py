#! /usr/bin/env python3
#
# Copyright 2023 Flant JSC
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
#
# This script is used for generating rules for user-authz roles to
# ./modules/140-user-authz/docs/README.md and ./modules/140-user-authz/docs/README_RU.md.
# NOTE: this is a very poorly written script.
# It inserts data between lines "<!-- start placeholder -->" and "<!-- end placeholder -->".
# It useses rendered template from /deckhouse/modules/140-user-authz/templates/cluster-roles.yaml
# Steps to use:
#   * export USER_AUTHZ_RENDER_ROLES=yes
#   * make tests-modules FOCUS=user-authz
#   * ./tools/authz_generate_rules_for_roles.py /tmp/rendered_templates.yaml
#   * make lint-markdown-fix
#   * check diff for ./modules/140-user-authz/docs/README.md and ./modules/140-user-authz/docs/README_RU.md files


from re import sub
from typing import Dict, Any, List, MutableSet, Tuple
import sys
import os
import yaml


def my_join(l):
    return f'`{"`, `".join(l)}`'


READ_VERBS = ["get", "list", "watch"]
READ_WRITE_VERBS = ["get", "list", "watch", "create", "delete", "deletecollection", "patch", "update"]
WRITE_VERBS = ["create", "delete", "deletecollection", "patch", "update"]

READ_VERBS_STR = my_join(["get", "list", "watch"])
READ_WRITE_VERBS_STR = my_join(["get", "list", "watch", "create", "delete", "deletecollection", "patch", "update"])
WRITE_VERBS_STR = my_join(["create", "delete", "deletecollection", "patch", "update"])


def process_rule(rule: Dict[str, List[str]]) -> Tuple[str, MutableSet[str]]:
    if not rule and not rule.get("verbs"):
        return None

    resources = set()
    for resource in rule.get("resources", []):
        for api_group in rule.get("apiGroups", []):
            resources.add(f"{api_group + '/' if api_group else ''}{resource}")

    verbs_set = set(rule.get("verbs", []))
    if verbs_set == set(READ_WRITE_VERBS):
        result_verb = "read-write"
    elif verbs_set == set(READ_VERBS):
        result_verb = "read"
    elif verbs_set == set(WRITE_VERBS):
        result_verb = "write"
    else:
        result_verb = ",".join(rule.get("verbs", []))
    return result_verb, resources


def process_rules(rules: Dict[str, Any]) -> Dict[str, Any]:
    result_rules = dict()
    for rule in rules:
        if not rule or not rule.get("apiGroups"):
            continue

        verb, resources = process_rule(rule)
        result_rules[verb] = result_rules.get(verb, set()) | resources
    return result_rules


def camel_case(s):
    s = sub(r"(_|-)+", " ", s).title().replace(" ", "")
    return ''.join([s])


def update_readme(readme_file: str, data: str):
    directory = os.path.dirname(os.path.realpath(__file__))
    directory = os.path.join(directory, "..", "modules/140-user-authz/docs")

    lines = []
    with open(os.path.join(directory, readme_file), "r", encoding="utf-8") as f:
        skip = 0
        for line in f.readlines():
            if "end placeholder" in line:
                skip = 0

            if skip:
                continue
            lines.append(line)
            if "start placeholder" in line:
                lines.append(data)
                skip = 1

    with open(os.path.join(directory, readme_file), "w", encoding="utf-8") as f:
        f.write("".join(lines))


def main():
    module_prefix = "user-authz:"
    postfixes = ("user", "privileged-user", "editor", "admin", "cluster-editor", "cluster-admin")
    names = {module_prefix+postfix: camel_case(postfix) for postfix in postfixes}

    with open(sys.argv[1], "r", encoding="utf-8") as f:
        manifests_iterator = yaml.safe_load_all(f)
        manifests = []
        for manifest in manifests_iterator:
            if not manifest or manifest.get("kind") != "ClusterRole":
                continue
            manifests.append(manifest)

    all_rules = {}
    for manifest in manifests:
        name = manifest.get("metadata", {}).get("name")
        if name in names:
            all_rules[name] = process_rules(manifest.get("rules", []))

    full_rbac_names = list(names)
    excludes = {full_rbac_names[i]: full_rbac_names[:i] for i in range(len(full_rbac_names))}
    excludes[module_prefix+"cluster-editor"] = excludes[module_prefix+"cluster-editor"][:-1]

    for name, values in all_rules.items():
        for excl in excludes[name]:
            for verb in values:
                all_rules[name][verb] = values[verb] - all_rules[excl].get(verb, set())

    result = {}
    for key, verbs in all_rules.items():
        for verb, values in verbs.items():
            if len(values) < 1:
                continue
            if not result.get(key):
                result[key] = {}
            result[key][verb] = sorted(list(values))

    result_string = f'''
* read - {READ_VERBS_STR}
* read-write - {READ_WRITE_VERBS_STR}
* write - {WRITE_VERBS_STR}

```yaml
'''
    for name, values in result.items():
        postfix_annotation = ""
        if len(excludes[name]) > 0:
            postfix_annotation = f" (and all rules from `{'`, `'.join([names[excl] for excl in excludes[name]])}`)"

        result_string += yaml.safe_dump(
            {f"Role `{names[name]}`{postfix_annotation}": values}) + "\n"
    result_string += "```\n"
    update_readme("README.md", "`verbs` aliases:"+result_string)
    update_readme("README_RU.md", "сокращения для `verbs`:"+result_string)


if __name__ == "__main__":
    main()
