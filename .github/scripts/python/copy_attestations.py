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
import subprocess
import os

def oras(command_list):
    completed_process = subprocess.run(["oras"] + command_list, text=True, capture_output=True)
    completed_process.check_returncode()
    return completed_process.stdout

images_tags_path = os.getenv("IMAGES_TAGS_PATH")

with open(images_tags_path) as f:
  images = json.load(f)['Images']

registry_from = os.getenv("REGISTRY_FROM")
registry_to = os.getenv("REGISTRY_TO")

for k in images.keys():
  if k.endswith("-vex-artifact"):
    copied_image = k.removesuffix("-vex-artifact")
    sha256 = images[copied_image]['DockerImageDigest'].removeprefix('sha256:')
    from_image = f'{registry_from}:sha256-{sha256}.att'
    to_image = f'{registry_to}:sha256-{sha256}.att'
    print(f'Copying {copied_image}: {from_image} => {to_image}')
    print(oras(['cp', from_image, to_image]))
