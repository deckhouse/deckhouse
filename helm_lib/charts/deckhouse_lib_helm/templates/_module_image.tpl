{{- /* Usage: {{ include "helm_lib_module_image" (list . "<container-name>" "<module-name>(optional)") }} */ -}}
{{- /* returns image name */ -}}
{{- define "helm_lib_module_image" }}
  {{- $rawModuleName := include "helm_lib_internal_module_raw_name" . }}
  {{- $imageDigest := include "helm_lib_module_image_digest_no_fail" . }}
  {{- if not $imageDigest }}
    {{- fail (printf "Image %s.%s has no digest" (include "helm_lib_module_camelcase_name" $rawModuleName) (index . 1 | trimAll "\"")) }}
  {{- end }}
  {{- /* the legacy registry override addresses images by the kebab-cased module name */}}
  {{- $registryBase := include "helm_lib_internal_module_registry_base" (list (index . 0) (include "helm_lib_internal_module_own_package" .) $rawModuleName (include "helm_lib_module_kebabcase_name" $rawModuleName)) }}
  {{- if not $registryBase }}
    {{- fail "Registry base is not set" }}
  {{- end }}
  {{- printf "%s@%s" $registryBase $imageDigest }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_image_no_fail" (list . "<container-name>" "<module-name>(optional)") }} */ -}}
{{- /* returns image name if found */ -}}
{{- define "helm_lib_module_image_no_fail" }}
  {{- $imageDigest := include "helm_lib_module_image_digest_no_fail" . }}
  {{- if $imageDigest }}
    {{- /* the legacy registry override addresses images by the chart name */}}
    {{- $registryBase := include "helm_lib_internal_module_registry_base" (list (index . 0) (include "helm_lib_internal_module_own_package" .) (include "helm_lib_internal_module_raw_name" .) (index . 0).Chart.Name) }}
    {{- if $registryBase }}
      {{- printf "%s@%s" $registryBase $imageDigest }}
    {{- end }}
  {{- end }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_image_digest" (list . "<container-name>" "<module-name>(optional)") }} */ -}}
{{- /* returns image digest */ -}}
{{- define "helm_lib_module_image_digest" }}
  {{- $imageDigest := include "helm_lib_module_image_digest_no_fail" . }}
  {{- if not $imageDigest }}
    {{- fail (printf "Image %s.%s has no digest" (include "helm_lib_module_camelcase_name" (include "helm_lib_internal_module_raw_name" .)) (index . 1 | trimAll "\"")) }}
  {{- end }}
  {{- $imageDigest }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_image_digest_no_fail" (list . "<container-name>" "<module-name>(optional)") }} */ -}}
{{- /* returns image digest if found */ -}}
{{- define "helm_lib_module_image_digest_no_fail" }}
  {{- $context := index . 0 }} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $containerName := index . 1 | trimAll "\"" }} {{- /* Container name */ -}}
  {{- if include "helm_lib_internal_module_own_package" . }}
    {{- index ($context.Module.Package.Digests | default dict) $containerName | default "" }}
  {{- else }}
    {{- $moduleName := include "helm_lib_module_camelcase_name" (include "helm_lib_internal_module_raw_name" .) }}
    {{- include "helm_lib_internal_platform_image_digest" (list $context $moduleName $containerName) }}
  {{- end }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_common_image" (list . "<container-name>") }} */ -}}
{{- /* returns image name from common module */ -}}
{{- define "helm_lib_module_common_image" }}
  {{- $image := include "helm_lib_module_common_image_no_fail" . }}
  {{- if not $image }}
    {{- fail (printf "Image common.%s has no digest" (index . 1 | trimAll "\"")) }}
  {{- end }}
  {{- $image }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_common_image_no_fail" (list . "<container-name>") }} */ -}}
{{- /* returns image name from common module if found */ -}}
{{- define "helm_lib_module_common_image_no_fail" }}
  {{- $context := index . 0 }} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $containerName := index . 1 | trimAll "\"" }} {{- /* Container name */ -}}
  {{- $imageDigest := include "helm_lib_internal_platform_image_digest" (list $context "common" $containerName) }}
  {{- $registryBase := include "helm_lib_internal_platform_registry_base" $context }}
  {{- if and $imageDigest $registryBase }}
    {{- printf "%s@%s" $registryBase $imageDigest }}
  {{- end }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_internal_module_registry_base" (list $context "<own-package>" "<module-name>" "<override-path>") }} */ -}}
{{- /* internal: returns the registry path module images live in, or empty when it cannot be resolved */ -}}
{{- define "helm_lib_internal_module_registry_base" }}
  {{- $context := index . 0 }}
  {{- $ownPackage := index . 1 }}
  {{- $rawModuleName := index . 2 }}
  {{- $overridePath := index . 3 | trimAll "/" }}
  {{- $registryBase := "" }}

  {{- if $ownPackage }}
    {{- /* Module package: embedded images live in the platform registry, downloaded ones next to the package */}}
    {{- if $context.Module.Package.Embedded }}
      {{- $registryBase = include "helm_lib_internal_platform_registry_base" $context }}
    {{- else }}
      {{- $repository := $context.Module.Package.Registry.repository }}
      {{- $packageName := $context.Module.Package.Name }}
      {{- if and $repository $packageName }}
        {{- $registryBase = printf "%s/%s" $repository $packageName }}
      {{- end }}
    {{- end }}
  {{- else }}
    {{- /* Legacy module: the registry may be overridden in the module values */}}
    {{- $registryBase = include "helm_lib_internal_platform_registry_base" $context }}
    {{- $moduleValues := index $context.Values (include "helm_lib_module_camelcase_name" $rawModuleName) | default dict }}
    {{- $moduleRegistry := index $moduleValues "registry" | default dict }}
    {{- if index $moduleRegistry "base" }}
      {{- $registryBase = printf "%s/%s" (trimAll "/" (index $moduleRegistry "base")) $overridePath }}
    {{- end }}
  {{- end }}

  {{- $registryBase }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_internal_module_raw_name" (list . "<container-name>" "<module-name>(optional)") }} */ -}}
{{- /* internal: returns the optional module name argument or the chart name */ -}}
{{- define "helm_lib_internal_module_raw_name" }}
  {{- if ge (len .) 3 }}
    {{- index . 2 | trimAll "\"" }}
  {{- else }}
    {{- (index . 0).Chart.Name }}
  {{- end }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_internal_module_own_package" (list . "<container-name>" "<module-name>(optional)") }} */ -}}
{{- /* internal: non-empty when images resolve from the current module's own package */ -}}
{{- define "helm_lib_internal_module_own_package" }}
  {{- $context := index . 0 }}
  {{- /* An explicit module name asks for another module's image, which only the global map holds */}}
  {{- if and (lt (len .) 3) $context.Module $context.Module.Package }}
    {{- true }}
  {{- end }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_internal_platform_image_digest" (list $context "<module-name>" "<container-name>") }} */ -}}
{{- /* internal: returns a digest from the platform image map, or empty when the map does not hold it */ -}}
{{- define "helm_lib_internal_platform_image_digest" }}
  {{- $modulesImages := index ((index . 0).Values.global | default dict) "modulesImages" | default dict }}
  {{- $moduleDigests := index (index $modulesImages "digests" | default dict) (index . 1) | default dict }}
  {{- index $moduleDigests (index . 2) | default "" }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_internal_platform_registry_base" $context }} */ -}}
{{- /* internal: returns the platform registry path, or empty when the platform image map is absent */ -}}
{{- define "helm_lib_internal_platform_registry_base" }}
  {{- $modulesImages := index (.Values.global | default dict) "modulesImages" | default dict }}
  {{- index (index $modulesImages "registry" | default dict) "base" | default "" }}
{{- end }}
