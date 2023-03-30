#!/usr/bin/env python3

# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

import unittest

from hook import convert_spec


class TestConvertSpec(unittest.TestCase):
    def test_success_rule(self):
        res = convert_spec({
            "rules": [
                {
                    "rule": {
                        "name": "Detect Write Below /etc/hosts",
                        "desc": "an attempt to write to /etc/hosts file (CVE-2020-8557)",
                        "condition": "open_write and container and fd.name=/etc/hosts",
                        "output": "File /etc/hosts opened for writing",
                        "priority": "Error",
                        "tags": ["filesystem", "mitre_persistence"],
                        "source": "Syscall",
                    }
                }
            ]
        })

        expect = [
            {
                "rule": "Detect Write Below /etc/hosts",
                "desc": "an attempt to write to /etc/hosts file (CVE-2020-8557)",
                "condition": "open_write and container and fd.name=/etc/hosts",
                "output": "File /etc/hosts opened for writing",
                "priority": "Error",
                "tags": ["filesystem", "mitre_persistence"],
                "source": "syscall",
            }
        ]
        self.assertListEqual(res, expect)

    def test_success_with_macro(self):
        res = convert_spec({
            "rules": [
                {
                  "macro": {
                      "name": "Never True",
                      "condition": "(evt.num=0)",
                  }
                },
                {
                    "rule": {
                        "name": "Detect Write Below /etc/hosts",
                        "desc": "an attempt to write to /etc/hosts file (CVE-2020-8557)",
                        "condition": "open_write and container and fd.name=/etc/hosts",
                        "output": "File /etc/hosts opened for writing",
                        "priority": "Error",
                        "tags": ["filesystem", "mitre_persistence"],
                        "source": "Syscall",
                    }
                },
                {
                    "rule": {
                        "name": "Test name",
                        "desc": "Alert about test",
                        "condition": "ka.user.name == \"test\"",
                        "output": "Test output %ka.user.name",
                        "priority": "Error",
                        "source": "K8sAudit",
                    }
                }
            ]
        })

        expect = [
            {
                "condition": "(evt.num=0)",
                "macro": "Never True"
            },
            {
                "rule": "Detect Write Below /etc/hosts",
                "desc": "an attempt to write to /etc/hosts file (CVE-2020-8557)",
                "condition": "open_write and container and fd.name=/etc/hosts",
                "output": "File /etc/hosts opened for writing",
                "priority": "Error",
                "tags": ["filesystem", "mitre_persistence"],
                "source": "syscall",
            },
            {
                "desc": "Alert about test",
                "condition": 'ka.user.name == "test"',
                "output": "Test output %ka.user.name",
                "priority": "Error",
                "source": "k8s_audit",
                "rule": "Test name",
            }
        ]
        self.assertListEqual(res, expect)

    def test_success_with_required_engine(self):
        res = convert_spec({
            "requiredEngineVersion": 1,
            "rules": [
                {
                    "macro": {
                        "name": "Never True",
                        "condition": "(evt.num=0)",
                    }
                },
            ]
        })

        expect = [
            {
                "required_engine_version": 1,
            },
            {
                "condition": "(evt.num=0)",
                "macro": "Never True"
            },
        ]
        self.assertListEqual(res, expect)


if __name__ == '__main__':
    unittest.main()
