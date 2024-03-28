{{- define "bgpadvertisement_template" }}
  {{- $context :=  index . 0 }}
  {{- $ipaddressPool :=  index . 1 }}
  {{- $key :=  index . 2 }}
  {{- $bgpAdvertisement :=  index . 3 }}
  {{- $communities := list }}
  {{- if index $bgpAdvertisement "communities" }}
    {{- range $c := $bgpAdvertisement.communities }}
      {{- if index $context.Values.metallb.bgpCommunities $c }}
        {{- $communities = append $communities (index $context.Values.metallb.bgpCommunities $c) }}
      {{- end }}
    {{- end }}
  {{- end }}
---
apiVersion: metallb.io/v1beta1
kind: BGPAdvertisement
metadata:
  name: {{ $ipaddressPool.name }}-{{ $key }}
  namespace: d8-{{ $context.Chart.Name }}
  {{- include "helm_lib_module_labels" (list $context (dict "app" "speaker")) | nindent 2 }}
spec:
  {{- if index $bgpAdvertisement "aggregation-length" }}
  aggregationLength: {{ index $bgpAdvertisement "aggregation-length" }}
  {{- end }}
  {{- if index $bgpAdvertisement "localpref" }}
  localPref: {{ index $bgpAdvertisement "localpref" }}
  {{- end }}
  {{- if gt (len $communities) 0 }}
  communities:
    {{- range $v := $communities }}
  - {{ $v }}
    {{- end }}
  {{- end }}
  ipAddressPools:
  - {{ $ipaddressPool.name }}
{{- end }}
