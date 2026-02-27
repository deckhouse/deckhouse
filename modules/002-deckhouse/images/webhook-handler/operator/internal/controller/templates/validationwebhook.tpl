#!/usr/bin/python3
from typing import Optional

from deckhouse import hook
from dotmap import DotMap

config = """
configVersion: v1
{{- if .ValidatingWebhook }}
kubernetesValidating:
{{ list .ValidatingWebhook | toYaml }}
{{- end }}
{{- if (ge (len .Context) 1) }}
kubernetes:
{{- range .Context}}
- name: {{ .Name }}
  group: main
  executeHookOnEvent: []
  executeHookOnSynchronization: false
  keepFullObjectsInMemory: false
{{ toYaml .Kubernetes | indent 2 }}
{{- end }}
{{- end }}
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

{{ .Handler.Python }}

if __name__ == "__main__":
    hook.run(main, config=config)
