#!/usr/bin/python3
from typing import Optional

from deckhouse import hook, validations
from dotmap import DotMap

config = """
configVersion: v1
kubernetesValidating:
- includeSnapshotsFrom:
  - some_node
  - some_node
  matchConditions:
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
kubernetes:
- name: some_node
  group: main
  executeHookOnEvent: []
  executeHookOnSynchronization: false
  keepFullObjectsInMemory: false
  apiVersion: v1
  jqFilter: |
    { "nodeName": .metadata.name }
  kind: Node
  labelSelector:
    matchLabels:
      foo: bar
  nameSelector:
    matchNames:
    - global
  namespace:
    labelSelector:
      matchLabels:
        bar: foo
- name: some_node
  group: main
  executeHookOnEvent: []
  executeHookOnSynchronization: false
  keepFullObjectsInMemory: false
  apiVersion: v1
  jqFilter: |
    { "nodeName": .metadata.name }
  kind: Node
  labelSelector:
    matchLabels:
      foo: bar
  nameSelector:
    matchNames:
    - global
  namespace:
    labelSelector:
      matchLabels:
        bar: foo
"""

def main(ctx: hook.Context):
    # validate gets the binding context as a DotMap, a dict with dot notation.
    validations.handle(ctx, validate)

def validate(ctx: DotMap) -> tuple[Optional[str], bool]:
  # logic here
  return None, True

if __name__ == "__main__":
    hook.run(main, config=config)
