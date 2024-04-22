# # Module obj names
# {{- define "docker-auth-name" -}}
# "docker-auth"
# {{- end }}

# {{- define "docker-distribution-name" -}}
# "docker-distribution"
# {{- end }}

# {{- define "seaweedfs-name" -}}
# "seaweedfs"
# {{- end }}

# {{- define "namespace-name"  -}}
# {{- printf "d8-%s" (include "system-registry" .) }}
# {{- end }}


# # Module obj labels
# {{- define "docker-auth-labels" -}}
# {{- /* Input: list[context, map[label_name]label_value] */ }}
# {{- $context := index . 0 }}
# {{- $labels_dict := (dict "app" (include "docker-auth-name" .)) }}
# {{- if eq (len .) 2 }}
#     {{- $extra_labels := index . 1 }}
#     {{- range $label_key, $label_value := $extra_labels }}
#         {{- $_ := set $labels_dict $label_key $label_value }}
#     {{- end }}
# {{- end }}
# {{- include "helm_lib_module_labels" (list $context $labels_dict) }}
# {{- end }}

# {{- define "docker-distribution-labels" -}}
# {{- /* Input: list[context, map[label_name]label_value] */ }}
# {{- $context := index . 0 }}
# {{- $labels_dict := (dict "app" (include "docker-distribution-name" .)) }}
# {{- if eq (len .) 2 }}
#     {{- $extra_labels := index . 1 }}
#     {{- range $label_key, $label_value := $extra_labels }}
#         {{- $_ := set $labels_dict $label_key $label_value }}
#     {{- end }}
# {{- end }}
# {{- include "helm_lib_module_labels" (list $context $labels_dict) }}
# {{- end }}

# {{- define "seaweedfs-labels" -}}
# {{- /* Input: list[context, map[label_name]label_value] */ }}
# {{- $context := index . 0 }}
# {{- $labels_dict := (dict "app" (include "seaweedfs-name" .)) }}
# {{- if eq (len .) 2 }}
#     {{- $extra_labels := index . 1 }}
#     {{- range $label_key, $label_value := $extra_labels }}
#         {{- $_ := set $labels_dict $label_key $label_value }}
#     {{- end }}
# {{- end }}
# {{- include "helm_lib_module_labels" (list $context $labels_dict) }}
# {{- end }}
