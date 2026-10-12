# Copyright 2026 Flant JSC
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

"""shell-operator logs every plain-text line of a hook at info, stderr included; a JSON line carries
its own level. warn_stderr() makes what the hook writes to stderr, a traceback included, go out as
warnings."""

import json
import sys


class WarnLines:
    """A text stream that writes every line it gets as a JSON line with level warn."""

    def __init__(self, stream):
        self._stream = stream
        self._pending = ""

    def write(self, text):
        self._pending += text
        while "\n" in self._pending:
            line, self._pending = self._pending.split("\n", 1)
            self._emit(line)
        return len(text)

    def flush(self):
        if self._pending:
            self._emit(self._pending)
            self._pending = ""
        self._stream.flush()

    def _emit(self, line):
        if line.strip():
            self._stream.write(json.dumps({"level": "warn", "msg": line}) + "\n")

    def __getattr__(self, name):
        return getattr(self._stream, name)


def warn_stderr():
    sys.stderr = WarnLines(sys.stderr)
