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
{{ toYaml .Kubernetes | indent 2 }}
{{- end }}
{{- end }}
{{- if (ge (len .KubernetesCustomResourceConversion) 1) }}
kubernetesCustomResourceConversion:
{{ toYaml .KubernetesCustomResourceConversion }}
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
