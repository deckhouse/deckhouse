{{/* Collect map: { "<license>": {"accountID": "<account>", "editions": ["GeoLite2-City","GeoLite2-ASN", ...]}, ... } */}}
{{- define "geoip_collect_license_editions" -}}
{{- $controllers := .controllers | default (list) -}}
{{- $out := dict -}}

{{- range $crd := $controllers }}
  {{- $spec := (get $crd "spec") | default dict -}}
  {{- $geo  := (get $spec "geoIP2") | default dict -}}
  {{- $lic  := (get $geo "maxmindLicenseKey") | default "" | toString | trim -}}
  {{- $accRaw  := ((get $geo "accountID") | default 0) | int -}}
  {{- if $lic }}
    {{- $ids := (get $geo "maxmindEditionIDs") -}}
    {{- $existing := (get $out $lic) | default dict -}}
    {{- $existingAccRaw := ((get $existing "accountID") | default 0) | int -}}
    {{- $existingEditions := (get $existing "editions") | default (list) -}}
    {{- $mergedEditions := (uniq (concat $existingEditions $ids)) -}}
    {{- $resolvedAcc := (ternary $accRaw $existingAccRaw (gt $accRaw 0)) -}}
    {{- $_ := set $out $lic (dict "accountID" $resolvedAcc "editions" $mergedEditions) -}}
  {{- end }}
{{- end }}

{{- toJson $out -}}
{{- end -}}
