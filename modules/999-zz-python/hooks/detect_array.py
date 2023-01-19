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

from deckhouse_sdk import hook

config = """
configVersion: v1
beforeHelm: 1
schedule:
- crontab: "* * * * *"
"""


def main(ctx: hook.Context):
    # ctx.values.zzPython.internal.count += 1
    # if ctx.values.zzPython.array:
    #     ctx.values.zzPython.internal.statement = "THE ARRAY IS HERE"
    # else:
    #     ctx.values.zzPython.internal.statement = "NO ARRAY IN CONFIG"

    # At runtime, module name is discovered automatically, so we can use ctx shortcuts.
    # In tests, module name must be passed explicitly.
    ctx.internal.count += 1
    if ctx.config.array:
        ctx.internal.statement = "THE ARRAY IS HERE"
    else:
        ctx.internal.statement = "NO ARRAY IN CONFIG"


if __name__ == "__main__":
    hook.run(main, config=config)
