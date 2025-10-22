#!/usr/bin/env python3

# Copyright 2025 Flant JSC
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

import json
import os
import re
import subprocess
import sys

from obtain_edition_list import obtain_edition_list


def crane(command_list):
    completed_process = subprocess.run(["crane"] + command_list, text=True, capture_output=True)
    completed_process.check_returncode()
    return completed_process.stdout


def regctl(command_list):
    completed_process = subprocess.run(["regctl"] + command_list, text=True, capture_output=True)
    completed_process.check_returncode()
    return completed_process.stdout


registry = os.getenv("DECKHOUSE_REGISTRY")
max_versions_to_compare = int(os.getenv("VERSIONS_TO_COMPARE"))
editions = [i.lower() for i in obtain_edition_list(os.getenv("EDITIONS_FILE"))]
version_regex = re.compile(r"^v[0-9]+\.[0-9]+\.[0-9]+$")
edition_data = {}

# Gather image digests
for edition in editions:
    edition_data[edition] = {k: None for k in crane(["ls", f"{registry}/deckhouse/{edition}/modules"]).rstrip().split('\n')}
    for module in edition_data[edition]:
        version_list = [i for i in crane(["ls", f"{registry}/deckhouse/{edition}/modules/{module}"]).rstrip().split('\n') if version_regex.match(i)]
        version_list.sort(key=lambda s: list(map(int, s.removeprefix('v').split('.'))))
        edition_data[edition][module] = {k: None for k in version_list[-max_versions_to_compare:]}
        for version in edition_data[edition][module]:
            edition_data[edition][module][version] = crane(['digest', f'{registry}/deckhouse/{edition}/modules/{module}:{version}']).rstrip()

# Find unique digests
unique_images = {}
for edition, modules in edition_data.items():
    for module, versions in modules.items():
        if module not in unique_images:
            unique_images[module] = {}
        for version, digest in versions.items():
            if version not in unique_images[module]:
                unique_images[module][version] = set()
            unique_images[module][version].add(digest)

# Find which module versions have more than one unique digest
found = False
for module, versions in unique_images.items():
    for version, digests in versions.items():
        if len(digests) > 1:
            found = True
            print(f'Found differing image digests for module {module}:{version}:')
            for edition in editions:
                if module in edition_data[edition] and version in edition_data[edition][module]:
                    print(f'\t{edition} digest: {edition_data[edition][module][version]}')
            # Compare image_digests for differing modules
            module_image_digests = {}
            unique_module_images = {}
            module_diff_found = False
            for edition in editions:
                if module in edition_data[edition] and version in edition_data[edition][module]:
                    module_image_digests[edition] = json.loads(regctl([
                        "image",
                        "get-file",
                        f"{registry}/deckhouse/{edition}/modules/{module}:{version}",
                        "images_digests.json"
                    ]))
            for edition, module_digests in module_image_digests.items():
                for name, digest in module_digests.items():
                    if name not in unique_module_images:
                        unique_module_images[name] = set()
                    unique_module_images[name].add(digest)
            for name, module_digest_set in unique_module_images.items():
                if len(module_digest_set) > 1:
                    module_diff_found = True
                    print(f"\tFound differing module component {name}:")
                    for edition, module_digests in module_image_digests.items():
                        print(f"\t\t{edition} digest: {module_digests[name]}")
            if not module_diff_found:
                print(f"\tNo differing component images for module {module}:{version}")

if found:
    sys.exit(1)
