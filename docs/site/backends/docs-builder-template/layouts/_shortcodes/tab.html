{{- if .Parent -}}
  {{- $name := .Get "name" | default (.Get 0) | strings.TrimSpace -}}
  {{- $content := partial "tab-content.html" .Inner -}}
  {{- if not $name -}}
    {{- errorf "[%s] %q: tab shortcode requires a label ({{< tab \"Label\" >}} or name=\"Label\")" site.Language.Lang .Page.Path -}}
  {{- end -}}
  {{- if not (.Parent.Store.Get "tabs") -}}
    {{- .Parent.Store.Set "tabs" slice -}}
  {{- end -}}
  {{- $.Parent.Store.Add "tabs" (dict "name" $name "content" $content) -}}
{{- else -}}
  {{- errorf "[%s] %q: tab shortcode missing parent" site.Language.Lang .Page.Path -}}
{{- end -}}
