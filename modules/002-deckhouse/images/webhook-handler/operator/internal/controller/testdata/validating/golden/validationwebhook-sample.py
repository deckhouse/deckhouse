#!/usr/bin/python3
from typing import Optional

from deckhouse import hook
from dotmap import DotMap

config = """
configVersion: v1
kubernetesValidating:
- group: main
  includeSnapshotsFrom:
  - services
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
  group: main
  executeHookOnEvent: []
  executeHookOnSynchronization: false
  keepFullObjectsInMemory: false
  apiVersion: v1
  kind: Service
"""

def main(ctx: hook.Context):
    try:
        # DotMap is a dict with dot notation
        binding_context = DotMap(ctx.binding_context)
        message, allowed = validate(binding_context)
        if allowed:
            if message:
                ctx.output.validations.allow(message)  # warning
            else:
                ctx.output.validations.allow()
        else:
            ctx.output.validations.deny(message)
    except Exception as e:
        ctx.output.validations.error(str(e))

def validate(ctx: DotMap) -> tuple[Optional[str], bool]:
    resource = ctx.review.request.name
    if "test" in resource:
        return "TEST: service with \"test\" in .metadata.name", False
    return None, True


if __name__ == "__main__":
    hook.run(main, config=config)
