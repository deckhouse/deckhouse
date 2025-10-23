---
title: Restoring registry access when license token is expired or invalid
permalink: en/admin/configuration/registry/restore-token.html
---

{% alert level="warning" %}
Use this guide only when the standard [registry change procedure](./third-party.html) is unavailable.
{% endalert %}

If, after the license token expired, the Deckhouse Kubernetes Platform (DKP) pods were restarted, their logs will show a registry connection error when pulling DKP images. To switch the cluster to a new token, run the following steps on any master node:

1. Save the current `deckhouse-registry` secret configuration to a temporary file:

   ```shell
   d8 k -n d8-system get secret deckhouse-registry -o yaml > /tmp/deckhouse-registry.yaml
   ```

1. In the temporary file `/tmp/deckhouse-registry.yaml`, replace the `.dockerconfigjson` field with a Base64-encoded string containing the registry connection parameters. You can generate the required string with the commands below, substituting your own `MYPASSWORD` and `MYREGISTRY` values:

   ```shell
   declare MYUSER='license-token'
   declare MYPASSWORD='example-token'
   declare MYREGISTRY='example-regsitry.deckhouse.ru'
   MYAUTH=$(echo -n "$MYUSER:$MYPASSWORD" | base64 -w0)
   MYRESULTSTRING=$(echo -n "{\"auths\":{\"$MYREGISTRY\":{\"username\":\"$MYUSER\",\"password\":\"$MYPASSWORD\",\"auth\":\"$MYAUTH\"}}}" | base64 -w0)
   echo "$MYRESULTSTRING"
   ```

1. Allow updating the stale secret:

   ```shell
   d8 k delete validatingadmissionpolicybindings.admissionregistration.k8s.io heritage-label-objects.deckhouse.io
   ```

1. Import the updated configuration:

   ```shell
   d8 k -n d8-system apply -f /tmp/deckhouse-registry.yaml
   ```

1. Find the problematic `deckhouse` Pod on the current master node and delete it:

   ```shell
   d8 k get pods -n d8-system -o wide
   d8 k delete pod -n d8-system deckhouse-<id>
   ```

1. Make sure the new `deckhouse` Pod has started successfully:

   ```shell
   d8 k get pods -n d8-system
   ```

1. If necessary, delete any remaining `deckhouse` Pods that are in an incorrect state.

1. Repeat the [standard procedure](./third-party.html) for changing the registry, substituting your token, the required registry address, and the edition instead of `example`:

   ```shell
   d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --user licence-token --password MY-PASSWORD registry-example.deckhouse.ru/deckhouse/example
   ```
