{{- define "static_machine_template_name" }}
  {{- $ng := index . 0 }}
  {{- $data := dict "name" $ng.name }}
  {{- if hasKey $ng.staticInstances "labelSelector" }}
    {{- $_ := set $data "labelSelector" $ng.staticInstances.labelSelector }}
  {{- end }}
  {{- $serialized := toYaml $data | sha256sum }}
  {{- printf "%s-%s" $ng.name (substr 0 8 $serialized) }}
{{- end }}
