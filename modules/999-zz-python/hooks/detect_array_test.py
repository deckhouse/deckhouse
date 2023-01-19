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
from deckhouse_sdk import hook
from dotmap import DotMap

module_name = "zzPython"

# Remember! This is a test, not a hook. It is not executed by Deckhouse. So this file must be not
# executable.


def run_hook(initial_values: dict = None):
    """
    Helper to run hook.main() with initial values.

    Values are config_values with .internal field added and controlled by hooks, so we mimic this
    behaviour in this helper. Since we use ctx.values, not config values directly, we don't need
    to fill config_values if they are empty. Config values are filled by module settings in module
    config and by defaults in openapi/config-values.yaml.
    """
    return hook.testrun(
        func=detect_array.main,
        initial_values=initial_values,
    )


def test_present_array_is_detected():
    values = DotMap()
    values.zzPython.array = [1]

    out = run_hook(initial_values=values)

    assert out.values.zzPython.internal.statement == "THE ARRAY IS HERE"


def test_empty_array_is_not_detected():
    values = DotMap()
    values.zzPython.array = []

    out = run_hook(initial_values=values)

    assert out.values.zzPython.internal.statement == "NO ARRAY IN CONFIG"


def test_absent_array_is_not_detected():
    values = DotMap({"zzPython": {}})

    out = run_hook(initial_values=values)

    assert out.values.zzPython.internal.statement == "NO ARRAY IN CONFIG"


def test_count_increses():
    values = DotMap()
    values.zzPython.internal.count = 12

    out = run_hook(initial_values=values)

    assert out.values.zzPython.internal.count == 13
