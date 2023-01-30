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
from dotmap import DotMap

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
"""


def main(ctx: hook.Context):
    try:
        # DotMap is a dict with dot notation
        bctx = DotMap(ctx.binding_context)

        print(bctx.pprint(pformat="json"))  # debug printing

        for obj in bctx.review.request.objects:
            converted = convert(bctx.fromVersion, bctx.toVersion, obj)

            print(converted.pprint(pformat="json"))  # debug printing

            # DotMap is not JSON serializable, we need raw dict
            ctx.output.conversions.collect(converted.toDict())
    except Exception as e:
        ctx.output.conversions.error(str(e))
        return


def convert(v_from: str, v_to: str, obj: DotMap) -> DotMap:
    # As we didnt't declare straight conversion from v1alpha1 -> v1, it will be done in two
    # sequential requests. Hence, we take care only about v1alpha1 -> v1beta1 and v1beta1 -> v1
    # conversions.
    match v_from, v_to:
        case "deckhouse.io/v1alpha1", "deckhouse.io/v1beta1":
            return conv_v1alpha1_to_v1beta1(obj)
        case "deckhouse.io/v1beta1", "deckhouse.io/v1":
            return conv_v1beta1_to_v1(obj)
        case _:
            raise Exception(f"Conversion from {v_from} to {v_to} is not supported")


def conv_v1alpha1_to_v1beta1(obj: DotMap) -> DotMap:
    new_obj = DotMap(obj)  # deep copy
    new_obj.apiVersion = "deckhouse.io/v1beta1"
    major, minor = new_obj.spec.version.split(".")
    new_obj.spec.version = {
        "major": int(major),
        "minor": int(minor),
    }
    return new_obj


def conv_v1beta1_to_v1(obj: DotMap) -> DotMap:
    new_obj = DotMap(obj)  # deep copy
    new_obj.apiVersion = "deckhouse.io/v1"
    new_obj.spec.modules = [{"name": m} for m in new_obj.spec.modules]
    return new_obj


if __name__ == "__main__":
    hook.run(main, config=config)
