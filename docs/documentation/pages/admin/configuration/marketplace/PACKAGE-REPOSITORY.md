---
title: Package repositories
permalink: en/admin/configuration/marketplace/package-repository.html
description: "Connect a package repository to Deckhouse Platform Marketplace using PackageRepository. Configure authentication, scan intervals, and monitor repository status."
---

To connect Deckhouse Platform (DP) to an application package repository, use the [PackageRepository](../../../reference/api/cr.html#packagerepository) resource. Once the resource is created, DP scans the repository and creates an [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion) object for each discovered package version.

Example of a PackageRepository manifest:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: PackageRepository
metadata:
  name: my-registry
spec:
  registry:
    repo: registry.example.com/packages
    scheme: HTTPS
    dockerCfg: <BASE64_ENCODED_DOCKER_CONFIG>
```

## Authentication and scan interval

### Authentication

To authenticate to the package repository, use one of the following methods:

- [`dockerCfg`](../../../reference/api/cr.html#packagerepository-v1alpha1-spec-registry-dockercfg): Docker configuration in the `~/.docker/config.json` format, Base64-encoded. It must contain an `auths` entry for the repository host. DP uses the username and password from this entry.
- [`login`](../../../reference/api/cr.html#packagerepository-v1alpha1-spec-registry-login) and [`password`](../../../reference/api/cr.html#packagerepository-v1alpha1-spec-registry-password): Explicit credentials. If both methods are specified, `login` and `password` take precedence.

  ```yaml
  spec:
    registry:
      repo: registry.example.com/packages
      scheme: HTTPS
      login: my-user
      password: my-password
  ```

If the repository uses a self-signed TLS certificate, specify the CA certificate in the [`ca`](../../../reference/api/cr.html#packagerepository-v1alpha1-spec-registry-ca) parameter:

```yaml
spec:
  registry:
    repo: registry.example.com/packages
    scheme: HTTPS
    dockerCfg: <BASE64_ENCODED_DOCKER_CONFIG>
    ca: |
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
```

### Scan interval

By default, DP scans the repository **every 6 hours**. To change the interval, set the [`scanInterval`](../../../reference/api/cr.html#packagerepository-v1alpha1-spec-scaninterval) parameter. The minimum interval is 3 minutes:

```yaml
spec:
  registry:
    repo: registry.example.com/packages
  scanInterval: 1h30m
```

For details about starting a scanning or viewing results, refer to ["Scanning"](scanning.html).

## Checking repository status

The state of the repository is shown in the status of the PackageRepository object.

To display brief information about the status, use the following command:

```bash
d8 k get packagerepository <REPOSITORY_NAME>
```

Output columns:

| Column | Description |
|---|---|
| `Phase` | Repository phase. Displayed as `Active` after the first successful scan |
| `Scan` | Time of the last successful scan |
| `Repository` | Repository address ([`spec.registry.repo`](../../../reference/api/cr.html#packagerepository-v1alpha1-spec-registry-repo)) |
| `Packages` | Number of packages in the repository |
| `MSG` | Message of the `LastScanSucceeded` condition, for example, the error of the last scan |

For detailed information about the status, use the following command:

```bash
d8 k get packagerepository <REPOSITORY_NAME> -o yaml
```

For status field descriptions, refer to the [PackageRepository](../../../reference/api/cr.html#packagerepository-v1alpha1-status) documentation.

To check the result of the last scan, view the `LastScanSucceeded` condition:

```bash
d8 k get packagerepository <REPOSITORY_NAME> \
  -o jsonpath='{.status.conditions[?(@.type=="LastScanSucceeded")]}'
```

If the last scan completed successfully, the condition status is set to `True`. Otherwise, it is `False`, and the error details are available in the status message.

## Viewing discovered package versions

After a successful scan, [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion) objects (or `apv` for short) are automatically created in the cluster.

To print out a list of all created ApplicationPackageVersion objects, run the following command:

```bash
d8 k get apv
```

Example output:

<!-- markdownlint-disable MD031 -->
```console
NAME                           PACKAGE    REPOSITORY    METADATALOADED   USEDBY   AGE
my-registry-redis-v7.2.0       redis      my-registry   True                      5m
my-registry-postgres-v15.0.0   postgres   my-registry   True                      5m
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

To filter the versions by package name, use the following command (the example shows the versions of the `redis` package):

```bash
d8 k get apv -l packages.deckhouse.io/package=redis
```

{% alert level="info" %}
`MetadataLoaded=True` means that the package's OpenAPI schema, description, and requirements were successfully loaded from the repository. A package version with `MetadataLoaded=False` cannot be installed until the metadata is loaded.
{% endalert %}
