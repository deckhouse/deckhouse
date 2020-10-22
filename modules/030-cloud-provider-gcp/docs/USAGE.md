---
title: "Сloud provider — GCP: примеры конфигурации"
---

## Пример конфигурации модуля

```yaml
cloudProviderGcpEnabled: "true"
cloudProviderGcp: |
  networkName: default
  subnetworkName: kube
  region: europe-north1
  zones:
  - europe-north1-a
  - europe-north1-b
  - europe-north1-c
  extraInstanceTags:
  - kube
  disableExternalIP: false
  sshKey: "ssh-rsa testetestest"
  serviceAccountKey: |
    {
      "type": "service_account",
      "project_id": "test",
      "private_key_id": "easfsadfdsafdsafdsaf",
      "private_key": "-----BEGIN PRIVATE KEY-----\ntesttesttesttest\n-----END PRIVATE KEY-----\n",
      "client_email": "test@test-sandbox.iam.gserviceaccount.com",
      "client_id": "1421324321314131243214",
      "auth_uri": "https://accounts.google.com/o/oauth2/auth",
      "token_uri": "https://oauth2.googleapis.com/token",
      "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
      "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/test%test-sandbox.iam.gserviceaccount.com"
    }
```


## Пример CR `GCPInstanceClass`

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: GCPInstanceClass
metadata:
  name: test
spec:
  machineType: n1-standard-1
```
