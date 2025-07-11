---
title: "The neuvector module: usage examples"
---

## Enabling NeuVector

1. Enable the module:

    ```bash
    d8 platform module enable neuvector
    ```

1. Apply the configuration (use your own data in the `name`, `bootstrapPassword`, `host` fields):

    ```yaml
    apiVersion: deckhouse.io/v1alpha1
    kind: ModuleConfig
    metadata:
      name: neuvector
    spec:
      enabled: true
      settings:
        controller:
          ingress:
            enabled: true
            host: neuvector.example.com
        manager:
          ingress:
            enabled: true
            host: neuvector-ui.example.com
    ```

1. Access the NeuVector console at your configured ingress host.
- Navigate to the configured hostname ingress.
- Log in with the username `admin` and your configured password.
- Start configuring security and monitoring policies.

## Configure Vulnerability Scanning

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: neuvector
spec:
  settings:
    scanner:
      enabled: true
      replicas: 2
      resources:
        requests:
          cpu: 500m
          memory: 1Gi
```

## Getting the password

If you need to get the admin password stored in the Kubernetes secret in the d8-neuvector namespace, use the following command:

```txt
kubectl -n d8-neuvector get secret admin -o jsonpath='{.data.password}' | base64 -d
```
