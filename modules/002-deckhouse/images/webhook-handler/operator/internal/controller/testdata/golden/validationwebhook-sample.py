#!/usr/bin/python3
from typing import Optional

from deckhouse import hook
from dotmap import DotMap

config = """
configVersion: v1
kubernetesValidating:
- group: main
  name: service.apps.kubernetes.io
  rules:
  - apiGroups:
    - '*'
    apiVersions:
    - '*'
    operations:
    - CREATE
    - UPDATE
    - DELETE
    resources:
    - services
    scope: '*'
kubernetes:
- name: services
  apiVersion: v1
  group: main
  kind: Service
"""

def main(ctx: hook.Context):
    try:
        # DotMap is a dict with dot notation
        binding_context = DotMap(ctx.binding_context)
        validate(binding_context, ctx.output.validations)
    except Exception as e:
        ctx.output.validations.error(str(e))

def validate(ctx: DotMap, output: hook.ValidationsCollector):
    resource = ctx.review.request.name
    if "test" in resource:
        output.deny("TEST: service with \"test\" in .metadata.name")
        return
    output.allow()


if __name__ == "__main__":
    hook.run(main, config=config)
