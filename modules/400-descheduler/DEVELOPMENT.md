# How to generate

```shell
cd /deckhouse/modules/400-descheduler/hooks/internal/api/v1alpha1 && \
go generate .
```

```shell
cat /deckhouse/modules/400-descheduler/crds/deckhouse.io_deschedulers.yaml | yq '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.deschedulerPolicy.properties.strategies.default.removePodsViolatingInterPodAntiAffinity = {} | .spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.deschedulerPolicy.properties.strategies.default.removePodsViolatingNodeAffinity = {} | del(.. | select(has("nodeSelector")).nodeSelector.description) | del(.. | select(has("tolerations")).tolerations | .. | select(has("description")).description)' > \/deckhouse/modules/400-descheduler/crds/deschedulers.yaml
```

```shell
rm /deckhouse/modules/400-descheduler/crds/deckhouse.io_deschedulers.yaml
```

```shell
cat /deckhouse/modules/400-descheduler/crds/deschedulers.yaml | \
yq '.spec.versions[] | select(.name == "v1alpha1") | .schema.openAPIV3Schema | del(.. | select(has("default")).default) | .properties.metadata.additionalProperties = true' | \
 tee /deckhouse/modules/400-descheduler/openapi/descheduler_v1alpha1.yaml
 ```
