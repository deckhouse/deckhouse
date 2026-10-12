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

import io
import json
import unittest

from stderr_level import WarnLines


class TestWarnLines(unittest.TestCase):
    def lines(self, *chunks):
        out = io.StringIO()
        stream = WarnLines(out)
        for chunk in chunks:
            stream.write(chunk)
        stream.flush()
        return [json.loads(line) for line in out.getvalue().splitlines()]

    def test_every_line_is_a_warning(self):
        self.assertEqual(self.lines('Traceback (most recent call last):\n  File "x", line 1\nValueError: "bad"\n'),
                         [{"level": "warn", "msg": 'Traceback (most recent call last):'},
                          {"level": "warn", "msg": '  File "x", line 1'},
                          {"level": "warn", "msg": 'ValueError: "bad"'}])

    def test_a_line_split_across_writes_is_one_line(self):
        self.assertEqual(self.lines("par", "tial\n"), [{"level": "warn", "msg": "partial"}])

    def test_flush_writes_the_unfinished_line(self):
        self.assertEqual(self.lines("no newline"), [{"level": "warn", "msg": "no newline"}])

    def test_blank_lines_are_dropped(self):
        self.assertEqual(self.lines("\n  \nx\n"), [{"level": "warn", "msg": "x"}])


if __name__ == "__main__":
    unittest.main()
