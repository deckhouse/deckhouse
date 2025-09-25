#!/usr/bin/python3

# Copyright 2024 Flant JSC
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

import typing
from dotmap import DotMap
from deckhouse import hook, utils

config = """
configVersion: v1
kubernetesCustomResourceConversion:
  - name: v1beta1_to_v1beta2_machinedeployment
    crdName: machinedeployments.cluster.x-k8s.io
    conversions:
      - fromVersion: cluster.x-k8s.io/v1beta1
        toVersion:   cluster.x-k8s.io/v1beta2
  - name: v1beta2_to_v1beta1_machinedeployment
    crdName: machinedeployments.cluster.x-k8s.io
    conversions:
      - fromVersion: cluster.x-k8s.io/v1beta2
        toVersion:   cluster.x-k8s.io/v1beta1
"""

class MachineDeploymentConversion(utils.BaseConversionHook):
    def __init__(self, ctx: hook.Context):
        super().__init__(ctx)

    @staticmethod
    def _move_key(obj: dict, src: str, dst: str):
        if src in obj and obj[src] is not None and obj[src] != "":
            obj[dst] = obj[src]
            del obj[src]

    @staticmethod
    def _group_from_apiversion(av: str) -> str:
        if not isinstance(av, str):
            return av
        parts = av.split("/", 1)
        return parts[0] if parts and parts[0] else av

    def _v1beta1_to_v1beta2_refs(self, dm: DotMap):
        cref = dm.get("spec.template.spec.bootstrap.configRef")
        if isinstance(cref, dict):
            if "apiVersion" in cref:
                cref["apiGroup"] = self._group_from_apiversion(cref["apiVersion"])
                del cref["apiVersion"]

        iref = dm.get("spec.template.spec.infrastructureRef")
        if isinstance(iref, dict):
            if "apiVersion" in iref:
                iref["apiGroup"] = self._group_from_apiversion(iref["apiVersion"])
                del iref["apiVersion"]

    def _v1beta2_to_v1beta1_refs(self, dm: DotMap):
        cref = dm.get("spec.template.spec.bootstrap.configRef")
        if isinstance(cref, dict):
            if "apiGroup" in cref and "apiVersion" not in cref:
                cref["apiVersion"] = cref["apiGroup"]
            if "apiGroup" in cref:
                del cref["apiGroup"]

        # infrastructureRef
        iref = dm.get("spec.template.spec.infrastructureRef")
        if isinstance(iref, dict):
            if "apiGroup" in iref and "apiVersion" not in iref:
                iref["apiVersion"] = iref["apiGroup"]
            if "apiGroup" in iref:
                del iref["apiGroup"]

    def v1beta1_to_v1beta2_machinedeployment(self, o: dict) -> typing.Tuple[str | None, dict]:
        dm = DotMap(o)
        dm.apiVersion = "cluster.x-k8s.io/v1beta2"
        self._v1beta1_to_v1beta2_refs(dm)
        return None, dm.toDict()

    def v1beta2_to_v1beta1_machinedeployment(self, o: dict) -> typing.Tuple[str | None, dict]:
        dm = DotMap(o)
        dm.apiVersion = "cluster.x-k8s.io/v1beta1"
        self._v1beta2_to_v1beta1_refs(dm)
        return None, dm.toDict()


def main(ctx: hook.Context):
    MachineDeploymentConversion(ctx).run()

if __name__ == "__main__":
    hook.run(main, config=config)