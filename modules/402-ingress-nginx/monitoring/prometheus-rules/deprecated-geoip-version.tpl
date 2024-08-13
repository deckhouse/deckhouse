- name: kubernetes.ingress-nginx.deprecated-geoip
  rules:
  - alert: DeprecatedGeoIPVersion
    expr: >-
      count(d8_deprecated_geoip_version) > 0
    labels:
      severity_level: "9"
    annotations:
      description: |-
        There is an IngressNginxController and/or an Ingress object that utilize(s) Nginx GeoIPv1 module's variables. The module is deprecated and its support is discontinued from Ingess Nginx Controller of version 1.10 and higher. It's recommend to upgrade your configuration to use [GeoIPv2 module]({{ include "helm_lib_module_documentation_uri" (list . "/modules/402-ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-geoip2") }}).
        Use the following command to get the list of the IngressNginxControllers that contain GeoIPv1 variables:
        `kubectl  get ingressnginxcontrollers.deckhouse.io -o json | jq '.items[] | select(..|strings | test("\\$geoip_(country_(code3|code|name)|area_code|city_continent_code|city_country_(code3|code|name)|dma_code|latitude|longitude|region|region_name|city|postal_code|org)([^_a-zA-Z0-9]|$)+")) | .metadata.name'`

        Use the following command to get the list of the Ingress objects that contain GeoIPv1 variables:
        `kubectl  get ingress -A -o json | jq '.items[] | select(..|strings | test("\\$geoip_(country_(code3|code|name)|area_code|city_continent_code|city_country_(code3|code|name)|dma_code|latitude|longitude|region|region_name|city|postal_code|org)([^_a-zA-Z0-9]|$)+")) | "\(.metadata.namespace)/\(.metadata.name)"' | sort | uniq`
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_extended_monitoring_deprecated_annotation: "DeprecatedGeoIPVersion,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_extended_monitoring_deprecated_annotation: "DeprecatedGeoIPVersion,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      summary: Deprecated GeoIP version 1 is being used in the cluster.
