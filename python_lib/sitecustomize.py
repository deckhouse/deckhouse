#!/usr/bin/env python3
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
    """Returns module root path and module name for hook being executed.

    E.g. for hook path /deckhouse/modules/999-zz-python/hooks/detect_array.py
    module root path is /deckhouse/modules/999-zz-python and module name is 999-zz-python

    Args:
        path (str): hook absolute path

    Returns:
        (str, str): module root and module dir name, last segment of the root
    """
    while True:
        parent, _ = os.path.split(path)
        # Don't stick to absolute path (/deckhouse/modules) to keep it portable
        if os.path.split(parent)[1] == "modules":
            return parent
        path = parent


hook_path = os.path.abspath(sys.argv[0])
mod_root = find_module_root(hook_path)

# Add Python packages discovery for module hooks
if mod_root:
    lib_path = os.path.join(mod_root, "lib", "python", "dist")
    sys.path.append(lib_path)

    os.environ["D8_MODULE_ROOT"] = mod_root
