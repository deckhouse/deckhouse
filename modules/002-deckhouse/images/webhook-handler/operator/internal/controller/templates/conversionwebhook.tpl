#!/usr/bin/python3

import typing

from dotmap import DotMap
from deckhouse import hook, utils

config = """
configVersion: v1
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
{{- if (ge (len .Conversions) 1) }}
kubernetesCustomResourceConversion:
{{- range .Conversions }}
  - name: {{.From}}_to_{{.To}}
    crdName: {{ $.Name}}
    conversions:
    - fromVersion: {{ getGroup $.Name }}/{{.From}}
      toVersion: {{ getGroup $.Name }}/{{.To}}
{{- if .IncludeSnapshotsFrom }}
    includeSnapshotsFrom:
{{ toYaml .IncludeSnapshotsFrom | indent 6 }}
{{- end }}
{{- end }}
{{- end }}
"""

class Conversion(utils.BaseConversionHook):
    def __init__(self, ctx: hook.Context):
        super().__init__(ctx)

{{- range .Conversions}}
{{ .Handler.Python | indent 4 }}
{{- end }}

def main(ctx: hook.Context):
    Conversion(ctx).run()


if __name__ == "__main__":
    hook.run(main, config=config)
