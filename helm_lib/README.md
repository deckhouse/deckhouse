Helm utils template definitions for Deckhouse modules.

# Environment variables

## helm_lib_envs_for_proxy
Add HTTP_PROXY, HTTPS_PROXY and NO_PROXY environment variables for container 
depends on [proxy settings](https://deckhouse.io/documentation/v1/deckhouse-configure-global.html#parameters-modules-proxy).

### Arguments
- Dot object (.) with .Values, .Chart, etc

### Example
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
  namespace: d8-some-ns
spec:
  template:
    spec:
      containers:
      - name: app
        args: []
        env:
          {{- include "helm_lib_envs_for_proxy" . | nindent 10 }}
...
```

# High availability - utils definitions

Here and next "cluster is high available" means that cluster has 2 and more control-plane nodes.
Here and next "HA mode enabled by config" means that high available mode enabled for module 
([for example for prometheus module](https://deckhouse.io/documentation/v1/modules/300-prometheus/configuration.html#parameters-highavailability))
or enable by [global configuration](https://deckhouse.io/documentation/v1/deckhouse-configure-global.html#parameters-highavailability).

## helm_lib_is_ha_to_value
Returns value **_Yes_** if cluster highly available or HA mode enabled by config, else — returns **_No_**

### Arguments
- Arguments list
  - Dot object (.) with .Values, .Chart, etc
  - **_Yes_** value
  - **_No_** value

### Example
In the next example if cluster is high available or HA mode enabled by config deployment will have 2 replicas 
else will have 1 replica
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
  namespace: d8-some-ns
spec:
  replicas: {{ include "helm_lib_is_ha_to_value" (list . 2 1) }}
  template:
    spec:
      containers:
      - name: app
        args: []
```

## helm_lib_ha_enabled
Returns not empty string if cluster highly available or HA mode enabled by config, else returns empty string.
Usually this method using in conditions

### Arguments
- Dot object (.) with .Values, .Chart, etc

### Example
In the next example if cluster is high available or HA mode enabled by config deployment can one unavailable replica when rolling update.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
  namespace: d8-some-ns
spec:
  revisionHistoryLimit: 2
  strategy:
    type: RollingUpdate
    {{- if (include "helm_lib_ha_enabled" .) }}
    rollingUpdate:
      maxSurge: 0
      maxUnavailable: 1
    {{- end }}
...
```

# High availability - render part of specs
