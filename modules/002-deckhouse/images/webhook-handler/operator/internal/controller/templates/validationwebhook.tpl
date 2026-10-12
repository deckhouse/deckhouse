#!/usr/bin/python3
from typing import Optional

from deckhouse import hook, validations
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
    # validate gets the binding context as a DotMap, a dict with dot notation.
    validations.handle(ctx, validate)

{{ .Handler.Python }}

if __name__ == "__main__":
    hook.run(main, config=config)
