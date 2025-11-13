---
title: Integration with Yandex Cloud services
permalink: en/admin/integrations/public/yandex/services.html
---

Deckhouse Kubernetes Platform (DKP) supports native integration with several Yandex Cloud services.
This allows for external monitoring, secure secret management, and automated data synchronization between the infrastructure and the cluster.

This section describes steps to configure the following integrations:

- Yandex Lockbox via External Secrets Operator
- Yandex Managed Service for Prometheus

## Integration with Yandex Lockbox

DKP supports integration with Yandex Lockbox using the External Secrets Operator (ESO).
This allows Kubernetes secrets in the cluster to be automatically synchronized with Lockbox secrets.

To set up the integration:

1. Create a [service account](https://yandex.cloud/en/docs/iam/operations/sa/create) for External Secrets Operator:

   ```shell
   yc iam service-account create --name eso-service-account
   ```

1. Create an [authorized key](https://yandex.cloud/en/docs/iam/operations/authentication/manage-authorized-keys) for the service account and save it to a file:

   ```shell
   yc iam key create --service-account-name eso-service-account --output authorized-key.json
   ```

1. [Assign](https://yandex.cloud/en/docs/iam/operations/sa/assign-role-for-sa) `lockbox.editor`, `lockbox.payloadViewer`, and `kms.keys.encrypterDecrypter` [roles](https://yandex.cloud/en/docs/lockbox/security/#service-roles) to the service account for accessing all secrets in the folder:

   ```shell
   folder_id=<folderID>
   yc resource-manager folder add-access-binding --id=${folder_id} --service-account-name eso-service-account --role lockbox.editor
   yc resource-manager folder add-access-binding --id=${folder_id} --service-account-name eso-service-account --role lockbox.payloadViewer
   yc resource-manager folder add-access-binding --id=${folder_id} --service-account-name eso-service-account --role kms.keys.encrypterDecrypter
   ```

   For fine-grained access control, refer to the [Yandex Lockbox access documentation](https://yandex.cloud/en/docs/lockbox/security/).

1. Install External Secrets Operator using the Helm chart as described in [this guide](https://yandex.cloud/en/docs/managed-kubernetes/operations/applications/external-secrets-operator#helm-install).

   Note that you may need to specify additional parameters such as `nodeSelector`, `tolerations`, etc.
   To do this, use the `./external-secrets/values.yaml` file after extracting the Helm chart.

   Download and extract the chart using the following command:

   ```shell
   helm pull oci://cr.yandex/yc-marketplace/yandex-cloud/external-secrets/chart/external-secrets \
     --version 0.5.5 \
     --untar
   ```

   Install the chart:

   ```shell
   helm install -n external-secrets --create-namespace \
     --set-file auth.json=authorized-key.json \
     external-secrets ./external-secrets/
   ```

   Where:

   - `authorized-key.json`: The file created in step 2.

1. Create a [SecretStore](https://external-secrets.io/latest/api/secretstore/) resource containing the `sa-creds` secret:

   ```console
   kubectl -n external-secrets apply -f - <<EOF
   
   apiVersion: external-secrets.io/v1alpha1
   kind: SecretStore
   metadata:
     name: secret-store
   spec:
     provider:
       yandexlockbox:
         auth:
           authorizedKeySecretRef:
             name: sa-creds
             key: key'
   EOF
   ```

   Where:

   - `sa-creds`: Name of the Secret containing the authorized key. The secret appears after the Helm chart is installed.
   - `key`: Name of the key in the `.data` field of the secret.

1. Verify that External Secrets Operator is working:

   ```shell
   kubectl -n external-secrets get po
   ```

   Example output:

   ```console
   NAME                                                READY   STATUS    RESTARTS   AGE
   external-secrets-55f78c44cf-dbf6q                   1/1     Running   0          77m
   external-secrets-cert-controller-78cbc7d9c8-rszhx   1/1     Running   0          77m
   external-secrets-webhook-6d7b66758-s7v9c            1/1     Running   0          77m
   ```

   Check the SecretStore status as well:

   ```shell
   kubectl -n external-secrets get secretstores.external-secrets.io 
   ```

   Example output:

   ```console
   NAME           AGE   STATUS
   secret-store   69m   Valid
   ```

1. Create a [secret](https://yandex.cloud/en/docs/lockbox/operations/secret-create) in Yandex Lockbox with the following parameters:

   - Name: `lockbox-secret`.
   - Key: Enter non-confidential ID `password`.
   - Value: Enter confidential data for storing `p@$$w0rd`.

1. Create an [ExternalSecret](https://external-secrets.io/latest/api/externalsecret/) pointing to the `lockbox-secret` in the `secret-store`:

   ```console
   kubectl -n external-secrets apply -f - <<EOF

   apiVersion: external-secrets.io/v1alpha1
   kind: ExternalSecret
   metadata:
     name: external-secret
   spec:
     refreshInterval: 1h
     secretStoreRef:
       name: secret-store
       kind: SecretStore
     target:
       name: k8s-secret
     data:
     - secretKey: password
       remoteRef:
         key: <SECRET_ID>
         property: password
   EOF
   ```

   Where:

   - `target.name`: Name of the new secret.
     External Secret Operator will create this secret in the DKP cluster
     and put the `lockbox-secret` Yandex Lockbox secret parameters into it.
   - `data[].secretKey`: Name of the key in the `.data` field of the secret created by External Secret Operator.
   - `data[].remoteRef.key`: ID of the `lockbox-secret` Yandex Lockbox secret created earlier
     (for example, `e6q28nvfmhu539******`).
   - `data[].remoteRef.property`: Key for the `lockbox-secret` Yandex Lockbox secret.

1. Verify that the new `k8s-secret` key contains the `lockbox-secret` secret value:

   ```shell
   kubectl -n external-secrets get secret k8s-secret -ojson | jq -r '.data.password' | base64 -d
   ```

   The output will contain the `password` key value of the `lockbox-secret` secret created earlier:

   ```console
   p@$$w0rd
   ```

## Integration with Yandex Managed Service for Prometheus

This integration lets you use [Yandex Managed Service for Prometheus](https://yandex.cloud/en/docs/monitoring/operations/prometheus/) as an external storage for metrics (for example, for long-term retention).

To configure PrometheusRemoteWrite, follow these steps:

1. Create a [service account](https://yandex.cloud/en/docs/iam/operations/sa/create) with the `monitoring.editor` role.
1. Create an [API key](https://yandex.cloud/en/docs/iam/operations/authentication/manage-api-keys) for the service account.
1. Create a [PrometheusRemoteWrite](/modules/prometheus/cr.html#prometheusremotewrite) resource:

   ```console
   kubectl apply -f - <<EOF
   
   apiVersion: deckhouse.io/v1
   kind: PrometheusRemoteWrite
   metadata:
     name: yc-remote-write
   spec:
     url: <REMOTE_WRITE_URL>
     bearerToken: <API_KEY>
   EOF
   ```

   Where:

   - `<REMOTE_WRITE_URL>`: URL from the **Yandex Monitoring** -> **Prometheus** -> **Remote write** page.
   - `<API_KEY>`: API key from the previous step (for example, `AQVN1HHJReSrfo9jU3aopsXrJyfq_UHs********`).

   You can specify additional parameters as described in the `prometheus` module [documentation](/modules/prometheus/cr.html#prometheusremotewrite).

For more details on writing metrics, refer to the [Yandex Cloud documentation](https://yandex.cloud/en/docs/monitoring/operations/prometheus/ingestion/remote-write).

To read metrics in Grafana:

1. Create a [service account](https://yandex.cloud/en/docs/iam/operations/sa/create) with the `monitoring.viewer` role.
1. Create an [API key](https://yandex.cloud/en/docs/iam/operations/authentication/manage-api-keys) for the service account.
1. Create a [GrafanaAdditionalDatasource](/modules/prometheus/cr.html#grafanaadditionaldatasource) resource:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: GrafanaAdditionalDatasource
   metadata:
     name: managed-prometheus
   spec:
     type: prometheus
     access: Proxy
     url: <GRAFANA_READ_URL>
     basicAuth: false
     jsonData:
       timeInterval: 30s
       httpMethod: POST
       httpHeaderName1: Authorization
     secureJsonData:
       httpHeaderValue1: Bearer <API_KEY>
   ```

   Where:

   - `<GRAFANA_READ_URL>`: URL from the **Yandex Monitoring** -> **Prometheus** -> **Reading Grafana metrics** page.
   - `<API_KEY>`: API key from the previous step (for example, `AQVN1HHJReSrfo9jU3aopsXrJyfq_UHs********`).

   You can specify additional parameters as described in the `prometheus` module [documentation](/modules/prometheus/cr.html#grafanaadditionaldatasource).

For more details on reading metrics with Grafana, refer to the [Yandex Cloud documentation](https://yandex.cloud/en/docs/monitoring/operations/prometheus/querying/grafana).
