{{/* Collect map: { "<license>": {"maxmindAccountID": "<account>", "editions": ["GeoLite2-City","GeoLite2-ASN", ...]}, ... } */}}
{{- define "geoip_collect_license_editions" -}}
{{- $controllers := .controllers | default (list) -}}
{{- $out := dict -}}

{{- range $crd := $controllers }}
  {{- $spec := (get $crd "spec") | default dict -}}
  {{- $geo  := (get $spec "geoIP2") | default dict -}}
  {{- $lic  := (get $geo "maxmindLicenseKey") | default "" | toString | trim -}}
  {{- $accRaw  := (((get $geo "maxmindAccountID") | default (get $geo "accountID") | default 0) | int) -}}
  {{- if $lic }}
    {{- $ids := (get $geo "maxmindEditionIDs") -}}
    {{- $existing := (get $out $lic) | default dict -}}
    {{- $existingAccRaw := ((get $existing "maxmindAccountID") | default 0) | int -}}
    {{- $existingEditions := (get $existing "editions") | default (list) -}}
    {{- $existingMirror := (get $existing "maxmindMirror") | toString | trim -}}
    {{- $existingSkipTLS := (get $existing "maxmindMirrorSkipTLSVerify") | default false -}}
    {{- $mergedEditions := (uniq (concat $existingEditions $ids)) -}}
    {{- $resolvedAcc := (ternary $accRaw $existingAccRaw (gt $accRaw 0)) -}}
    {{- $mirror := (get $geo "maxmindMirror") | toString | trim -}}
    {{- $skipTLS := (get $geo "maxmindMirrorSkipTLSVerify") | default false -}}
    {{- $resolvedMirror := (ternary $mirror $existingMirror (ne $mirror "")) -}}
    {{- $resolvedSkipTLS := (or $existingSkipTLS $skipTLS) -}}
    {{- $_ := set $out $lic (dict "maxmindAccountID" $resolvedAcc "editions" $mergedEditions "maxmindMirror" $resolvedMirror "maxmindMirrorSkipTLSVerify" $resolvedSkipTLS) -}}
  {{- end }}
{{- end }}

{{- toJson $out -}}
{{- end -}}
