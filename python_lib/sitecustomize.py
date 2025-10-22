#!/usr/bin/python3
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
import os
import sys


def find_module_root(path):
    """Returns module root path in Deckhouse pod or Webhook Handler pod.

    E.g. for hook path /deckhouse/modules/987-module-name/hooks/detect_array.py
    For Deckhouse, module root is
        $ANY_PARENT/modules/987-module-name
    For Webhook Handler, the root is
        $ANY_PARENT/987-module-name/webhooks

    Args:
        path (str): hook absolute path

    Returns:
        (str): hook 'module' root
    """
    while True:
        # We are in the module root if we found Chart.yaml file.
        if os.path.exists(os.path.join(path, "module.yaml")) or os.path.exists(os.path.join(path, "Chart.yaml")):
            return path

        parent, _ = os.path.split(path)
        # Discover module root for deckhouse, or module webhooks root for webhook handler.
        if os.path.split(parent)[1] == "modules" or path == "/":
            # If we are in the FS root, it's likely we are not in Deckhouse pod or webhook handler
            # pod, so we just break the loop.
            return path
        if os.path.split(parent)[1] == "webhooks":
            return parent
        path = parent


hook_path = os.path.abspath(sys.argv[0])
mod_root = find_module_root(hook_path)

# Add Python packages discovery for module hooks
if mod_root:
    lib_path = os.path.join(mod_root, "lib", "python", "dist")
    sys.path.append(lib_path)
