{{- /*
  Returns one field of a credential Secret collected into internal.credentialSecrets, or an
  empty string when the Secret is absent or does not carry the field.

  Usage: {{ include "yandex_credential_secret" (list . "d8-credentials" "secret") }}
    0 — template context with .Values
    1 — Secret name, the key under internal.credentialSecrets
    2 — field to read: "secret" or "identity"

  The lookup has to tolerate an empty credentialSecrets map: the Secret is created by the
  operator (or projected from the legacy YandexClusterConfiguration by
  hooks/yandex_cluster_configuration.go), so it is missing while a cluster is being
  bootstrapped, and the workloads still have to render.
*/ -}}
{{- define "yandex_credential_secret" -}}
{{- $context := index . 0 -}}
{{- $secretName := index . 1 -}}
{{- $field := index . 2 -}}
{{- $credSecret := index $context.Values.cloudProviderYandex.internal.credentialSecrets $secretName -}}
{{- if $credSecret -}}
{{- dig $field "" $credSecret -}}
{{- end -}}
{{- end -}}

{{- /*
  The five helpers below resolve the network facts the workloads need. Each one repeats a rule
  that candi/ already implements in HCL, and the pairing is deliberate — the two sides have to
  agree, or the module and the infrastructure disagree about which network the cluster runs in.

  The rule is always the same: a value the operator stated in ModuleConfig
  (nodes.parameters.existing*) wins, and internal.providerDiscoveryData is the fallback for
  whatever the infrastructure run created itself. That mirrors candi, where
  `existing_network_id`/`existing_route_table_id`/`existing_zone_to_subnet_id_map` short-circuit
  the resources terraform would otherwise create:

    layouts/<layout>/base-infrastructure/main.tf   network_id = existing_network_id != "" ? ... : created
    terraform-modules/vpc-components/main.tf  route_table_id = existing_route_table_id == "" ? created : existing

  A cluster whose infrastructure DKP does not create (a static cluster that adds ephemeral nodes
  in Yandex Cloud) has no discovery data at all, so the ModuleConfig side is the only source
  there; a cluster DKP does create records the same values in discovery data, so both sides
  agree and the priority never actually fires.
*/ -}}

{{- /*
  The VPC network the load balancer target group is registered in. candi reports this as the
  cluster's own network, so the operator's stated nodes.parameters.existingNetworkID wins and
  internal.providerDiscoveryData.defaultLbTargetGroupNetworkId is the fallback.
  Usage: {{ include "yandex_default_lb_target_group_network_id" . }}
*/ -}}
{{- define "yandex_default_lb_target_group_network_id" -}}
{{- $existing := dig "nodes" "parameters" "existingNetworkID" "" .Values.cloudProviderYandex -}}
{{- if $existing -}}
{{- $existing -}}
{{- else -}}
{{- dig "internal" "providerDiscoveryData" "defaultLbTargetGroupNetworkId" "" .Values.cloudProviderYandex -}}
{{- end -}}
{{- end -}}

{{- /*
  The internal networks, as a JSON array. candi emits `[network_id]`, so a stated
  existingNetworkID yields a one-element list; discovery data keeps whatever it recorded.
  ccm.parameters.additionalInternalNetworkIDs is appended on top and the result is
  deduplicated, so a cluster DKP did not build (e.g. hybrid) can list more than the single
  existing network.
  Usage: {{ include "yandex_internal_network_ids" . | fromJsonArray }}
*/ -}}
{{- define "yandex_internal_network_ids" -}}
{{- $existing := dig "nodes" "parameters" "existingNetworkID" "" .Values.cloudProviderYandex -}}
{{- $base := list -}}
{{- if $existing -}}
{{- $base = list $existing -}}
{{- else -}}
{{- $base = dig "internal" "providerDiscoveryData" "internalNetworkIDs" (list) .Values.cloudProviderYandex -}}
{{- end -}}
{{- $additional := dig "ccm" "parameters" "additionalInternalNetworkIDs" (list) .Values.cloudProviderYandex -}}
{{- concat $base $additional | uniq | toJson -}}
{{- end -}}

{{- /*
  Zone -> subnet mapping, as a JSON object.
  Usage: {{ include "yandex_zone_to_subnet_id_map" . | fromJson }}
*/ -}}
{{- define "yandex_zone_to_subnet_id_map" -}}
{{- $existing := dig "nodes" "parameters" "existingZoneToSubnetIDMap" (dict) .Values.cloudProviderYandex -}}
{{- if $existing -}}
{{- $existing | toJson -}}
{{- else -}}
{{- dig "internal" "providerDiscoveryData" "zoneToSubnetIdMap" (dict) .Values.cloudProviderYandex | toJson -}}
{{- end -}}
{{- end -}}

{{- /*
  The route table the subnets are attached to. Empty is a legitimate answer only for a cluster
  that has neither stated one nor run its infrastructure; the CCM template rejects it there.
  Usage: {{ include "yandex_route_table_id" . }}
*/ -}}
{{- define "yandex_route_table_id" -}}
{{- $existing := dig "nodes" "parameters" "existingRouteTableID" "" .Values.cloudProviderYandex -}}
{{- if $existing -}}
{{- $existing -}}
{{- else -}}
{{- dig "internal" "providerDiscoveryData" "routeTableID" "" .Values.cloudProviderYandex -}}
{{- end -}}
{{- end -}}

{{- /*
  The zones the cluster works with, as a sorted JSON array. This repeats the `zones` output of
  candi/layouts/<layout>/base-infrastructure/outputs.tf verbatim:

    zones = length(local.zones) > 0
      ? tolist(setintersection(keys(zone_to_subnet_id_map), local.zones))
      : keys(zone_to_subnet_id_map)

  so a globally restricted set of zones narrows the subnets the cluster covers, and an absent
  (or empty) restriction means every zone those subnets cover. `sortAlpha` reproduces the order
  terraform's set-to-list conversion produces; node-manager derives its default zone set from
  this list, so the order has to be stable across renders.
  Usage: {{ include "yandex_zones" . | fromJsonArray }}
*/ -}}
{{- define "yandex_zones" -}}
{{- $covered := keys (include "yandex_zone_to_subnet_id_map" . | fromJson) | sortAlpha -}}
{{- $restricted := dig "nodes" "parameters" "zones" (list) .Values.cloudProviderYandex -}}
{{- if $restricted -}}
{{- $intersection := list -}}
{{- range $zone := $covered -}}
{{- if has $zone $restricted -}}
{{- $intersection = append $intersection $zone -}}
{{- end -}}
{{- end -}}
{{- $intersection | toJson -}}
{{- else -}}
{{- $covered | toJson -}}
{{- end -}}
{{- end -}}

{{- /*
  Whether nodes get a public IP, as "true"/"false". candi decides this per layout — only
  `without-nat` sets it (layouts/<layout>/base-infrastructure/outputs.tf) — so the layout in
  ModuleConfig is the whole answer. Discovery data stays as the fallback for the one case
  where the layout has not reached values yet.
  Usage: {{ eq (include "yandex_should_assign_public_ip_address" .) "true" }}
*/ -}}
{{- define "yandex_should_assign_public_ip_address" -}}
{{- $layout := dig "nodes" "parameters" "layout" "" .Values.cloudProviderYandex -}}
{{- if $layout -}}
{{- eq $layout "WithoutNAT" -}}
{{- else -}}
{{- dig "internal" "providerDiscoveryData" "shouldAssignPublicIPAddress" false .Values.cloudProviderYandex -}}
{{- end -}}
{{- end -}}
