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


from dataclasses import dataclass

from deckhouse import hook
from ensure_crds import CRDGetter, handler
from kubernetes import client


@dataclass
class CRDGetterMock(CRDGetter):
    crds: dict

    def get(self, name: str) -> dict:
        crd = self.crds.get(name, None)
        print("mock get", name, crd)
        if crd is None:
            raise client.rest.ApiException(status=404)
        return crd


def test_all_crds_are_new():
    out = hook.testrun(handler(CRDGetterMock({})))

    assert len(out.kube_operations.data) == 2, "Two CRDs should be created"

    names = set()
    for patch in out.kube_operations.data:
        assert patch["operation"] == "CreateOrUpdate"
        names.add(patch["object"]["metadata"]["name"])

    assert "pythons.deckhouse.io" in names, "Python CRD should be created"
    assert "nodejss.deckhouse.io" in names, "NodeJS CRD should be created"
