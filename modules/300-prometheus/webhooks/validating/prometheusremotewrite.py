#!/usr/bin/python3
from typing import Optional

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

from deckhouse import hook
from dotmap import DotMap
from cryptography import x509
from cryptography.hazmat.backends import default_backend

config = """
configVersion: v1
kubernetesValidating:
- name: prometheusremotewrite-policy.deckhouse.io
  group: main
  rules:
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["v1alpha1", "v1"]
    operations:  ["CREATE", "UPDATE"]
    resources:   ["prometheusremotewrites"]
    scope:       "Cluster"
kubernetes:
- name: prometheusremotewrites
  group: main
  executeHookOnEvent: []
  executeHookOnSynchronization: false
  keepFullObjectsInMemory: false
  apiVersion: deckhouse.io/v1
  kind: PrometheusRemoteWrite
  jqFilter: |
    {
      "name": .metadata.name,
      "url": .spec.url,
    }
"""


def main(ctx: hook.Context):
    try:
        # DotMap is a dict with dot notation
        binding_context = DotMap(ctx.binding_context)
        validate(binding_context, ctx.output.validations)
    except Exception as e:
        ctx.output.validations.error(str(e))


def validate(ctx: DotMap, output: hook.ValidationsCollector):
    operation = ctx.review.request.operation
    if operation == "CREATE" or operation == "UPDATE":
        validate_creation_or_update(ctx, output)
    else:
        raise Exception(f"Unknown operation {ctx.operation}")


def validate_creation_or_update(ctx: DotMap, output: hook.ValidationsCollector):
    error = check_verify_url_signatures(ctx)
    if error is not None:
        output.deny(error)
        return
    error = check_verify_ca_signatures(ctx)
    if error is not None:
        output.deny(error)
        return
    output.allow()


# check that all image references don't have intersection, it's required by ratify
# https://ratify.dev/docs/plugins/verifier/cosign/#scopes
def check_verify_url_signatures(ctx: DotMap) -> Optional[str]:
    url = ctx.review.request.object.spec.url
    if len(url) == 0:
        return "Url has empty string"
    filtered_name = ctx.review.request.name
    if len([rw for rw in ctx.snapshots.prometheusremotewrites if rw.filterResult.url == url and rw.filterResult.name != filtered_name]) > 0:
        return f"Remote write URL {url} is already in use"
    # search in all prometheusremote write if url alredy used
    return None
    
def check_verify_ca_signatures(ctx: DotMap) -> Optional[str]:
    ca = ctx.review.request.object.spec.tlsConfig.ca
    if len(ca) == 0:
        return None
    try:
        x509.load_pem_x509_certificate(ca.encode(), default_backend())
        return None
    except Exception as e:
        return f"Certificate verification failed: {e}"
    
if __name__ == "__main__":
    hook.run(main, config=config)
