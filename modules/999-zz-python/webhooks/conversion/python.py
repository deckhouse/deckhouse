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

import json
from copy import deepcopy
from dataclasses import dataclass

from deckhouse_sdk import hook

config = """
configVersion: v1
kubernetesCustomResourceConversion:
- name: python_conversions
  crdName: pythons.deckhouse.io
  conversions:
  - fromVersion: deckhouse.io/v1alpha1
    toVersion: deckhouse.io/v1beta1
  - fromVersion: deckhouse.io/v1beta1
    toVersion: deckhouse.io/v1
  #  EXPLICIT BACKWARDS CONVERSIONS
  - fromVersion: deckhouse.io/v1beta1
    toVersion: deckhouse.io/v1alpha1
  - fromVersion: deckhouse.io/v1
    toVersion: deckhouse.io/v1beta1
"""


def main(ctx: hook.Context):
    conv = ConverterDispatcher(
        ConverterAdapter(
            from_version="deckhouse.io/v1alpha1",
            to_version="deckhouse.io/v1beta1",
            converter=Converter_from_v1alpha1_to_v1beta1(),
        ),
        ConverterAdapter(
            from_version="deckhouse.io/v1beta1",
            to_version="deckhouse.io/v1",
            converter=Converter_from_v1beta1_to_v1(),
        ),
    )
    try:
        bctx = ctx.binding_context

        print("BINDING CONTEXT")
        print(json.dumps(bctx, indent=2))  # debug printing

        v_from, v_to = bctx["fromVersion"], bctx["toVersion"]
        objects = bctx["review"]["request"]["objects"]
        if not objects:
            return

        for obj in objects:
            converted = conv.convert(v_from, v_to, obj)

            # print(json.dumps(converted, indent=2))  # debug printing

            ctx.output.conversions.collect(converted)
    except Exception as e:
        print("conversion error", str(e))  # debug printing
        ctx.output.conversions.error(str(e))

    print("CONVERSION RESPONSE")
    print(json.dumps(ctx.output.conversions.data[0], indent=2))  # debug printing


class Converter:
    def forward(self, obj: dict) -> dict:
        raise NotImplementedError()

    def backward(self, obj: dict) -> dict:
        raise NotImplementedError()


@dataclass
class ConverterAdapter(Converter):
    """Handy convert wrapper to wrap basic operations: deepcopy and changing apiVersion"""

    from_version: str
    to_version: str
    converter: Converter

    def forward(self, obj: dict) -> dict:
        obj = deepcopy(obj)
        obj["apiVersion"] = self.to_version
        return self.converter.forward(obj)

    def backward(self, obj: dict) -> dict:
        obj = deepcopy(obj)
        obj["apiVersion"] = self.from_version
        return self.converter.backward(obj)


class ConverterDispatcher:
    def __init__(self, *adapters: ConverterAdapter):
        self._converters = {self._key_a(a): a for a in adapters}

    def _keys(self, v_from, v_to):
        return f"{v_from}:{v_to}", f"{v_to}:{v_from}"

    def _key_a(self, a: ConverterAdapter):
        return self._keys(a.from_version, a.to_version)[0]

    def convert(self, v_from: str, v_to: str, obj: dict) -> dict:
        key_fwd, key_bwd = self._keys(v_from, v_to)

        if key_fwd in self._converters:
            return self._converters[key_fwd].forward(obj)

        if key_bwd in self._converters:
            return self._converters[key_bwd].backward(obj)

        raise Exception(f"Conversion from {v_from} to {v_to} is not supported")


class Converter_from_v1alpha1_to_v1beta1(Converter):
    def forward(self, obj: dict) -> dict:
        major, minor = obj["spec"]["version"].split(".")
        obj["spec"]["version"] = {
            "major": int(major),
            "minor": int(minor),
        }
        return obj

    def backward(self, obj: dict) -> dict:
        version = obj["spec"]["version"]
        obj["spec"]["version"] = f'{version["major"]}.{version["minor"]}'
        return obj


class Converter_from_v1beta1_to_v1(Converter):
    def forward(self, obj: dict) -> dict:
        obj["spec"]["modules"] = [{"name": m} for m in obj["spec"]["modules"]]
        return obj

    def backward(self, obj: dict) -> dict:
        obj["spec"]["modules"] = [m["name"] for m in obj["spec"]["modules"]]
        return obj


if __name__ == "__main__":
    hook.run(main, config=config)
