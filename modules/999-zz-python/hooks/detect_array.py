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

import sys

from deckhouse_sdk import hook
from dotmap import DotMap

config = """
configVersion: v1
beforeHelm: 1
schedule:
- crontab: "* * * * *"
"""


def main(ctx: hook.Context):
    print("sys.path", sys.path)

    values = DotMap(ctx.values)  # deep copy
    print("VALUES BEFORE", values.pprint(pformat="json"))

    values.zzPython.internal.count += 1
    if values.zzPython.array:
        values.zzPython.internal.statement = "THE ARRAY IS HERE"
    else:
        values.zzPython.internal.statement = "NO ARRAY IN CONFIG"

    print("VALUES AFTER", values.pprint(pformat="json"))
    ctx.values = values.toDict()  # make values JSON serializable


if __name__ == "__main__":
    hook.run(main, config=config)
