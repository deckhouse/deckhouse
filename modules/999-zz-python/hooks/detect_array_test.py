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
from shell_operator import hook

module_name = "zzPython"


def run_hook(config_values: dict = None, initial_values: dict = None):
    if initial_values is None:
        initial_values = config_values

    return hook.testrun(
        func=detect_array.main,
        module_name=module_name,  # we have to pass module name to make ctx shortcuts work
        config_values=config_values,
        initial_values=initial_values,
    )


def test_present_array_is_detected():
    config_values = {module_name: {"array": [1]}}
    out = run_hook(
        config_values=config_values,
        # initial_values=config_values,
    )

    assert out.values.zzPython.internal.statement == "THE ARRAY IS HERE"


def test_empty_array_is_not_detected():
    config_values = {module_name: {"array": []}}
    out = run_hook(
        config_values=config_values,
        # initial_values=config_values,
    )

    assert out.values.zzPython.internal.statement == "NO ARRAY IN CONFIG"


def test_absent_array_is_not_detected():
    config_values = {module_name: {}}
    out = run_hook(
        config_values=config_values,
        # initial_values=config_values,
    )

    assert out.values.zzPython.internal.statement == "NO ARRAY IN CONFIG"


def test_count_increses():
    out = run_hook(
        initial_values={module_name: {"internal": {"count": 0}}},
    )
    assert out.values.zzPython.internal.count == 1

    out = run_hook(
        initial_values={module_name: {"internal": {"count": 1}}},
    )
    assert out.values.zzPython.internal.count == 2
