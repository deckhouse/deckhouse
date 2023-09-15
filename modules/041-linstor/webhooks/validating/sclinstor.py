#!/usr/bin/env python3

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

from deckhouse import hook
from dotmap import DotMap

config = """
configVersion: v1
kubernetesValidating:
  - name: linstor-storage-class.deckhouse.io
    rules:
      - apiGroups:   ["storage.k8s.io"]
        apiVersions: ["v1"]
        operations:  ["*"]
        resources:   ["storageclasses"]
        scope:       "Cluster"
"""

def main(ctx: hook.Context):
    request = DotMap(ctx.binding_context).review.request

    # print("request", request.pprint(pformat="json"))  # debug printing

    try:
        if request.operation == 'DELETE':
            ctx.output.validations.allow()

        if request.userInfo.username == 'system:serviceaccount:d8-storage-configurator:storage-configurator':
            ctx.output.validations.allow()

        if request.object.provisioner != 'linstor.csi.linbit.com':
            ctx.output.validations.allow()

        ctx.output.validations.deny(f"Manual {request.operation} is prohibited. Please {request.operation} LinstorStorageClass {request.name}")
    except Exception as e:
        ctx.output.validations.allow(f'There is error {e} while validation {request.operation} with StorageClass {request.name}. Passing.')

if __name__ == "__main__":
    hook.run(main, config=config)
