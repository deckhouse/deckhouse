#!/usr/bin/python3
from typing import Optional

from deckhouse import hook, validations
from dotmap import DotMap

config = """
configVersion: v1
kubernetesValidating:
- matchConditions:
  - expression: request.resource.group != "rbac.authorization.k8s.io"
    name: yyyy
  name: validationwebhook.deployments.apps
  namespace:
    labelSelector:
      matchExpressions:
      - key: runlevel
        operator: NotIn
        values:
        - "0"
        - "1"
  rules:
  - apiGroups:
    - apps
    apiVersions:
    - v1
    - v1beta1
    operations:
    - CREATE
    - UPDATE
    resources:
    - deployments
    - replicasets
    scope: Namespaced
"""

def main(ctx: hook.Context):
    # validate gets the binding context as a DotMap, a dict with dot notation.
    validations.handle(ctx, validate)

def validate(ctx: DotMap) -> tuple[Optional[str], bool]:
  # logic here
  return None, True

if __name__ == "__main__":
    hook.run(main, config=config)
