---
title: "The documentation module: usage"
description: "An example of the documentation module configuration, authentication, keeping the prepared documentation of module versions in an S3 storage of your own and the high availability mode."
---

## An example of the module configuration

The module runs on the system nodes, authenticates users through an external authentication service and lets in the users of the `admins` group:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: documentation
spec:
  version: 1
  enabled: true
  settings:
    nodeSelector:
      node-role/system: ""
    tolerations:
    - key: dedicated.deckhouse.io
      operator: Equal
      value: system
    auth:
      externalAuthentication:
        authURL: "https://<applicationDomain>/auth"
        authSignInURL: "https://<applicationDomain>/sign-in"
      allowedUserGroups:
      - admins
```

## Authentication

The [user-authn](../user-authn/) module provides authentication by default. Also, an external authentication service can be set in the [auth.externalAuthentication](configuration.html#parameters-auth-externalauthentication) parameter.
When the `user-authn` module is disabled and the `auth.externalAuthentication` parameter is not set, the module uses basic authentication with the user `admin` and an auto-generated password.

Use `d8 k` to see the password:

```shell
d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values documentation -o json | jq '.internal.auth.password'
```

Delete the Secret to re-generate the password:

```shell
d8 k -n d8-system delete secret/documentation-basic-auth
```

## S3 storage of your own

With `moduleStorage.s3.type: External` the module keeps the prepared documentation of module versions in an S3 bucket of your own. Create a Secret in any namespace with these keys. Deckhouse Platform copies it into the `documentation-s3-connection` Secret in `d8-system`. After the Secret is created or changed, the documentation pod restarts within 5 minutes. Keep only the connection keys in the Secret: all of its keys are copied.

| Key | Required | Value |
|---|---|---|
| `S3_ENDPOINT` | yes | The address of the storage, `https://` only. |
| `S3_BUCKET` | yes | The name of the bucket. |
| `S3_REGION` | yes | The region of the bucket, for example `us-east-1`. |
| `AWS_ACCESS_KEY_ID` | yes | The access key. |
| `AWS_SECRET_ACCESS_KEY` | yes | The secret key. |
| `S3_CA_CERT` | no | The CA certificate of the storage in PEM format, when the system ones do not trust it. |

The key needs the rights to read, write, list and delete objects under the prefix from [moduleStorage.s3.external.prefix](configuration.html#parameters-modulestorage-s3-external-prefix) (`documentation/` by default). The module reads, writes and deletes no object outside of the prefix, so the S3 bucket may be shared with other data.

An example of the Secret and of the module configuration that uses it:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: documentation-s3
  namespace: documentation-storage
stringData:
  S3_ENDPOINT: https://s3.example.com
  S3_BUCKET: documentation
  S3_REGION: us-east-1
  AWS_ACCESS_KEY_ID: <access key>
  AWS_SECRET_ACCESS_KEY: <secret key>
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: documentation
spec:
  version: 1
  settings:
    moduleStorage:
      s3:
        type: External
        external:
          secretRef:
            namespace: documentation-storage
            name: documentation-s3
```

While the Secret does not exist or the storage does not answer, the versions are kept in the pod only. After a restart of the pod the module prepares them again. The `docs_builder_ondemand_s3_ready` metric of the documentation pod is 0 then. A mistake in the Secret (a missing or misnamed key, an `http://` address) is written to the log of the `builder` container of the documentation pod. When the storage stays not ready for 15 minutes, the `D8DocumentationS3NotReady` alert fires.

## High availability mode

The high availability mode is turned on by the [highAvailability](configuration.html#parameters-highavailability) parameter; when the parameter is not set, the HA mode of the cluster applies. In this mode the documentation runs in two replicas on different nodes, so two nodes the module can run on are needed (the system nodes by default). With one such node the second replica stays `Pending`, and the PodDisruptionBudget blocks draining the node with the running replica until the second replica runs on another node. The authenticator of the documentation (DexAuthenticator), if it is deployed, also runs in two replicas.

A user stays on one replica: the `d8-documentation-affinity` cookie picks it. Through an Ingress the cookie lives 24 hours, also after the browser is closed; through the Gateway API it lives until the browser is closed. That replica prepares and keeps the module versions the user opens.

Turn on keeping versions in S3 ([moduleStorage.s3.type](configuration.html#parameters-modulestorage-s3-type): `System` or `External`): then a version prepared by one replica is opened by the other from S3 without preparing it again, and a user moved to the other replica when theirs restarts does not wait for the versions again. Without S3 each replica reads the container registry on its own and prepares on its own the versions the search uses and the versions its users open. In this case the documentation takes twice the CPU and sends twice the requests to the registry, and a user moved to the other replica waits for a version they already opened once more.

With S3 one replica, picked through a Lease, reads the container registry and prepares the versions the search uses; the other takes them from S3 within a minute and prepares only the versions its users open. A replica starts taking users once it has received the versions from S3, at most 5 minutes after its start.

After an update of Deckhouse Platform that changes the look of the documentation, the versions kept in S3 open as they were and are prepared again in the background, one at a time, when no user waits for a version. A version that the new look can not prepare stays as it was.
