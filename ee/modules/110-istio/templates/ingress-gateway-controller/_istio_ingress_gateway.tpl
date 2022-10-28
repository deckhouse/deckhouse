{{- define "ingress_gateway_name" -}}
{{- $name := .  -}}
{{ printf "ingressgateway-%s" $name -}}
{{- end -}}
