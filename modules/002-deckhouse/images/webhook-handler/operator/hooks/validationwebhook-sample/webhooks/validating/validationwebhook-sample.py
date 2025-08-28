
#!/usr/bin/python3
from typing import Optional

from deckhouse import hook
from dotmap import DotMap
from cryptography import x509
from cryptography.hazmat.backends import default_backend

config = """
configVersion: v1
kubernetesValidating:
- group: main
  name: test
  clientconfig:
      url: null
      service:
          namespace: example-namespace
          name: example-service
          path: null
          port: null
      cabundle: []
  rules:
      - operations:
          - CREATE
          - UPDATE
        rule:
          apigroups:
              - apps
          apiversions:
              - v1
              - v1beta1
          resources:
              - deployments
              - replicasets
          scope: Namespaced
  failurepolicy: null
  matchpolicy: null
  namespaceselector:
      matchlabels: {}
      matchexpressions:
          - key: runlevel
            operator: NotIn
            values:
              - "0"
              - "1"
  objectselector:
      matchlabels:
          foo: bar
      matchexpressions: []
  sideeffects: None
  timeoutseconds: null
  admissionreviewversions:
      - v1
  matchconditions:
      - name: yyyy
        expression: request.resource.group != "rbac.authorization.k8s.io"
kubernetes:
- name: validationwebhook-sample
  group: main
  - name: some_node
    kubernetes:
      apiversion: v1
      kind: Node
      nameselector:
          matchNames:
              - global
      matchnames: []
      labelselector:
          matchLabels:
              foo: bar
      matchlabels: {}
      foo: ""
      namespaceselector:
          matchLabels:
              bar: foo
      jqfilter:
          nodename: .metadata.name
"""

def main(ctx: hook.Context):
    try:
        # DotMap is a dict with dot notation
        binding_context = DotMap(ctx.binding_context)
        validate(binding_context, ctx.output.validations)
    except Exception as e:
        ctx.output.validations.error(str(e))

def validate(request: admission.Request, context: []Object || Context) -> tuple[Optional[str], bool]:
  // logic here
  return "message", True


if __name__ == "__main__":
    hook.run(main, config=config)
