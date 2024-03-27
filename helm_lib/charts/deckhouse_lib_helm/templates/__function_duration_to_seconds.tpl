{{- /* Usage: {{ int (include "helm_lib_duration_to_seconds" $duration) }} */ -}}
{{- /* */ -}}
{{- /* returns the number of seconds */ -}}
{{- define "helm_lib_duration_to_seconds" -}}
{{-   $durationStr := . -}}
{{-   $matches := regexFindAll "[0-9]+\\w" $durationStr -1 -}}
{{-   $seconds := 0 -}}
{{-   range $match := $matches -}}
{{-     if hasSuffix "d" $match -}}
{{-       $seconds = add $seconds (mul (atoi (trimSuffix "d" $match)) 86400) -}}
{{-     else if hasSuffix "h" $match -}}
{{-       $seconds = add $seconds (mul (atoi (trimSuffix "h" $match)) 3600) -}}
{{-     else if hasSuffix "m" $match -}}
{{-       $seconds = add $seconds (mul (atoi (trimSuffix "m" $match)) 60) -}}
{{-     else if hasSuffix "s" $match -}}
{{-       $seconds = add $seconds (atoi (trimSuffix "s" $match)) -}}
{{-     end -}}
{{-   end -}}
{{    $seconds }}
{{- end -}}
