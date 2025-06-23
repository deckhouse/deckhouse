---
title: "The log-shipper module: logs transformations"
description: Examples of using log transformations
---

{% raw %}

## Transform mixed logs, JSON or strings to JSON. Parse JSON and reduce nesting

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ClusterLoggingDestination
metadata:
  name: string-to-json
spec:
  ...
  transformations:
    - action: EnsureStructuredMessage
      ensureStructuredMessage:
        soureFormat: String
          string:
            targetField: msg
            depth: 1
```

```bash
# LOGS:

/docker-entrypoint.sh: Configuration complete; ready for start up
{"level" : "info","msg" : "fetching.module.release", "releasechannel" : "Stable", "time" : "2025-06-23T08:00:29Z"}

# RESULT TRANSFORMATIONS:

"message": { "msg": "/docker-entrypoint.sh: Configuration complete; ready for start up"}
"message": {"level" : "info","msg" : "fetching.module.release", "releasechannel" : "Stable", "time" : "2025-06-23T08:00:29Z"}

```

## Transform mixed logs, JSON or Klog to JSON. Parse JSON

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ClusterLoggingDestination
metadata:
  name: klog-to-json
spec:
  ...
  transformations:
    - action: EnsureStructuredMessage
      ensureStructuredMessage:
        soureFormat: Klog
```

```bash
# LOGS:

I0505 17:59:40.692994   28133 klog.go:70] hello from klog
{"level" : "info","msg" : "fetching.module.release", "releasechannel" : "Stable", "time" : "2025-06-23T08:00:29Z"}

# RESULT TRANSFORMATIONS:

"message": {"file":"klog.go","id":28133,"level":"info","line":70,"message":"hello from klog","timestamp":"2025-05-05T17:59:40.692994Z"}
"message": {"level" : "info","msg" : "fetching.module.release", "releasechannel" : "Stable", "time" : "2025-06-23T08:00:29Z"}

```

## JSON parsing and nesting reduction

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ClusterLoggingDestination
metadata:
  name: parse-json
spec:
  ...
  transformations:
    - action: EnsureStructuredMessage
      ensureStructuredMessage:
        soureFormat: JSON
          json:
            depth: 1
```

```bash
# LOG:

{"level" : { "severity": "info" },"msg" : "fetching.module.release"}

# RESULT TRANSFORMATIONS:

"message": {"level" : "{ \"severity\": \"info\" }","msg" : "fetching.module.release"}

```

## Replacing dots with underscores in label keys

- When applying a transform to labels in message, you must first perform an esureStructuredMessage transform to parse the json

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ClusterLoggingDestination
metadata:
  name: replace-dot
spec:
  ...
  transformations:
    - action: ReplaceDotKeys
      replaceDotKeys:
        labels:
          - pod_labels
```

```bash
# LOG:

{"msg" : "fetching.module.release"} # pod label pod.app=test

# RESULT TRANSFORMATIONS:

{"message": {"msg" : "fetching.module.release"}, pod_labels: {"pod_app": "test"}}

```

## Removing labels

- When applying a transform to labels in message, you must first perform an esureStructuredMessage transform to parse the json

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ClusterLoggingDestination
metadata:
  name: drop-label
spec:
  ...
  transformations:
    - action: DropLabels
      dropLabels:
        labels:
          - example
```

## Example of removing a label from a message

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ClusterLoggingDestination
metadata:
  name: drop-label
spec:
  ...
  transformations:
    - action: EnsureStructuredMessage
      ensureStructuredMessage:
        soureFormat: JSON
          json:
            depth: 2
    - action: DropLabels
      dropLabels:
        labels:
          - message.example
```

```bash
# LOG:

{"msg" : "fetching.module.release", "example": "test"}

# RESULT TRANSFORMATIONS:

"message": {"msg" : "fetching.module.release"}

```

{% endraw %}