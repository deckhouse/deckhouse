- name: kubernetes.ingress-nginx.deprecated-geoip
  rules:
  - alert: DeprecatedGeoIPVersion
    expr: >-
      count(d8_deprecated_geoip_version) > 0
    labels:
      severity_level: "9"
    annotations:
      summary: Deprecated GeoIP version 1 is used in the cluster.
      description: |-
        An IngressNginxController and/or Ingress object in the cluster is using variables from the deprecated NGINX GeoIPv1 module. Support for this module has been discontinued in Ingress NGINX Controller version 1.10 and higher.

        It's recommended that you update your configuration to use the [GeoIPv2 module]({{ include "helm_lib_module_documentation_uri" (list . "/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-geoip2") }}).

        To get a list of the IngressNginxControllers using GeoIPv1 variables, run the following command:

        ```shell
        d8 k get ingressnginxcontrollers.deckhouse.io -o json | jq '.items[] | select(..|strings | test("\\$geoip_(country_(code3|code|name)|area_code|city_continent_code|city_country_(code3|code|name)|dma_code|latitude|longitude|region|region_name|city|postal_code|org)([^_a-zA-Z0-9]|$)+")) | .metadata.name'
        ```

        To get a list of the Ingress objects using GeoIPv1 variables, run the following command:

        ```shell
        d8 k get ingress -A -o json | jq '.items[] | select(..|strings | test("\\$geoip_(country_(code3|code|name)|area_code|city_continent_code|city_country_(code3|code|name)|dma_code|latitude|longitude|region|region_name|city|postal_code|org)([^_a-zA-Z0-9]|$)+")) | "\(.metadata.namespace)/\(.metadata.name)"' | sort | uniq
        ```
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_extended_monitoring_deprecated_annotation: "DeprecatedGeoIPVersion,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_extended_monitoring_deprecated_annotation: "DeprecatedGeoIPVersion,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
