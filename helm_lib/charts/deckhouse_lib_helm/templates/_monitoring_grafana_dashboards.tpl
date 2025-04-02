{{- /* Usage: {{ include "helm_lib_grafana_dashboard_definitions_recursion" (list . <root dir> [current dir]) }} */ -}}
{{- /* returns all the dashboard-definintions from <root dir>/ */ -}}
{{- /* current dir is optional — used for recursion but you can use it for partially generating dashboards */ -}}
{{- define "helm_lib_grafana_dashboard_definitions_recursion" -}}
  {{- $context := index . 0 }} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $rootDir := index . 1 }} {{- /* Dashboards root dir */ -}}
  {{- /* Dashboards current dir */ -}}

  {{- $currentDir := "" }}
  {{- if gt (len .) 2 }} {{- $currentDir = index . 2 }} {{- else }} {{- $currentDir = $rootDir }} {{- end }}

  {{- $currentDirIndex := (sub ($currentDir | splitList "/" | len) 1) }}
  {{- $rootDirIndex := (sub ($rootDir | splitList "/" | len) 1) }}
  {{- $folderNamesIndex := (add1 $rootDirIndex) }}

  {{- range $path, $_ := $context.Files.Glob (print $currentDir "/*.json") }}
    {{- $fileName := ($path | splitList "/" | last ) }}
    {{- $definition := ($context.Files.Get $path) }}

    {{- $folder := (index ($currentDir | splitList "/") $folderNamesIndex | replace "-" " " | title) }}
    {{- $resourceName := (regexReplaceAllLiteral "\\.json$" $path "") }}
    {{- $resourceName = ($resourceName | replace " " "-" | replace "." "-" | replace "_" "-") }}
    {{- $resourceName = (slice ($resourceName | splitList "/") $folderNamesIndex | join "-") }}
    {{- $resourceName = (printf "%s-%s" $context.Chart.Name $resourceName) }}

{{ include "helm_lib_single_dashboard" (list $context $resourceName $folder $definition) }}
  {{- end }}

  {{- range $path, $_ := $context.Files.Glob (print $currentDir "/*.tpl") }}
    {{- $fileName := ($path | splitList "/" | last ) }}
    {{- $definition := tpl ($context.Files.Get $path) $context }}

    {{- $folder := (index ($currentDir | splitList "/") $folderNamesIndex | replace "-" " " | title) }}
    {{- $resourceName := (regexReplaceAllLiteral "\\.tpl$" $path "") }}
    {{- $resourceName = ($resourceName | replace " " "-" | replace "." "-" | replace "_" "-") }}
    {{- $resourceName = (slice ($resourceName | splitList "/") $folderNamesIndex | join "-") }}
    {{- $resourceName = (printf "%s-%s" $context.Chart.Name $resourceName) }}

{{ include "helm_lib_single_dashboard" (list $context $resourceName $folder $definition) }}
  {{- end }}

  {{- $subDirs := list }}
  {{- range $path, $_ := ($context.Files.Glob (print $currentDir "/**.json")) }}
    {{- $pathSlice := ($path | splitList "/") }}
    {{- $subDirs = append $subDirs (slice $pathSlice 0 (add $currentDirIndex 2) | join "/") }}
  {{- end }}
  {{- range $path, $_ := ($context.Files.Glob (print $currentDir "/**.tpl")) }}
    {{- $pathSlice := ($path | splitList "/") }}
    {{- $subDirs = append $subDirs (slice $pathSlice 0 (add $currentDirIndex 2) | join "/") }}
  {{- end }}

  {{- range $subDir := ($subDirs | uniq) }}
{{ include "helm_lib_grafana_dashboard_definitions_recursion" (list $context $rootDir $subDir) }}
  {{- end }}

{{- end }}


{{- /* Usage: {{ include "helm_lib_grafana_dashboard_definitions" . }} */ -}}
{{- /* returns dashboard-definintions from monitoring/grafana-dashboards/ */ -}}
{{- define "helm_lib_grafana_dashboard_definitions" -}}
  {{- $context := . }} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- if ( $context.Values.global.enabledModules | has "prometheus-crd" ) }}
{{- include "helm_lib_grafana_dashboard_definitions_recursion" (list $context "monitoring/grafana-dashboards") }}
  {{- end }}
{{- end }}


{{- /* Usage: {{ include "helm_lib_single_dashboard" (list . "dashboard-name" "folder" $dashboard) }} */ -}}
{{- /* renders a single dashboard */ -}}
{{- define "helm_lib_single_dashboard" -}}
  {{- $context := index . 0 }}       {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $resourceName := index . 1 }}  {{- /* Dashboard name */ -}}
  {{- $folder := index . 2 }}        {{- /* Folder */ -}}
  {{- $definition := index . 3 }}    {{/* Dashboard definition */}}
  {{- $propagated := contains "-propagated-" $resourceName }}
  {{- $resourceName = $resourceName | replace "-propagated-" "-" }}
---
apiVersion: deckhouse.io/v1
kind: GrafanaDashboardDefinition
metadata:
  name: d8-{{ $resourceName }}
  {{- include "helm_lib_module_labels" (list $context (dict "prometheus.deckhouse.io/grafana-dashboard" "" "observability.deckhouse.io/skip-dashboard-conversion" "")) | nindent 2 }}
spec:
  folder: {{ $folder | quote }}
  definition: |
    {{- $definition | nindent 4 }}
  {{- if $context.Values.global.enabledModules | has "observability" }}
---
apiVersion: observability.deckhouse.io/v1alpha1
kind: {{ $propagated | ternary "ClusterObservabilityPropagatedDashboard" "ClusterObservabilityDashboard" }}
metadata:
  annotations:
    metadata.deckhouse.io/category: {{ $folder | quote }}
  name: d8-{{ $resourceName }}
  {{- include "helm_lib_module_labels" (list $context (dict "observability.deckhouse.io/dashboard-origin" "module")) | nindent 2 }}
spec:
  definition: |
    {{- $definition | nindent 4 }}
  {{- end }}
{{- end }}
