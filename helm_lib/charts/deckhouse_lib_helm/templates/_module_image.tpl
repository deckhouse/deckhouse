{{- /* Usage: {{ include "helm_lib_module_image" (list . "<container-name>" "<module-name>(optional)") }} */ -}}
{{- /* returns image name */ -}}
{{- define "helm_lib_module_image" }}
  {{- $context := index . 0 }} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $containerName := index . 1 | trimAll "\"" }} {{- /* Container name */ -}}
  {{- $rawModuleName := $context.Chart.Name }}
  {{- if ge (len .) 3 }}
  {{- $rawModuleName = (index . 2) }} {{- /* Optional module name */ -}}
  {{- end }}
  {{- $moduleName := (include "helm_lib_module_camelcase_name" $rawModuleName) }}
  {{- $imageDigest := index $context.Values.global.modulesImages.digests $moduleName $containerName }}
  {{- if not $imageDigest }}
  {{- $error := (printf "Image %s.%s has no digest" $moduleName $containerName ) }}
  {{- fail $error }}
  {{- end }}
  {{- $registryBase := $context.Values.global.modulesImages.registry.base }}
  {{- /*  handle external modules registry */}}
  {{- if index $context.Values $moduleName }}
    {{- if index $context.Values $moduleName "registry" }}
      {{- if index $context.Values $moduleName "registry" "base" }}
        {{- $host := trimAll "/" (index $context.Values $moduleName "registry" "base") }}
        {{- $path := trimAll "/" (include "helm_lib_module_kebabcase_name" $rawModuleName) }}
        {{- $registryBase = join "/" (list $host $path) }}
      {{- end }}
    {{- end }}
  {{- end }}
  {{- /* end of external module handling block */}}
  {{- printf "%s@%s" $registryBase $imageDigest }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_image_no_fail" (list . "<container-name>") }} */ -}}
{{- /* returns image name if found */ -}}
{{- define "helm_lib_module_image_no_fail" }}
  {{- $context := index . 0 }} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $containerName := index . 1 | trimAll "\"" }} {{- /* Container name */ -}}
  {{- $moduleName := (include "helm_lib_module_camelcase_name" $context) }}
  {{- if ge (len .) 3 }}
  {{- $moduleName = (include "helm_lib_module_camelcase_name" (index . 2)) }} {{- /* Optional module name */ -}}
  {{- end }}
  {{- $imageDigest := index $context.Values.global.modulesImages.digests $moduleName $containerName }}
  {{- if $imageDigest }}
    {{- $registryBase := $context.Values.global.modulesImages.registry.base }}
    {{- if index $context.Values $moduleName }}
      {{- if index $context.Values $moduleName "registry" }}
        {{- if index $context.Values $moduleName "registry" "base" }}
          {{- $host := trimAll "/" (index $context.Values $moduleName "registry" "base") }}
          {{- $path := trimAll "/" $context.Chart.Name }}
          {{- $registryBase = join "/" (list $host $path) }}
        {{- end }}
      {{- end }}
    {{- end }}
    {{- printf "%s@%s" $registryBase $imageDigest }}
  {{- end }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_common_image" (list . "<container-name>") }} */ -}}
{{- /* returns image name from common module */ -}}
{{- define "helm_lib_module_common_image" }}
  {{- $context := index . 0 }} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $containerName := index . 1 | trimAll "\"" }} {{- /* Container name */ -}}
  {{- $imageDigest := index $context.Values.global.modulesImages.digests "common" $containerName }}
  {{- if not $imageDigest }}
  {{- $error := (printf "Image %s.%s has no digest" "common" $containerName ) }}
  {{- fail $error }}
  {{- end }}
  {{- printf "%s@%s" $context.Values.global.modulesImages.registry.base $imageDigest }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_common_image_no_fail" (list . "<container-name>") }} */ -}}
{{- /* returns image name from common module if found */ -}}
{{- define "helm_lib_module_common_image_no_fail" }}
  {{- $context := index . 0 }} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $containerName := index . 1 | trimAll "\"" }} {{- /* Container name */ -}}
  {{- $imageDigest := index $context.Values.global.modulesImages.digests "common" $containerName }}
  {{- if $imageDigest }}
  {{- printf "%s@%s" $context.Values.global.modulesImages.registry.base $imageDigest }}
  {{- end }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_image_digest" (list . "<container-name>" "<module-name>(optional)") }} */ -}}
{{- /* returns image digest */ -}}
{{- define "helm_lib_module_image_digest" }}
  {{- $context := index . 0 }} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $containerName := index . 1 | trimAll "\"" }} {{- /* Container name */ -}}
  {{- $rawModuleName := $context.Chart.Name }}
  {{- if ge (len .) 3 }}
  {{- $rawModuleName = (index . 2) }} {{- /* Optional module name */ -}}
  {{- end }}
  {{- $moduleName := (include "helm_lib_module_camelcase_name" $rawModuleName) }}
  {{- $moduleMap := index $context.Values.global.modulesImages.digests $moduleName | default dict }}
  {{- $imageDigest := index $moduleMap $containerName | default "" }}
  {{- if not $imageDigest }}
  {{- $error := (printf "Image %s.%s has no digest" $moduleName $containerName ) }}
  {{- fail $error }}
  {{- end }}
  {{- printf "%s" $imageDigest }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_image_digest_no_fail" (list . "<container-name>" "<module-name>(optional)") }} */ -}}
{{- /* returns image digest if found */ -}}
{{- define "helm_lib_module_image_digest_no_fail" }}
  {{- $context := index . 0 }} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $containerName := index . 1 | trimAll "\"" }} {{- /* Container name */ -}}
  {{- $moduleName := (include "helm_lib_module_camelcase_name" $context) }}
  {{- if ge (len .) 3 }}
  {{- $moduleName = (include "helm_lib_module_camelcase_name" (index . 2)) }} {{- /* Optional module name */ -}}
  {{- end }}
  {{- $moduleMap := index $context.Values.global.modulesImages.digests $moduleName | default dict }}
  {{- $imageDigest := index $moduleMap $containerName | default "" }}
  {{- printf "%s" $imageDigest }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_csi_image_with_common_fallback" (list . "<raw-container-name>" "<semver>") }} */ -}}
{{- /* returns image name from storage foundation module if enabled, otherwise from common module */ -}}
{{- define "helm_lib_csi_image_with_common_fallback" }}
  {{- $context := index . 0 }} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $rawContainerName := index . 1 | trimAll "\"" }} {{- /* Container raw name */ -}}
  {{- $kubernetesSemVer := index . 2 }} {{- /* Kubernetes semantic version */ -}}
  {{- $imageDigest := "" }}
  {{- $registryBase := $context.Values.global.modulesImages.registry.base }}
  {{- /* Try to get from storage foundation module if enabled */}}
  {{- if $context.Values.global.enabledModules | has "storage-foundation" }}
    {{- $registryBase = join "/" (list $registryBase "modules" "storage-foundation" ) }}
    {{- $storageFoundationDigests := index $context.Values.global.modulesImages.digests "storageFoundation" | default dict }}
    {{- $currentMinor := int $kubernetesSemVer.Minor }}
    {{- $kubernetesMajor := int $kubernetesSemVer.Major }}
    {{- /* Iterate from currentMinor down to 0: use offset from 0 to currentMinor, then calculate minorVersion = currentMinor - offset */}}
    {{- range $offset := until (int (add $currentMinor 1)) }}
      {{- if not $imageDigest }}
        {{- $minorVersion := int (sub $currentMinor $offset) }}
        {{- $containerName := join "" (list $rawContainerName "ForK8SGE" $kubernetesMajor $minorVersion) }}
        {{- $digest := index $storageFoundationDigests $containerName | default "" }}
        {{- if $digest }}
          {{- $imageDigest = $digest }}
        {{- end }}
      {{- end }}
    {{- end }}
    {{- /* Fallback to base container name if no versioned image found (when minor reached 0) */}}
    {{- if not $imageDigest }}
      {{- $imageDigest = index $storageFoundationDigests $rawContainerName | default "" }}
    {{- end }}
  {{- /* Fallback to common module if storage foundation module is not enabled */}}
  {{- else }}
    {{- $containerName := join "" (list $rawContainerName $kubernetesSemVer.Major $kubernetesSemVer.Minor) }}
    {{- $imageDigest = index $context.Values.global.modulesImages.digests "common" $containerName | default "" }}
  {{- end }}
  {{- if $imageDigest }}
    {{- printf "%s@%s" $registryBase $imageDigest }}
  {{- end }}
{{- end }}
