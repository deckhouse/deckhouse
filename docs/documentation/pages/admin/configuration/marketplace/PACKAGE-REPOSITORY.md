---
title: Package repositories
permalink: en/admin/configuration/marketplace/package-repository.html
description: "Connect a package registry to Deckhouse Kubernetes Platform Marketplace using PackageRepository. Configure authentication, scan intervals, and monitor repository status."
---

Connecting the Deckhouse Kubernetes Platform (DKP) to a container registry containing application packages is done using the [PackageRepository](../../../reference/api/cr.html#packagerepository). Once connected, DKP automatically scans the registry and creates [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion) objects for each discovered package version.

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
    dockerCfg: <base64-encoded-docker-config>
```

## Authentication and Scanning Interval Management

### Authentication

One of the following methods can be used for authentication in the registry:

- **`dockerCfg`**: base64-encoded Docker config JSON (`~/.docker/config.json` format). Preferred when the registry uses token-based authentication.
- **`login` + `password`**: explicit credentials.

  ```yaml
  spec:
    registry:
      repo: registry.example.com/packages
      scheme: HTTPS
      login: my-user
      password: my-password
  ```

If the registry uses a self-signed TLS certificate, provide it via `ca`:

```yaml
spec:
  registry:
    repo: registry.example.com/packages
    scheme: HTTPS
    dockerCfg: <base64-encoded-docker-config>
    ca: |
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
```

### Scan interval

By default, DKP rescans the registry every **6 hours**. The interval can be overridden using the `scanInterval` parameter:

```yaml
spec:
  registry:
    repo: registry.example.com/packages
  scanInterval: 1h30m
```

## Checking repository status

The repository's status is displayed in the PackageRepository object's status.

To display brief information about the status, use the following command:

```bash
d8 k get packagerepository <REPOSITORY_NAME>
```

Output columns:

| Column | Description |
|---|---|
| `Phase` | Current state of the repository |
| `Scan` | Timestamp of the last scan |
| `MSG` | Message from the last scan condition |
| `Packages` | Total number of packages discovered (hidden by default, use `-o wide`) |

For detailed information about the status, use the following command:

```bash
d8 k get packagerepository <REPOSITORY_NAME> -o yaml
```

Key status fields:

| Field | Description |
|---|---|
| `status.phase` | Current repository phase |
| `status.lastScanTime` | Time of the most recent scan of any outcome |
| `status.lastChangeTime` | Time of the last scan that found at least one new version |
| `status.lastNewVersions` | Number of new versions found in the most recent scan |
| `status.packagesCount` | Total packages in the repository |
| `status.packages[]` | List of packages with `name` and `type` fields |
| `status.conditions` | Detailed conditions, including `LastScanSucceeded` |

The `LastScanSucceeded` condition:

```bash
d8 k get packagerepository my-registry \
  -o jsonpath='{.status.conditions[?(@.type=="LastScanSucceeded")].message}'
```

## Viewing discovered package versions

After a successful scan, [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion) objects appear in the cluster (you can use the abbreviated name `apv`):

```bash
d8 k get apv
```

Example output:

<!-- markdownlint-disable MD031 -->
```console
NAME                           PACKAGE     REPOSITORY   TRANSITIONTIME   METADATALOADED   MESSAGE   USEDBY
my-registry-redis-v7.2.0       redis       my-registry  5m               True
my-registry-postgres-v15.0.0   postgres    my-registry  5m               True
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

Filtering by package name can be done using the following command (in this example, versions of the `redis` package are being filtered):

```bash
d8 k get apv -l package=redis
```

{% alert level="info" %}
`MetadataLoaded=True` means the package's OpenAPI schema, description, and requirements were successfully loaded from the registry. A package with `MetadataLoaded=False` cannot be installed until the metadata is retrieved.
{% endalert %}
