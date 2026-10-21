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

### Distrusted roots

`scripts/distrusted.txt` holds the Subject Key Identifiers of roots a node must not trust, one per
line, nothing else. `install` does not stage them, and `d8-ca-updater` blocks them in the
distribution's own store: the p11-kit blocklist on the RHEL family, `!` in
`/etc/ca-certificates.conf` on the Debian family. `uninstall` leaves the block in place.

| Subject Key Identifier | Root | Why |
|------------------------|------|-----|
| `7C5D028413D4CC8A9B81CE171C2E291E9C486342` | emSign ECC Root CA - G3 | Cross-signs eMudhra intermediates whose keys leaked ([bug 2083449](https://bugzilla.mozilla.org/show_bug.cgi?id=2083449)) |
| `A0110A233E96F107ECE2AF29EF82A57FD030A4B4` | Comodo AAA Services root | Same intermediates |

Remove an entry once every supported distribution ships a `ca-certificates` without that root.
