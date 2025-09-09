#!/usr/bin/python3
from typing import Optional

from deckhouse import hook
from dotmap import DotMap

config = """
configVersion: v1
kubernetesValidating:
{{ list .ValidatingWebhook | toYaml }}
{{- if (ge (len .Context) 1) }}
kubernetes:
{{- range .Context}}
- name: {{ .Name }}
{{ toYaml .Kubernetes | indent 2 }}
{{- end }}
{{- end }}
"""

def main(ctx: hook.Context):
    try:
        # DotMap is a dict with dot notation
        binding_context = DotMap(ctx.binding_context)
        validate(binding_context, ctx.output.validations)
    except Exception as e:
        ctx.output.validations.error(str(e))

{{ .Handler.Python }}

if __name__ == "__main__":
    hook.run(main, config=config)
