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
  - fromVersion: v1alpha1
    toVersion: v1beta1
  - fromVersion: v1beta1
    toVersion: v1

"""


def main(ctx: hook.Context):
    bctx = DotMap(ctx.binding_context)
    if (
        bctx.fromVersion == "deckhouse.io/v1alpha1"
        and bctx.toVersion == "deckhouse.io/v1beta1"
    ):
        for obj in bctx.review.request.objects:
            try:
                ctx.output.conversions.collect(conv_v1alpha1_to_v1beta1(obj).toDict())
            except Exception as e:
                ctx.output.conversions.error(str(e))
                return

    if (
        bctx.fromVersion == "deckhouse.io/v1beta1"
        and bctx.toVersion == "deckhouse.io/v1"
    ):
        for obj in bctx.review.request.objects:
            try:
                ctx.output.conversions.collect(conv_v1beta1_to_v1(obj).toDict())
            except Exception as e:
                ctx.output.conversions.error(str(e))
                return

    if (
        bctx.fromVersion == "deckhouse.io/v1alpha1"
        and bctx.toVersion == "deckhouse.io/v1"
    ):
        for obj in bctx.review.request.objects:
            try:
                ctx.output.conversions.collect(
                    conv_v1beta1_to_v1(conv_v1beta1_to_v1(obj)).toDict()
                )
            except Exception as e:
                ctx.output.conversions.error(str(e))
                return


conversions = [
    # to apiVersion, conv function
    ("deckhouse.io/v1beta1", conv_v1alpha1_to_v1beta1),
    ("deckhouse.io/v1", conv_v1beta1_to_v1),
]


def conv_v1alpha1_to_v1beta1(obj):
    new_obj = DotMap(obj)  # deep copy
    new_obj.apiVersion = "deckhouse.io/v1beta1"
    major, minor = new_obj.spec.version.split(".")
    new_obj.spec.version = {
        "major": int(major),
        "minor": int(minor),
    }
    return new_obj


def conv_v1beta1_to_v1(obj):
    new_obj = DotMap(obj)  # deep copy
    new_obj.apiVersion = "deckhouse.io/v1"
    new_obj.spec.modules = [{"name": m} for m in new_obj.spec.modules]
    return new_obj


if __name__ == "__main__":
    hook.run(main, config=config)
