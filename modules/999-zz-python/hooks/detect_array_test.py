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


import detect_array
from deckhouse_sdk import hook, module

module_name = "zzPython"


def run_hook(config_values: dict = None, initial_values: dict = None):
    if initial_values is None:
        initial_values = config_values

    # We have to pass module name to make ctx shortcuts work in the hook under test. We can do
    # it either here, or via env variable D8_MODULE_ROOT, see Makefile.
    #
    # module_name=module_name,
    return hook.testrun(
        func=detect_array.main,
        config_values=config_values,
        initial_values=initial_values,
    )


def config_values(d: dict):
    # Discovers module name from D8_MODULE_ROOT env variable.
    mod_name = module.get_name()
    return {mod_name: d or {}}


def internal_values(d: dict):
    # Discovers module name from D8_MODULE_ROOT env variable.
    mod_name = module.get_name()
    return {mod_name: {"internal": d or {}}}


def test_present_array_is_detected():
    out = run_hook(config_values=config_values({"array": [1]}))

    assert out.values.zzPython.internal.statement == "THE ARRAY IS HERE"


def test_empty_array_is_not_detected():
    out = run_hook(config_values=config_values({"array": []}))

    assert out.values.zzPython.internal.statement == "NO ARRAY IN CONFIG"


def test_absent_array_is_not_detected():
    out = run_hook(config_values=config_values({}))

    assert out.values.zzPython.internal.statement == "NO ARRAY IN CONFIG"


def test_count_increses():
    out = run_hook(initial_values=internal_values({"count": 12}))

    assert out.values.zzPython.internal.count == 13
