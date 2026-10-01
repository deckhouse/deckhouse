---
title: Package repositories
permalink: en/admin/configuration/marketplace/package-repository.html
description: "Connect a container registry with packages to Deckhouse Platform Marketplace using PackageRepository. Configure authentication, scan intervals, and monitor repository status."
---

To connect Deckhouse Platform (DP) to a container registry with application packages, use the [PackageRepository](../../../reference/api/cr.html#packagerepository) resource. Once the resource is created, DP scans the registry and creates an [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion) object for each discovered package version.

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

Use one of the following methods to authenticate to the container registry:

- **`dockerCfg`**: Docker configuration in the `~/.docker/config.json` format, Base64-encoded. It must contain an `auths` entry for the registry host: DP uses the username and password from this entry.
- **`login` + `password`**: explicit credentials. If both methods are specified, `login` and `password` take precedence.

  ```yaml
  spec:
    registry:
      repo: registry.example.com/packages
      scheme: HTTPS
      login: my-user
      password: my-password
  ```

If the registry uses a self-signed TLS certificate, specify the CA certificate in the `ca` parameter:

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

By default, DP scans the registry every **6 hours**. To change the interval, set the `scanInterval` parameter. The minimum interval is 3 minutes:

```yaml
spec:
  registry:
    repo: registry.example.com/packages
  scanInterval: 1h30m
```

When scans start and how to start one manually is described in [Scanning](scanning.html#triggering-a-manual-scan).

## Checking repository status

The state of the repository is shown in the status of the PackageRepository object.

To display brief information about the status, use the following command:

```bash
d8 k get packagerepository <REPOSITORY_NAME>
```

Output columns:

| Column | Description |
|---|---|
| `Phase` | Repository phase: `Active` after the first successful scan |
| `Scan` | Time of the last successful scan |
| `Repository` | Repository address (`spec.registry.repo`) |
| `Packages` | Number of packages in the repository |
| `MSG` | Message of the `LastScanSucceeded` condition, for example, the error of the last scan |

For detailed information about the status, use the following command:

```bash
d8 k get packagerepository <REPOSITORY_NAME> -o yaml
```

Key status fields:

| Field | Description |
|---|---|
| `status.phase` | Repository phase: `Active` after the first successful scan |
| `status.lastScanTime` | Time of the last successful scan. A failed scan doesn't update it |
| `status.lastChangeTime` | Time of the last scan that found at least one new version |
| `status.lastNewVersions` | Number of new versions found in the last successful scan |
| `status.packagesCount` | Total number of packages in the repository |
| `status.packages[]` | List of packages with `name` and `type` fields |
| `status.conditions` | Detailed conditions, including `LastScanSucceeded` |

To check the result of the last scan, view the `LastScanSucceeded` condition:

```bash
d8 k get packagerepository <REPOSITORY_NAME> \
  -o jsonpath='{.status.conditions[?(@.type=="LastScanSucceeded")]}'
```

The condition is `True` if the last scan succeeded. Otherwise, it is `False`, and its message contains the error.

## Viewing discovered package versions

After a successful scan, [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion) objects appear in the cluster (you can use the short name `apv`):

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
`MetadataLoaded=True` means that the package's OpenAPI schema, description, and requirements were successfully loaded from the container registry. A package version with `MetadataLoaded=False` cannot be installed until the metadata is loaded.
{% endalert %}
