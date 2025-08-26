#!/usr/bin/python3
from typing import Optional

from deckhouse import hook
from dotmap import DotMap
from cryptography import x509
from cryptography.hazmat.backends import default_backend

config = """
configVersion: v1
kubernetesValidating:
- name: {{ .Name }}.deckhouse.io
  group: main
# \{\{ .Spec.Webhook }}
kubernetes:
- name: {{ .Name }}
  group: main
# \{\{ .Spec.Context }}
"""


def main(ctx: hook.Context):
    try:
        # DotMap is a dict with dot notation
        binding_context = DotMap(ctx.binding_context)
        validate(binding_context, ctx.output.validations)
    except Exception as e:
        ctx.output.validations.error(str(e))

{{ .Spec.Handler.Python }}

if __name__ == "__main__":
    hook.run(main, config=config)
