{{/* Collect map: { "<license>": {"maxmindAccountID": "<account>", "editions": ["GeoLite2-City","GeoLite2-ASN", ...], "maxmindMirror": {"url": "<mirror>", "insecureSkipVerify": <bool>}}, ... } */}}
{{- define "geoip_collect_license_editions" -}}
{{- $controllers := .controllers | default (list) -}}
{{- $out := dict -}}

{{- range $crd := $controllers }}
  {{- $spec := (get $crd "spec") | default dict -}}
  {{- $geo  := (get $spec "geoIP2") | default dict -}}
  {{- $lic  := (get $geo "maxmindLicenseKey") | default "" | toString | trim -}}
  {{- $mirror := (get $geo "maxmindMirror") | default dict -}}
  {{- $mirrorURL := (get $mirror "url") | default "" | toString | trim -}}
  {{- $skipTLS := (get $mirror "insecureSkipVerify") | default false -}}
  {{- $accRaw  := (((get $geo "maxmindAccountID") | default (get $geo "accountID") | default 0) | int) -}}
  {{- $key := $lic -}}
  {{- if and (eq $key "") (ne $mirrorURL "") }}
    {{- $hash := (sha1sum $mirrorURL) -}}
    {{- $key = printf "mirror:%s" (substr 0 8 $hash) -}}
  {{- end }}
  {{- if $key }}
    {{- $ids := (get $geo "maxmindEditionIDs") | default (list) -}}
    {{- $existing := (get $out $key) | default dict -}}
    {{- $existingAccRaw := ((get $existing "maxmindAccountID") | default 0) | int -}}
    {{- $existingEditions := (get $existing "editions") | default (list) -}}
    {{- $existingMirror := (get $existing "maxmindMirror") | default dict -}}
    {{- $existingMirrorURL := (get $existingMirror "url") | default "" | toString | trim -}}
    {{- $existingSkipTLS := (get $existingMirror "insecureSkipVerify") | default false -}}
    {{- $mergedEditions := (uniq (concat $existingEditions $ids)) -}}
    {{- $resolvedAcc := (ternary $accRaw $existingAccRaw (gt $accRaw 0)) -}}
    {{- $resolvedMirrorURL := (ternary $mirrorURL $existingMirrorURL (ne $mirrorURL "")) -}}
    {{- $resolvedSkipTLS := (or $existingSkipTLS $skipTLS) -}}
    {{- $resolvedMirror := dict -}}
    {{- if or $resolvedMirrorURL $resolvedSkipTLS }}
      {{- $_ := set $resolvedMirror "url" $resolvedMirrorURL -}}
      {{- $_ := set $resolvedMirror "insecureSkipVerify" $resolvedSkipTLS -}}
    {{- end }}
    {{- $_ := set $out $key (dict "maxmindAccountID" $resolvedAcc "editions" $mergedEditions "maxmindMirror" $resolvedMirror) -}}
  {{- end }}
{{- end }}

{{- toJson $out -}}
{{- end -}}
