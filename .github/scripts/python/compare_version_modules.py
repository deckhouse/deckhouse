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
import subprocess


def regctl(command_list):
    completed_process = subprocess.run(["regctl"] + command_list, text=True, capture_output=True)
    print(completed_process.stderr)
    completed_process.check_returncode()
    return completed_process.stdout


dangerous_modules = ["controlPlaneManager", "ingressNginx.controller"]

image_from = os.getenv("IMAGE_FROM")
image_to = os.getenv("IMAGE_TO")
max_attempts = int(os.getenv("COMPARE_MAX_ATTEMPTS", 1))
retry_delay = int(os.getenv("COMPARE_RETRY_DELAY", 30))

print(f"Lookup image_digests from {image_from}")

for attempts in range(1, max_attempts+1):
    try:
        digests_from = json.loads(regctl([
            "image",
            "get-file",
            image_from,
            "/deckhouse/modules/images_digests.json"
        ]))
    except subprocess.CalledProcessError as e:
        if attempts == max_attempts:
            raise
    else:
        break


print(f"Lookup image_digests from {image_to}")

for attempts in range(1, max_attempts+1):
    try:
        digests_to = json.loads(regctl([
            "image",
            "get-file",
            image_to,
            "/deckhouse/modules/images_digests.json"
        ]))
    except subprocess.CalledProcessError as e:
        if attempts == max_attempts:
            raise
    else:
        break

digests_unique = {}

for module, images in digests_from.items():
    if module not in digests_unique:
        digests_unique[module] = {}
    for image, digest in images.items():
        if image not in digests_unique[module]:
            digests_unique[module][image] = set()
        digests_unique[module][image].add(digest)

for module, images in digests_to.items():
    if module not in digests_unique:
        digests_unique[module] = {}
    for image, digest in images.items():
        if image not in digests_unique[module]:
            digests_unique[module][image] = set()
        digests_unique[module][image].add(digest)

results = []

for module, images in digests_unique.items():
    for image, digests_set in images.items():
        if len(digests_set) > 1:
            results.append(f"{module}.{image}")

results.sort()

dangerous_results = [i for i in results if any(m in i for m in dangerous_modules)]

if len(results) > 0:
    print("Found changes in following module images:")
    print(*results, sep="\n")
else:
    print("No changed module images found.")

if len(dangerous_results) > 0:
    print("Found possibly dangerous changes in following module images:")
    print(*dangerous_results, sep="\n")
    exit(1)
