# How to generate module values

```shell
cat /deckhouse/modules/400-descheduler/crds/deschedulers.yaml | \
yq '.spec.versions[] | select(.name == "v1alpha1") | .schema.openAPIV3Schema | del(.. | select(has("default")).default) | .properties.metadata.additionalProperties = true' | \
 tee /deckhouse/modules/400-descheduler/openapi/descheduler_v1alpha1.yaml
 ```
