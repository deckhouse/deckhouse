### Where the certificates come from

The trust set is not kept in this repository. It is imported from the `ca-certificates`
base image (`candi/alt_base_images.yml`), which container-factory builds from Debian's
[ca-certificates](https://salsa.debian.org/debian/ca-certificates.git) package: the Mozilla
set plus the Russian Trusted CAs of the Ministry of Digital Development.

To refresh the certificates on nodes, bump the base images. Nothing here needs editing:
the package digest follows the imported bundle, so `rpp-get` reinstalls the package and the
install script re-runs `d8-ca-updater`, which hands the new set to the distribution's own
`update-ca-trust` / `update-ca-certificates`.

To inspect what a given base image carries:

```shell
crane config "$(yq '.REGISTRY_PATH' candi/alt_base_images.yml)@$(yq '."ca-certificates"' candi/alt_base_images.yml)"
```
