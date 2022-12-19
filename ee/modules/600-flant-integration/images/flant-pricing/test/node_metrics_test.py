#!/usr/bin/env python3
#!/usr/bin/env micropython
#
# Copyright 2022 Flant JSC Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
#
import os
import sys

sys.path.insert(0, "../hooks")
from node_metrics import run

run()


with open(os.getenv("METRICS_PATH"), "r", encoding="utf-8") as f:
    result = f.read()

with open(os.getenv("METRICS_PATH_EXPECTED"), "r", encoding="utf-8") as f:
    expected = f.read()

assert result == expected, f"Expected: {expected}, got: {result}"
