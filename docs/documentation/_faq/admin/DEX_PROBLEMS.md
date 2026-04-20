---
title: What to do if there are problems applying DexProvider settings?
subsystems:
  - iam
lang: en
---

If you have changed the DexProvider settings in the `user-authn` module and one of the following problems occurs:

- No changes are visible (settings are not applied).
- When attempting to log in to the platform's web interface with any type of authorization, a `500 Internal Server Error` occurs without a detailed description.

Perform the following steps:

1. Check the status of the dex deployment:

   ```shell
   d8 k -n d8-user-authn get pod
   ```

   Example output:

   ```shell
   NAME                                    READY   STATUS    RESTARTS   AGE
   dex-5ddb779b7d-6pbhs                    2/2     Running   0          20h
   kubeconfig-generator-7c46977b9f-5kdmc   1/1     Running   0          20h
   ```

   If the module is functioning properly and the correct configuration is specified in [DexProvider](/modules/user-authn/cr.html#dexprovider), all pods will have the status `Running`.

1. Check the logs for the problematic pod:

   ```shell
   d8 k -n d8-user-authn logs dex-<pod-name>
   ```

   Based on the information from the logs, correct the configuration in the [DexProvider](/modules/user-authn/cr.html#dexprovider) resource and wait for the dex pods to restart. Within a few minutes, the pods will restart automatically, and the platform's web interface (located at `console.<CLUSTER_NAME_TEMPLATE>`) will become available and will reflect the changes made to the [DexProvider](/modules/user-authn/cr.html#dexprovider) resource.
