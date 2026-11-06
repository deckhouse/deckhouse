# Signature CAs

CAs trusted for signatures on Deckhouse images and binaries, picked by the `signature ca certs` define
in [`.werf/defines/signature-ca.tmpl`](../defines/signature-ca.tmpl):

- dev — `dev-int-ca.pem`, `prod-int-ca.pem`
- prod — `prod-int-ca.pem`
- cse — `cse-int-ca.pem`

dev is any build not from a tag, prod is a release build (from a tag) of any edition except CSE, cse is a CSE release build.

Each file is the issuer of a seguro PKI, downloaded from `https://seguro.flant.com/v1/<pki>/issuer/default/pem`:

- `dev-int-ca.pem` — `dev-int-pki`
- `prod-int-ca.pem` — `prod-int-pki`
- `cse-int-ca.pem` — `pki-flant-int-dh`
