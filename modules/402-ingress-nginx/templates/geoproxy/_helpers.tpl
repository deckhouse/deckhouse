{{/* Collect map: { "<license>": ["GeoLite2-City","GeoLite2-ASN", ...], ... } */}}
{{- define "geoip_collect_license_editions" -}}
{{- $controllers := .controllers | default (list) -}}
{{- $defaultIDs := .defaultIDs | default (list "GeoLite2-City" "GeoLite2-ASN") -}}
{{- $out := dict -}}

{{- range $crd := $controllers }}
  {{- $spec := (get $crd "spec") | default dict -}}
  {{- $geo  := (get $spec "geoIP2") | default dict -}}
  {{- $lic  := (get $geo "maxmindLicenseKey") | default "" | toString | trim -}}
  {{- if $lic }}
    {{- $ids := (get $geo "maxmindEditionIDs") | default $defaultIDs -}}
    {{- $ids = (ternary $ids (list $ids) (kindIs "slice" $ids)) -}}
    {{- $norm := list -}}
    {{- range $ids }}
      {{- $norm = append $norm ((. | toString | trim)) -}}
    {{- end -}}

    {{- $existing := (get $out $lic) | default (list) -}}
    {{- $_ := set $out $lic (uniq (concat $existing $norm)) -}}
  {{- end }}
{{- end }}

{{- toJson $out -}}
{{- end -}}
