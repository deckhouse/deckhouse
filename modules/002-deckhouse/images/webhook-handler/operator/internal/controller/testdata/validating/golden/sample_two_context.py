#!/usr/bin/python3
from typing import Optional

from deckhouse import hook
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
kubernetes:
- name: some_node
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
    try:
        # DotMap is a dict with dot notation
        binding_context = DotMap(ctx.binding_context)
        validate(binding_context, ctx.output.validations)
    except Exception as e:
        ctx.output.validations.error(str(e))

def validate(bindingcontext, output):
  # logic here
  return

if __name__ == "__main__":
    hook.run(main, config=config)
