#!/usr/bin/python3

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

import unittest

import reserved_policy_names as reserved


class TestCheckReservedName(unittest.TestCase):
    def test_ordinary_names_are_allowed(self):
        for name in ["foo", "d8-foo", "system-warn-foo", "foo-system-warn", "foo-system"]:
            with self.subTest(name=name):
                self.assertIsNone(reserved.check_reserved_name(name))

    def test_reserved_prefixes_are_denied(self):
        for name in ["d8-system-warn-foo", "d8-system-enforce-foo", "d8-pod-security-baseline-deny-default"]:
            with self.subTest(name=name):
                error = reserved.check_reserved_name(name)
                self.assertIsNotNone(error)
                self.assertIn(name, error)

    def test_no_prefix_contains_another(self):
        # The prefixes exist so that the constraints derived from any two policies stay
        # distinct, which holds only while no prefix is a prefix of another.
        for one in reserved.RESERVED_NAME_PREFIXES:
            for other in reserved.RESERVED_NAME_PREFIXES:
                if one is not other:
                    self.assertFalse(one.startswith(other), f"{one} starts with {other}")


if __name__ == "__main__":
    unittest.main()
