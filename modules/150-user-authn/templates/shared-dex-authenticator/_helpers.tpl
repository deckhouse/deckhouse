{{- /* The shared dex-authenticator: two Deployments of one image in the namespace of the module, each serving the
       DexAuthenticator objects of many namespaces as the clients of a table (patch 018 of the image). The system
       instance is for the objects of the d8-* namespaces, the other one for all the rest, so that the sign-in to the
       platform does not share a process with the applications of users. */ -}}

{{- /* The names of the instances. Usage: range (include "shared_dex_authenticator_instances" . | fromYamlArray) */ -}}
{{- define "shared_dex_authenticator_instances" -}}
- dex-authenticator-system
- dex-authenticator
{{- end -}}

{{- /* The id of the dex client of a DexAuthenticator: "@" joins name and namespace, which can both */ -}}
{{- /* contain a dash but not "@". user-authn-controller makes the id the same way. */ -}}
{{- /* Usage: include "dex_authenticator_client_id" (dict "name" $name "namespace" $namespace) */ -}}
{{- define "dex_authenticator_client_id" -}}
{{ printf "dex-authenticator-%s@%s" .name .namespace }}
{{- end -}}

{{- /* The labels of the pods of an instance and of what selects them. Usage: include "..." $instance */ -}}
{{- define "shared_dex_authenticator_pod_labels" -}}
app: shared-dex-authenticator
instance: {{ . }}
{{- end -}}

{{- define "shared_dex_authenticator_resources" }}
cpu: 100m
memory: 128Mi
{{- end }}

{{- define "shared_dex_authenticator_redis_resources" }}
cpu: 20m
memory: 64Mi
{{- end }}

{{- /* "true" when the cluster has HTTPRoute objects: dexclient-controller watches and removes the ones it made
       then. Whether it makes new ones is the setting below, so that turning the Gateway API off in the settings
       removes the routes instead of leaving them behind. */ -}}
{{- define "shared_dex_authenticator_http_routes" -}}
  {{- if include "helm_lib_kind_exists" (list . "HTTPRoute") -}}
true
  {{- end -}}
{{- end -}}

{{- /* "true" when the cluster serves ReferenceGrant as gateway.networking.k8s.io/v1, the version the controller writes,
       and has the admission policy kinds: dexclient-controller keeps a ReferenceGrant in d8-user-authn from the
       HTTPRoute objects of every namespace with a served DexAuthenticator to the Service of its instance. alb resolves the ExternalName Service of a DexAuthenticator in the auth-url of a
       route only with such a grant. The policy dexclient-controller.user-authn.deckhouse.io holds what it writes. */ -}}
{{- define "shared_dex_authenticator_reference_grants" -}}
  {{- if and (include "helm_lib_api_version_exists" (list . "gateway.networking.k8s.io/v1/ReferenceGrant")) (include "helm_lib_kind_exists" (list . "ValidatingAdmissionPolicy")) (include "helm_lib_kind_exists" (list . "ValidatingAdmissionPolicyBinding")) -}}
true
  {{- end -}}
{{- end -}}

{{- /* "true" when the module publishes through the Gateway API and the cluster has HTTPRoute objects: the routes
       of DexAuthenticator objects that name a ListenerSet are HTTPRoute objects then. */ -}}
{{- define "shared_dex_authenticator_gateway_api" -}}
  {{- if and (eq (include "user_authn_gateway_api_enabled" .) "true") (include "shared_dex_authenticator_http_routes" .) -}}
true
  {{- end -}}
{{- end -}}
