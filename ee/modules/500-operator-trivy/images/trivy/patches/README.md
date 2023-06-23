# Patches

## 001-trivy-db-mediatype-hack.patch

[issue](https://github.com/aquasecurity/trivy/issues/294)

[discussion](https://github.com/aquasecurity/trivy/discussions/4702)

This is a hack for downloading trivy-db from quay registry. Because quay only accepts several media types of image configs and layers:
```text
Failed validating \'enum\' in schema[\'properties\'][\'config\'][\'properties\'][\'mediaType\']:
    {\'description\': \'The MIME type of the referenced manifest\',
     \'enum\': [\'application/vnd.oci.image.config.v1+json\',
              \'application/vnd.cncf.helm.config.v1+json\',
              \'application/vnd.oci.source.image.config.v1+json\'],
     \'type\': \'string\'}

On instance[\'config\'][\'mediaType\']:
    \'application/vnd.aquasec.trivy.config.v1+json\'}]} {errors:[message:manifest invalid]}
```

## 002-insecure-registry-artifact.patch

[PR](https://github.com/aquasecurity/trivy/pull/4701)

Fix pulling artifacts from http registry:

```text
2023-06-23T13:52:56.086+0300    INFO    Need to update DB
2023-06-23T13:52:56.086+0300    INFO    DB Repository: registry.quay:8080/admin/deckhouse/security/trivy-db-docker
2023-06-23T13:52:56.086+0300    INFO    Downloading DB...
true
true
2023-06-23T13:52:56.426+0300    FATAL   init error: DB error: failed to download vulnerability DB: database download error: OCI repository error: 1 error occurred:
        * Get "https://registry.quay:8080/v2/": http: server gave HTTP response to HTTPS client

```


