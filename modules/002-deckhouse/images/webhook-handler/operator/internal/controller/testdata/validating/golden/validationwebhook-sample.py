#!/usr/bin/python3
from typing import Optional

from deckhouse import hook, validations
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
    # validate gets the binding context as a DotMap, a dict with dot notation.
    validations.handle(ctx, validate)

def validate(ctx: DotMap) -> tuple[Optional[str], bool]:
    resource = ctx.review.request.name
    if "test" in resource:
        return "TEST: service with \"test\" in .metadata.name", False
    return None, True


if __name__ == "__main__":
    hook.run(main, config=config)
