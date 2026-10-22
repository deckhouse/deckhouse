### How update ca-certificates in registry-package-proxy
Get fresh copy:
1. [./ca-certificates/mozilla/certdata.txt](https://hg.mozilla.org/releases/mozilla-release/raw-file/default/security/nss/lib/ckfw/builtins/certdata.txt)
2. [./ca-certificates/mozilla/blacklist.txt](https://salsa.debian.org/debian/ca-certificates/-/blob/master/mozilla/blacklist.txt)

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
