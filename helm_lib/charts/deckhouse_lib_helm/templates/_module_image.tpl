{{- /* Usage: {{ include "helm_lib_module_image" (list . "<container-name>" "<module-name>(optional)") }} */ -}}
{{- /* returns image name */ -}}
{{- define "helm_lib_module_image" }}
  {{- $context := index . 0 }} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $containerName := index . 1 | trimAll "\"" }} {{- /* Container name */ -}}

  {{- /* New approach: the module ships its own images in a package */}}
  {{- if include "helm_lib_internal_module_own_package" . }}
    {{- $registryBase := include "helm_lib_internal_module_package_registry_base" $context }}
    {{- if not $registryBase }}
      {{- if not $context.Module.Package.Registry.repository }}
        {{- fail "Registry base is not set" }}
      {{- end }}
      {{- fail "Package name is not set" }}
    {{- end }}

    {{- $imageDigest := index ($context.Module.Package.Digests | default dict) $containerName }}
    {{- if not $imageDigest }}
      {{- fail (printf "Image %s has no digest" $containerName) }}
    {{- end }}

    {{- printf "%s@%s" $registryBase $imageDigest }}

  {{- /* Legacy fallback: use global modulesImages values */}}
  {{- else }}
    {{- $rawModuleName := include "helm_lib_internal_module_raw_name" . }}
    {{- $moduleName := include "helm_lib_module_camelcase_name" $rawModuleName }}

    {{- $imageDigest := index $context.Values.global.modulesImages.digests $moduleName $containerName }}
    {{- if not $imageDigest }}
      {{- fail (printf "Image %s.%s has no digest" $moduleName $containerName) }}
    {{- end }}

    {{- /* here the external module registry override addresses images by the kebab-cased module name */}}
    {{- $registryPath := include "helm_lib_module_kebabcase_name" $rawModuleName }}
    {{- printf "%s@%s" (include "helm_lib_internal_module_registry_base" (list $context $moduleName $registryPath)) $imageDigest }}
  {{- end }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_image_no_fail" (list . "<container-name>" "<module-name>(optional)") }} */ -}}
{{- /* returns image name if found */ -}}
{{- define "helm_lib_module_image_no_fail" }}
  {{- $context := index . 0 }} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $containerName := index . 1 | trimAll "\"" }} {{- /* Container name */ -}}

  {{- /* New approach: the module ships its own images in a package */}}
  {{- if include "helm_lib_internal_module_own_package" . }}
    {{- $registryBase := include "helm_lib_internal_module_package_registry_base" $context }}
    {{- $imageDigest := include "helm_lib_module_image_digest_no_fail" . }}
    {{- if and $registryBase $imageDigest }}
      {{- printf "%s@%s" $registryBase $imageDigest }}
    {{- end }}

  {{- /* Legacy fallback: use global modulesImages values */}}
  {{- else }}
    {{- $moduleName := include "helm_lib_module_camelcase_name" (include "helm_lib_internal_module_raw_name" .) }}
    {{- $imageDigest := index $context.Values.global.modulesImages.digests $moduleName $containerName }}
    {{- if $imageDigest }}
      {{- /* here the external module registry override addresses images by the chart name */}}
      {{- printf "%s@%s" (include "helm_lib_internal_module_registry_base" (list $context $moduleName $context.Chart.Name)) $imageDigest }}
    {{- end }}
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
  {{- $imageDigest := include "helm_lib_module_image_digest_no_fail" . }}
  {{- if not $imageDigest }}
  {{- $moduleName := include "helm_lib_module_camelcase_name" (include "helm_lib_internal_module_raw_name" .) }}
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

  {{- /* New approach: the module ships its own images in a package */}}
  {{- if include "helm_lib_internal_module_own_package" . }}
    {{- index ($context.Module.Package.Digests | default dict) $containerName | default "" }}

  {{- /* Legacy fallback: use global modulesImages values */}}
  {{- else }}
    {{- $moduleName := include "helm_lib_module_camelcase_name" (include "helm_lib_internal_module_raw_name" .) }}
    {{- $moduleMap := index $context.Values.global.modulesImages.digests $moduleName | default dict }}
    {{- index $moduleMap $containerName | default "" }}
  {{- end }}
{{- end }}

{{- /* Decide whether the images resolve from the module's own package. */ -}}
{{- /* Returns a non-empty string when the context carries a package and no other module was named. */ -}}
{{- define "helm_lib_internal_module_own_package" }}
  {{- $context := index . 0 }} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- /* An explicit module name asks for another module's image, which only the global map holds */}}
  {{- if and (lt (len .) 3) $context.Module $context.Module.Package }}
    {{- true }}
  {{- end }}
{{- end }}

{{- /* Resolve the registry path of the images shipped in the module's own package. */ -}}
{{- /* Returns the platform registry base for an embedded package, the package path otherwise, empty when unresolvable. */ -}}
{{- define "helm_lib_internal_module_package_registry_base" }}
  {{- /* Embedded packages are built into the platform image set, so their images are addressed by digest alone */}}
  {{- if .Module.Package.Embedded }}
    {{- .Values.global.modulesImages.registry.base }}
  {{- else }}
    {{- $repository := .Module.Package.Registry.repository }}
    {{- $packageName := .Module.Package.Name }}
    {{- if and $repository $packageName }}
      {{- printf "%s/%s" $repository $packageName }}
    {{- end }}
  {{- end }}
{{- end }}

{{- /* Resolve the module name passed to an image helper. */ -}}
{{- /* Returns the optional third argument, or the chart name when it is omitted. */ -}}
{{- define "helm_lib_internal_module_raw_name" }}
  {{- if ge (len .) 3 }}
    {{- index . 2 }} {{- /* Optional module name */ -}}
  {{- else }}
    {{- (index . 0).Chart.Name }}
  {{- end }}
{{- end }}

{{- /* Resolve the registry path the legacy module images live in. */ -}}
{{- /* Returns the platform registry base, or the external module override when the module values set one. */ -}}
{{- define "helm_lib_internal_module_registry_base" }}
  {{- $context := index . 0 }} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $moduleName := index . 1 }} {{- /* Camelcased module name, the key the module values live under */ -}}
  {{- $registryPath := index . 2 }} {{- /* Path appended to the override host */ -}}

  {{- $registryBase := $context.Values.global.modulesImages.registry.base }}
{{- /*  handle external modules registry */}}
  {{- if index $context.Values $moduleName }}
    {{- if index $context.Values $moduleName "registry" }}
      {{- if index $context.Values $moduleName "registry" "base" }}
        {{- $host := trimAll "/" (index $context.Values $moduleName "registry" "base") }}
        {{- $path := trimAll "/" $registryPath }}
        {{- $registryBase = join "/" (list $host $path) }}
      {{- end }}
    {{- end }}
  {{- end }}
{{- /* end of external module handling block */}}
  {{- $registryBase }}
{{- end }}
