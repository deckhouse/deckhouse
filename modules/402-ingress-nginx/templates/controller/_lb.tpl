{{- /* Usage: include "ingress_nginx_lb_provider_annotations" (dict "ctx" . "loadBalancerIP" $ip) | nindent 4 */ -}}
{{- /* Emit provider-specific annotations for a LoadBalancer service when a predefined IP is set. */ -}}
{{- define "ingress_nginx_lb_cloud_provider_annotations" -}}
{{- $m := . | default dict -}}
{{- $ctx := ($m.ctx | default dict) -}} {{- /* Template context with .Values, .Chart, etc */ -}}
{{- $ip := ($m.loadBalancerIP | default "") -}} {{- /* Predefined IP address */ -}}
{{- if $ip }}
    {{- if ($ctx.Values.global.enabledModules | has "cloud-provider-openstack") }}
    loadbalancer.openstack.org/keep-floatingip: "true"
    loadbalancer.openstack.org/load-balancer-address: {{ $ip | quote }}
    {{- else if ($ctx.Values.global.enabledModules | has "cloud-provider-yandex") }}
    yandex.cpi.flant.com/listener-address-ipv4: {{ $ip | quote }}
    {{- end -}}
{{- end -}}
{{- end -}}
