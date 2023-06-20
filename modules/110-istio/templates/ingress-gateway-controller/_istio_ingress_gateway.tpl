{{- define "ingress_gateway_name" -}}
{{- $name := .  -}}
{{ printf "ingress-gateway-controller-%s" $name -}}
{{- end -}}
