{{- define "stringifyNodeSelector" }}
{{- $context := . }}
{{- $result := "" }}
{{- range $k, $v := $context }}
  {{- $result = printf "%s,%s=%s" $result $k $v }}
{{- end }}
{{- trimPrefix "," $result }}
{{- end }}

{{- define "slugify" }}
  {{- /* https://gitlab.com/gitlab-org/gitlab/-/blob/6db59634ecbb1581bbb16b627b9631ca96ce2e8d/lib/gitlab/utils.rb#L100 */}}
  {{- $oldName := index . 0 }}
  {{- $rootContext := index . 1 }}

  {{- $newName := lower $oldName }}
  {{- $newName = regexReplaceAllLiteral "[^a-z0-9]" $newName "-" }}

  {{- if gt (len $newName) 63 }}
    {{- if not ( index $rootContext.Release "bigNamePostfixes" ) }}
      {{- $_ := set $rootContext.Release "bigNamePostfixes" dict }}
    {{- end }}

    {{- /* This will allow us to reuse random string after helm release upgrade */}}
    {{- if not ( index $rootContext.Release.bigNamePostfixes $newName ) }}
      {{- $_ := set $rootContext.Release.bigNamePostfixes $newName ( randAlphaNum 10 | lower ) }}
    {{- end }}

    {{- $newNameShortened := substr 0 52 $newName }}
    {{- $newName = printf "%s-%s" $newNameShortened ( index $rootContext.Release.bigNamePostfixes $newName ) }}
  {{- end }}

  {{- $newName = regexReplaceAllLiteral "(^-+|-+$)" $newName "" }}
  {{- print $newName }}
{{- end }}

{{- define "normalize" }}
  {{- $newName := lower . }}
  {{- $newName = regexReplaceAll "\\W+" $newName "-" }}
  {{- $newName = regexReplaceAll "(^-+|-+$)" $newName "" }}
  {{- print $newName }}
{{- end }}