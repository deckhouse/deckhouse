{{- /* fencing_profile_checksum is the checksum of a built-in profile, passed as its name. The */}}
{{- /* annotation of the FencingSLAProfile and the ConfigMap of the agents render the same */}}
{{- /* include, so the agent compares the two values byte by byte. */}}
{{- define "fencing_profile_checksum" -}}
{{ include (printf "fencing_profile_spec_%s" .) "" | sha256sum }}
{{- end }}
