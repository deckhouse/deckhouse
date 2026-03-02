{{- /* Usage: {{ include "helm_lib_prometheus_rules_recursion" (list . <namespace> <root dir> [current dir] [file list]) }} */ -}}
{{- /* returns all the prometheus rules from <root dir>/ */ -}}
{{- /* current dir is optional — used for recursion but you can use it for partially generating rules */ -}}
{{- /* file list is optional - list of files to include (filters all files if provided) */ -}}
{{- define "helm_lib_prometheus_rules_recursion" -}}
  {{- $context := index . 0 }}    {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $namespace := index . 1 }}  {{- /* Namespace for creating rules */ -}}
  {{- $rootDir := index . 2 }}    {{- /* Rules root dir */ -}}
  {{- $currentDir := "" }}        {{- /* Current dir (optional) */ -}}
  {{- $fileList := list }}        {{- /* File list for filtering (optional) */ -}}

  {{- if gt (len .) 3 }} {{- $currentDir = index . 3 }} {{- else }} {{- $currentDir = $rootDir }} {{- end }}
  {{- if gt (len .) 4 }} {{- $fileList = index . 4 }} {{- end }}

  {{- $currentDirIndex := (sub ($currentDir | splitList "/" | len) 1) }}
  {{- $rootDirIndex := (sub ($rootDir | splitList "/" | len) 1) }}
  {{- $folderNamesIndex := (add1 $rootDirIndex) }}

  {{- range $path, $_ := $context.Files.Glob (print $currentDir "/*.{yaml,tpl}") }}
    {{- /* Filter files if fileList is provided */ -}}
    {{- $shouldProcess := true }}
    {{- if gt (len $fileList) 0 }}
      {{- $shouldProcess = has $path $fileList }}
    {{- end }}

    {{- if $shouldProcess }}
    {{- $fileName := ($path | splitList "/" | last ) }}
    {{- $definition := "" }}
    {{- if eq ($path | splitList "." | last) "tpl" -}}
      {{- $definition = tpl ($context.Files.Get $path) $context }}
    {{- else }}
      {{- $definition = $context.Files.Get $path }}
    {{- end }}

    {{- $definition = $definition | replace "__SCRAPE_INTERVAL__" (printf "%ds" ($context.Values.global.discovery.prometheusScrapeInterval | default 30)) | replace "__SCRAPE_INTERVAL_X_2__" (printf "%ds" (mul ($context.Values.global.discovery.prometheusScrapeInterval | default 30) 2)) | replace "__SCRAPE_INTERVAL_X_3__" (printf "%ds" (mul ($context.Values.global.discovery.prometheusScrapeInterval | default 30) 3)) | replace "__SCRAPE_INTERVAL_X_4__" (printf "%ds" (mul ($context.Values.global.discovery.prometheusScrapeInterval | default 30) 4)) }}

{{/*    Patch expression based on `d8_ignore_on_update` annotation*/}}

    {{ $definition = printf "Rules:\n%s" ($definition | nindent 2) }}
    {{- $definitionStruct :=  ( $definition | fromYaml )}}
    {{- if $definitionStruct.Error }}
      {{- fail ($definitionStruct.Error | toString) }}
    {{- end }}
    {{- range $rule := $definitionStruct.Rules }}

      {{- range $dedicatedRule := $rule.rules }}
        {{- if $dedicatedRule.annotations }}
          {{- if (eq (get $dedicatedRule.annotations "d8_ignore_on_update") "true") }}
            {{- $_ := set $dedicatedRule "expr" (printf "(%s) and ON() ((max(d8_is_updating) != 1) or ON() absent(d8_is_updating))" $dedicatedRule.expr) }}
          {{- end }}
        {{- end }}
      {{- end }}

    {{- end }}

    {{- $resourceName := (regexReplaceAllLiteral "\\.(yaml|tpl)$" $path "") }}
    {{- $resourceName = ($resourceName | replace " " "-" | replace "." "-" | replace "_" "-") }}
    {{- $resourceName = (slice ($resourceName | splitList "/") $folderNamesIndex | join "-") }}
    {{- $resourceName = (printf "%s-%s" $context.Chart.Name $resourceName) }}
    {{- $propagated := contains "propagated-" $resourceName }}
    {{- $hasObservabilityModule := has "observability" $context.Values.global.enabledModules }}
    {{- $useObservabilityRules := has "observability.deckhouse.io/v1alpha1/ClusterObservabilityMetricsRulesGroup" $context.Values.global.discovery.apiVersions }}
    {{- if and $hasObservabilityModule $useObservabilityRules }}
      {{- range $idx, $group := $definitionStruct.Rules }}
        {{- if $group.rules }}
          {{- $_ := unset $group "name" }}
          {{- $resourceName = $resourceName | replace "propagated-" "" }}
          {{- $groupResourceName := printf "%s-%d" $resourceName $idx }}
---
apiVersion: observability.deckhouse.io/v1alpha1
kind: {{ $propagated | ternary "ClusterObservabilityPropagatedMetricsRulesGroup" "ClusterObservabilityMetricsRulesGroup" }}
metadata:
  name: {{ $groupResourceName }}
  {{- include "helm_lib_module_labels" (list $context (dict "app" "prometheus" "prometheus" "main" "component" "rules")) | nindent 2 }}
spec:
  {{- $group | toYaml | nindent 2 }}
        {{- end }}
      {{- end }}
    {{- else }}
      {{- if $definitionStruct.Rules }}
        {{- $definition := $definitionStruct.Rules | toYaml }}
---
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: {{ $resourceName }}
  namespace: {{ $namespace }}
  {{- include "helm_lib_module_labels" (list $context (dict "app" "prometheus" "prometheus" "main" "component" "rules")) | nindent 2 }}
spec:
  groups:
    {{- $definition | nindent 4 }}
      {{- end }}
    {{- end }}
    {{- end }}
  {{- end }}

  {{- $subDirs := list }}
  {{- range $path, $_ := ($context.Files.Glob (print $currentDir "/**.{yaml,tpl}")) }}
    {{- $pathSlice := ($path | splitList "/") }}
    {{- $subDirs = append $subDirs (slice $pathSlice 0 (add $currentDirIndex 2) | join "/") }}
  {{- end }}

  {{- range $subDir := ($subDirs | uniq) }}
{{ include "helm_lib_prometheus_rules_recursion" (list $context $namespace $rootDir $subDir $fileList) }}
  {{- end }}
{{- end }}


{{- /* Usage: {{ include "helm_lib_prometheus_rules" (list . <namespace> [fileList]) }} */ -}}
{{- /* returns all the prometheus rules from monitoring/prometheus-rules/ optionally filtered by fileList */ -}}
{{- define "helm_lib_prometheus_rules" -}}
  {{- $context := index . 0 }}    {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $namespace := index . 1 }}  {{- /* Namespace for creating rules */ -}}
  {{- $rootDir := "monitoring/prometheus-rules" }}
  {{- $fileList := list }}
  {{- if gt (len .) 2 }}
    {{- $fileList = index . 2 }}
  {{- end }}
  {{- if ( $context.Values.global.enabledModules | has "operator-prometheus-crd" ) }}
{{- include "helm_lib_prometheus_rules_recursion" (list $context $namespace $rootDir $rootDir $fileList) }}
  {{- end }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_prometheus_target_scrape_timeout_seconds" (list . <timeout>) }} */ -}}
{{- /* returns adjust timeout value to scrape interval / */ -}}
{{- define "helm_lib_prometheus_target_scrape_timeout_seconds" -}}
  {{- $context := index . 0 }}  {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $timeout := index . 1 }}  {{- /* Target timeout in seconds */ -}}
  {{- $scrape_interval := (int $context.Values.global.discovery.prometheusScrapeInterval | default 30) }}
  {{- if gt $timeout $scrape_interval -}}
{{ $scrape_interval }}s
  {{- else -}}
{{ $timeout }}s
  {{- end }}
{{- end }}
