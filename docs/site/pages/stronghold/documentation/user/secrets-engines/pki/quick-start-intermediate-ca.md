---
title: "Quick Start - Intermediate CA Setup"
permalink: en/stronghold/documentation/user/secrets-engines/pki/quick-start-intermediate-ca.html
lang: en
description: The PKI secrets engine for Stronghold generates TLS certificates.
---

In the [first Quick Start guide](/docs/secrets/pki/quick-start-root-ca),
certificates were issued directly from the root certificate authority.
As described in the example, this is not a recommended practice. This guide
builds on the previous guide's root certificate authority and creates an
intermediate authority using the root authority to sign the intermediate's
certificate.

#### Mount the backend

To add another certificate authority to our Stronghold instance, we have to mount it
at a different path.

```shell-session
$ d8 stronghold secrets enable -path=pki_int pki
Success! Enabled the pki secrets engine at: pki_int/
```

#### Configure an intermediate CA

```shell-session
$ d8 stronghold secrets tune -max-lease-ttl=43800h pki_int
Success! Tuned the secrets engine at: pki_int/
```

That sets the maximum TTL for secrets issued from the mount to 5 years. This
value should be less than or equal to the root certificate authority.

Now, we generate our intermediate certificate signing request:

```shell-session
$ d8 stronghold write pki_int/intermediate/generate/internal common_name="mycompany.com Intermediate Authority" ttl=43800h
Key       Value
---       -----
csr       -----BEGIN CERTIFICATE REQUEST-----
MIICdDCCAVwCAQAwLzEtMCsGA1UEAxMkbXljb21wYW55LmNvbSBJbnRlcm1lZGlh
dGUgQXV0aG9yaXR5MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAt8gB
3SFRKQAsyUxYCjvVJnvyS6hmDMpe6cnSsImw1lRlW/tFtZQ8oWfeH3xoKdQvHBpB
ITMvD+VEVQXsAWxYPjz4Sb/TE5OImox+0sn0+vAyOw1xRqoWhbfNq1Xcuou80Tl9
OOovjlTC6sxtzPultpT8+2i/g0I3Haai1HqhoguiMSNfaBMJLPXZT7HI5PcwSdQf
5TzpIpW1P5L1PDXOGSAS0uoUPy/qI+xC9TxHdxk4pemAmgnK3ksgDugkgDI9fxSy
Xmnqj/V93nJ2yqVDKLI3cqr86uqZUTOL44+b82YQbZWg/3GsP2IxzNqeIdjRSc7P
A+wdZD2wUjt2DbmqqwIDAQABoAAwDQYJKoZIhvcNAQELBQADggEBACZrIIohnzFH
f3q7NrTZgrwzjQoVVkXOWtNhON/0Oes8lXNQXdiDThlcUdkwttW+Dpk5SwiBhCac
VGz6CjriXmdGeqpiz37uC32sT8+XEthTuFpD4+s9Uw6hrzyUktH8gmT2Zxz95XgW
foLUclFrLawGOAvE61N/ineosk7UrP0rvTU1ZIXKGWPyPUWpAKTGdNZm/w4AnfVH
aKouKQQaB3eWFHgL3P7Ibe+CgeVGNv/X3pfixkXwZXqLqgZj/glas9WvrRwFA4yH
3dMa9NuDuu0/XAtITQjny2w9m3JDoGax4DYrI7EVzDDsDXAEG6vFLpeZVEOBsmep
hwERznHOX2I=
-----END CERTIFICATE REQUEST-----
key_id    fbf5273c-9abb-49e7-cc7d-a071fb6ed00a
```

Take the signing request from the intermediate authority and sign it using
another certificate authority, in this case the root certificate authority
generated in the first example.

```shell-session
$ d8 stronghold write pki/root/sign-intermediate csr=@pki_int.csr format=pem_bundle ttl=43800h
Key              Value
---              -----
ca_chain         [-----BEGIN CERTIFICATE-----
MIIDvDCCAqSgAwIBAgIUMRHtnLYjFuUgE8BpcBOaSLuTgXwwDQYJKoZIhvcNAQEL
BQAwGDEWMBQGA1UEAxMNbXktd2Vic2l0ZS5ydTAeFw0yNDEyMjgxNTEzNDhaFw0y
NTEyMjgxNTE0MThaMC8xLTArBgNVBAMTJG15Y29tcGFueS5jb20gSW50ZXJtZWRp
YXRlIEF1dGhvcml0eTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBALfI
Ad0hUSkALMlMWAo71SZ78kuoZgzKXunJ0rCJsNZUZVv7RbWUPKFn3h98aCnULxwa
QSEzLw/lRFUF7AFsWD48+Em/0xOTiJqMftLJ9PrwMjsNcUaqFoW3zatV3LqLvNE5
fTjqL45UwurMbcz7pbaU/Ptov4NCNx2motR6oaILojEjX2gTCSz12U+xyOT3MEnU
H+U86SKVtT+S9Tw1zhkgEtLqFD8v6iPsQvU8R3cZOKXpgJoJyt5LIA7oJIAyPX8U
sl5p6o/1fd5ydsqlQyiyN3Kq/OrqmVEzi+OPm/NmEG2VoP9xrD9iMczaniHY0UnO
zwPsHWQ9sFI7dg25qqsCAwEAAaOB5jCB4zAOBgNVHQ8BAf8EBAMCAQYwDwYDVR0T
AQH/BAUwAwEB/zAdBgNVHQ4EFgQUFKBJrVkf09joKSKfEaeXqPVQl5EwHwYDVR0j
BBgwFoAUTDkSDys1tthH5fefhWyrCIxUTPIwRAYIKwYBBQUHAQEEODA2MDQGCCsG
AQUFBzAChihodHRwczovL3N0cm9uZ2hvbGQuZXhhbXBsZS5jb20vdjEvcGtpL2Nh
MDoGA1UdHwQzMDEwL6AtoCuGKWh0dHBzOi8vc3Ryb25naG9sZC5leGFtcGxlLmNv
bS92MS9wa2kvY3JsMA0GCSqGSIb3DQEBCwUAA4IBAQAwP+iei6UZ5RtyJ9hTaygk
K3nJxQbAyGm5O4pf5JjUKRJJHB09OHWY+K/QlOgef8f+pSmebame+JDUO4yc4tK2
8lzFA6wCeI98mk14TA/oHFkbPy3U0bHhfwmFDxy6umip/G9xGCoxrZ/kFuUBjMkB
ZBUsw1QO0iexGuTr7JpcLoseQFUTaT/MB9i9LIqjc/TuhTVNmv2uMSD803kfxr1/
gi4tIrBwcILu+TQKoLZmS5AzESvWX9lg6gi+E2AUvrjqxLNXTzDuUQz+k2YgeE1b
fSOWlAlFAkJBIuR9uMvkEoRmcuXMjkgxnjcKKKssr6/mQP3DRmsfzSlUiM7k9Ght
-----END CERTIFICATE----- -----BEGIN CERTIFICATE-----
MIIDOzCCAiOgAwIBAgIUDeh2Qguyww8FFuMIHUnTNAutRuIwDQYJKoZIhvcNAQEL
BQAwGDEWMBQGA1UEAxMNbXktd2Vic2l0ZS5ydTAeFw0yNDEyMjgxNTA1MzlaFw0y
NTEyMjgxNTA2MDlaMBgxFjAUBgNVBAMTDW15LXdlYnNpdGUucnUwggEiMA0GCSqG
SIb3DQEBAQUAA4IBDwAwggEKAoIBAQCgvdGja/Shz6K0SaFZLbvNg79a6m5e+8la
Lq8U0h6+iXINpfFIMqnu9RQjHO1JJ5ZWkFT7pTK2QcDdIxoMya2HvGXla5Q0SPXU
SMdLDcDumVdhJAddI5usvDPdYhU8j87xcbfphxH5nUeu9JfhSby4Cwpj6kBW1bAM
n4rQ4xoopTdKAkFfs71lfgk2pCxcdH0yDeU3UhadPApV/8WdasgBh8hYzOTZhFsm
PGIpBDtuKNi1o7muZqRReLOJWj0lJMr4UR1XJtOCqTfZ2Kf2RQKu8QEXBZ46o7h0
tN2Kn/lhExSnJWT5670ZHgq2Ewm+zMGdsETCcC8etLIduwEcs3wHAgMBAAGjfTB7
MA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBRMORIP
KzW22Efl95+FbKsIjFRM8jAfBgNVHSMEGDAWgBRMORIPKzW22Efl95+FbKsIjFRM
8jAYBgNVHREEETAPgg1teS13ZWJzaXRlLnJ1MA0GCSqGSIb3DQEBCwUAA4IBAQBL
zrZUsJf7p6TVS+zaTYLeIOsBZwyiMMmEN3UZlJ/a8zTJRYB7n/+g1FGDk7nRhBRN
IGROuUPtrz/hHMpEdwJWwzMMC64Wph4VDnzG6fK+aehTtKt1LWdL8HPvvuN1QBAM
Y0KVb+0csT1RPqXyCQc2phO3f0tRPgOm2eua6+s6p2tzN5BiN3yNFkMpaQBI6h5B
N69l4T9Q4Pf9a9UBmobvssLf73iK3csjBGFlMp9i6+CfJ04GlKP18UOGW4/0naTe
JwcRRIr6Bo3m5F404jjj6MQ4SiaE3t5DH9bW6ogCkCI0MtemxbLJ9Tk6RKYCgUZX
Rrcrw7YXK5bUEqe1Plrb
-----END CERTIFICATE-----]
certificate      -----BEGIN CERTIFICATE-----
MIIDvDCCAqSgAwIBAgIUMRHtnLYjFuUgE8BpcBOaSLuTgXwwDQYJKoZIhvcNAQEL
BQAwGDEWMBQGA1UEAxMNbXktd2Vic2l0ZS5ydTAeFw0yNDEyMjgxNTEzNDhaFw0y
NTEyMjgxNTE0MThaMC8xLTArBgNVBAMTJG15Y29tcGFueS5jb20gSW50ZXJtZWRp
YXRlIEF1dGhvcml0eTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBALfI
Ad0hUSkALMlMWAo71SZ78kuoZgzKXunJ0rCJsNZUZVv7RbWUPKFn3h98aCnULxwa
QSEzLw/lRFUF7AFsWD48+Em/0xOTiJqMftLJ9PrwMjsNcUaqFoW3zatV3LqLvNE5
fTjqL45UwurMbcz7pbaU/Ptov4NCNx2motR6oaILojEjX2gTCSz12U+xyOT3MEnU
H+U86SKVtT+S9Tw1zhkgEtLqFD8v6iPsQvU8R3cZOKXpgJoJyt5LIA7oJIAyPX8U
sl5p6o/1fd5ydsqlQyiyN3Kq/OrqmVEzi+OPm/NmEG2VoP9xrD9iMczaniHY0UnO
zwPsHWQ9sFI7dg25qqsCAwEAAaOB5jCB4zAOBgNVHQ8BAf8EBAMCAQYwDwYDVR0T
AQH/BAUwAwEB/zAdBgNVHQ4EFgQUFKBJrVkf09joKSKfEaeXqPVQl5EwHwYDVR0j
BBgwFoAUTDkSDys1tthH5fefhWyrCIxUTPIwRAYIKwYBBQUHAQEEODA2MDQGCCsG
AQUFBzAChihodHRwczovL3N0cm9uZ2hvbGQuZXhhbXBsZS5jb20vdjEvcGtpL2Nh
MDoGA1UdHwQzMDEwL6AtoCuGKWh0dHBzOi8vc3Ryb25naG9sZC5leGFtcGxlLmNv
bS92MS9wa2kvY3JsMA0GCSqGSIb3DQEBCwUAA4IBAQAwP+iei6UZ5RtyJ9hTaygk
K3nJxQbAyGm5O4pf5JjUKRJJHB09OHWY+K/QlOgef8f+pSmebame+JDUO4yc4tK2
8lzFA6wCeI98mk14TA/oHFkbPy3U0bHhfwmFDxy6umip/G9xGCoxrZ/kFuUBjMkB
ZBUsw1QO0iexGuTr7JpcLoseQFUTaT/MB9i9LIqjc/TuhTVNmv2uMSD803kfxr1/
gi4tIrBwcILu+TQKoLZmS5AzESvWX9lg6gi+E2AUvrjqxLNXTzDuUQz+k2YgeE1b
fSOWlAlFAkJBIuR9uMvkEoRmcuXMjkgxnjcKKKssr6/mQP3DRmsfzSlUiM7k9Ght
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIDOzCCAiOgAwIBAgIUDeh2Qguyww8FFuMIHUnTNAutRuIwDQYJKoZIhvcNAQEL
BQAwGDEWMBQGA1UEAxMNbXktd2Vic2l0ZS5ydTAeFw0yNDEyMjgxNTA1MzlaFw0y
NTEyMjgxNTA2MDlaMBgxFjAUBgNVBAMTDW15LXdlYnNpdGUucnUwggEiMA0GCSqG
SIb3DQEBAQUAA4IBDwAwggEKAoIBAQCgvdGja/Shz6K0SaFZLbvNg79a6m5e+8la
Lq8U0h6+iXINpfFIMqnu9RQjHO1JJ5ZWkFT7pTK2QcDdIxoMya2HvGXla5Q0SPXU
SMdLDcDumVdhJAddI5usvDPdYhU8j87xcbfphxH5nUeu9JfhSby4Cwpj6kBW1bAM
n4rQ4xoopTdKAkFfs71lfgk2pCxcdH0yDeU3UhadPApV/8WdasgBh8hYzOTZhFsm
PGIpBDtuKNi1o7muZqRReLOJWj0lJMr4UR1XJtOCqTfZ2Kf2RQKu8QEXBZ46o7h0
tN2Kn/lhExSnJWT5670ZHgq2Ewm+zMGdsETCcC8etLIduwEcs3wHAgMBAAGjfTB7
MA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBRMORIP
KzW22Efl95+FbKsIjFRM8jAfBgNVHSMEGDAWgBRMORIPKzW22Efl95+FbKsIjFRM
8jAYBgNVHREEETAPgg1teS13ZWJzaXRlLnJ1MA0GCSqGSIb3DQEBCwUAA4IBAQBL
zrZUsJf7p6TVS+zaTYLeIOsBZwyiMMmEN3UZlJ/a8zTJRYB7n/+g1FGDk7nRhBRN
IGROuUPtrz/hHMpEdwJWwzMMC64Wph4VDnzG6fK+aehTtKt1LWdL8HPvvuN1QBAM
Y0KVb+0csT1RPqXyCQc2phO3f0tRPgOm2eua6+s6p2tzN5BiN3yNFkMpaQBI6h5B
N69l4T9Q4Pf9a9UBmobvssLf73iK3csjBGFlMp9i6+CfJ04GlKP18UOGW4/0naTe
JwcRRIr6Bo3m5F404jjj6MQ4SiaE3t5DH9bW6ogCkCI0MtemxbLJ9Tk6RKYCgUZX
Rrcrw7YXK5bUEqe1Plrb
-----END CERTIFICATE-----
expiration       1766934858
issuing_ca       -----BEGIN CERTIFICATE-----
MIIDOzCCAiOgAwIBAgIUDeh2Qguyww8FFuMIHUnTNAutRuIwDQYJKoZIhvcNAQEL
BQAwGDEWMBQGA1UEAxMNbXktd2Vic2l0ZS5ydTAeFw0yNDEyMjgxNTA1MzlaFw0y
NTEyMjgxNTA2MDlaMBgxFjAUBgNVBAMTDW15LXdlYnNpdGUucnUwggEiMA0GCSqG
SIb3DQEBAQUAA4IBDwAwggEKAoIBAQCgvdGja/Shz6K0SaFZLbvNg79a6m5e+8la
Lq8U0h6+iXINpfFIMqnu9RQjHO1JJ5ZWkFT7pTK2QcDdIxoMya2HvGXla5Q0SPXU
SMdLDcDumVdhJAddI5usvDPdYhU8j87xcbfphxH5nUeu9JfhSby4Cwpj6kBW1bAM
n4rQ4xoopTdKAkFfs71lfgk2pCxcdH0yDeU3UhadPApV/8WdasgBh8hYzOTZhFsm
PGIpBDtuKNi1o7muZqRReLOJWj0lJMr4UR1XJtOCqTfZ2Kf2RQKu8QEXBZ46o7h0
tN2Kn/lhExSnJWT5670ZHgq2Ewm+zMGdsETCcC8etLIduwEcs3wHAgMBAAGjfTB7
MA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBRMORIP
KzW22Efl95+FbKsIjFRM8jAfBgNVHSMEGDAWgBRMORIPKzW22Efl95+FbKsIjFRM
8jAYBgNVHREEETAPgg1teS13ZWJzaXRlLnJ1MA0GCSqGSIb3DQEBCwUAA4IBAQBL
zrZUsJf7p6TVS+zaTYLeIOsBZwyiMMmEN3UZlJ/a8zTJRYB7n/+g1FGDk7nRhBRN
IGROuUPtrz/hHMpEdwJWwzMMC64Wph4VDnzG6fK+aehTtKt1LWdL8HPvvuN1QBAM
Y0KVb+0csT1RPqXyCQc2phO3f0tRPgOm2eua6+s6p2tzN5BiN3yNFkMpaQBI6h5B
N69l4T9Q4Pf9a9UBmobvssLf73iK3csjBGFlMp9i6+CfJ04GlKP18UOGW4/0naTe
JwcRRIr6Bo3m5F404jjj6MQ4SiaE3t5DH9bW6ogCkCI0MtemxbLJ9Tk6RKYCgUZX
Rrcrw7YXK5bUEqe1Plrb
-----END CERTIFICATE-----
serial_number    31:11:ed:9c:b6:23:16:e5:20:13:c0:69:70:13:9a:48:bb:93:81:7c
```

Now set the intermediate certificate authorities signing certificate to the
root-signed certificate. You may pass a bundle of PEM-encoded certificates,
for example the complete certificate chain.

```shell-session
$ d8 stronghold write pki_int/intermediate/set-signed certificate=@cert_bundle.pem
Key                 Value
---                 -----
existing_issuers    <nil>
existing_keys       <nil>
imported_issuers    [a736abe5-af0c-7117-1614-80bf91f30440 f2d3d94b-b56e-1258-698e-90070db5df65]
imported_keys       <nil>
mapping             map[a736abe5-af0c-7117-1614-80bf91f30440:fbf5273c-9abb-49e7-cc7d-a071fb6ed00a f2d3d94b-b56e-1258-698e-90070db5df65:]
```

The intermediate certificate authority is now configured and ready to issue
certificates.

#### Set URL configuration

Generated certificates can have the CRL location and the location of the
issuing certificate encoded. These values must be set manually, but can be
changed at any time.

```shell-session
$ d8 stronghold write pki_int/config/urls issuing_certificates="https://stronghold.example.com/v1/pki_int/ca" crl_distribution_points="https://stronghold.example.com/v1/pki_int/crl"
Key                        Value
---                        -----
crl_distribution_points    [https://stronghold.example.com/v1/pki_int/crl]
enable_templating          false
issuing_certificates       [https://stronghold.example.com/v1/pki_int/ca]
ocsp_servers               []
```

#### Configure a role

The next step is to configure a role. A role is a logical name that maps to a
policy used to generate those credentials. For example, let's create an
"example-dot-com" role:

```shell-session
$ d8 stronghold write pki_int/roles/example-dot-com \
    allowed_domains=example.com \
    allow_subdomains=true max_ttl=72h
```

#### Issue certificates

By writing to the `roles/example-dot-com` path we are defining the
`example-dot-com` role. To generate a new certificate, we simply write
to the `issue` endpoint with that role name: Stronghold is now configured to create
and manage certificates!

```shell-session
$ d8 stronghold write pki_int/issue/example-dot-com \
    common_name=blah.example.com
Key                 Value
---                 -----
ca_chain            [-----BEGIN CERTIFICATE-----
MIIDvDCCAqSgAwIBAgIUMRHtnLYjFuUgE8BpcBOaSLuTgXwwDQYJKoZIhvcNAQEL
BQAwGDEWMBQGA1UEAxMNbXktd2Vic2l0ZS5ydTAeFw0yNDEyMjgxNTEzNDhaFw0y
NTEyMjgxNTE0MThaMC8xLTArBgNVBAMTJG15Y29tcGFueS5jb20gSW50ZXJtZWRp
YXRlIEF1dGhvcml0eTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBALfI
Ad0hUSkALMlMWAo71SZ78kuoZgzKXunJ0rCJsNZUZVv7RbWUPKFn3h98aCnULxwa
QSEzLw/lRFUF7AFsWD48+Em/0xOTiJqMftLJ9PrwMjsNcUaqFoW3zatV3LqLvNE5
fTjqL45UwurMbcz7pbaU/Ptov4NCNx2motR6oaILojEjX2gTCSz12U+xyOT3MEnU
H+U86SKVtT+S9Tw1zhkgEtLqFD8v6iPsQvU8R3cZOKXpgJoJyt5LIA7oJIAyPX8U
sl5p6o/1fd5ydsqlQyiyN3Kq/OrqmVEzi+OPm/NmEG2VoP9xrD9iMczaniHY0UnO
zwPsHWQ9sFI7dg25qqsCAwEAAaOB5jCB4zAOBgNVHQ8BAf8EBAMCAQYwDwYDVR0T
AQH/BAUwAwEB/zAdBgNVHQ4EFgQUFKBJrVkf09joKSKfEaeXqPVQl5EwHwYDVR0j
BBgwFoAUTDkSDys1tthH5fefhWyrCIxUTPIwRAYIKwYBBQUHAQEEODA2MDQGCCsG
AQUFBzAChihodHRwczovL3N0cm9uZ2hvbGQuZXhhbXBsZS5jb20vdjEvcGtpL2Nh
MDoGA1UdHwQzMDEwL6AtoCuGKWh0dHBzOi8vc3Ryb25naG9sZC5leGFtcGxlLmNv
bS92MS9wa2kvY3JsMA0GCSqGSIb3DQEBCwUAA4IBAQAwP+iei6UZ5RtyJ9hTaygk
K3nJxQbAyGm5O4pf5JjUKRJJHB09OHWY+K/QlOgef8f+pSmebame+JDUO4yc4tK2
8lzFA6wCeI98mk14TA/oHFkbPy3U0bHhfwmFDxy6umip/G9xGCoxrZ/kFuUBjMkB
ZBUsw1QO0iexGuTr7JpcLoseQFUTaT/MB9i9LIqjc/TuhTVNmv2uMSD803kfxr1/
gi4tIrBwcILu+TQKoLZmS5AzESvWX9lg6gi+E2AUvrjqxLNXTzDuUQz+k2YgeE1b
fSOWlAlFAkJBIuR9uMvkEoRmcuXMjkgxnjcKKKssr6/mQP3DRmsfzSlUiM7k9Ght
-----END CERTIFICATE----- -----BEGIN CERTIFICATE-----
MIIDOzCCAiOgAwIBAgIUDeh2Qguyww8FFuMIHUnTNAutRuIwDQYJKoZIhvcNAQEL
BQAwGDEWMBQGA1UEAxMNbXktd2Vic2l0ZS5ydTAeFw0yNDEyMjgxNTA1MzlaFw0y
NTEyMjgxNTA2MDlaMBgxFjAUBgNVBAMTDW15LXdlYnNpdGUucnUwggEiMA0GCSqG
SIb3DQEBAQUAA4IBDwAwggEKAoIBAQCgvdGja/Shz6K0SaFZLbvNg79a6m5e+8la
Lq8U0h6+iXINpfFIMqnu9RQjHO1JJ5ZWkFT7pTK2QcDdIxoMya2HvGXla5Q0SPXU
SMdLDcDumVdhJAddI5usvDPdYhU8j87xcbfphxH5nUeu9JfhSby4Cwpj6kBW1bAM
n4rQ4xoopTdKAkFfs71lfgk2pCxcdH0yDeU3UhadPApV/8WdasgBh8hYzOTZhFsm
PGIpBDtuKNi1o7muZqRReLOJWj0lJMr4UR1XJtOCqTfZ2Kf2RQKu8QEXBZ46o7h0
tN2Kn/lhExSnJWT5670ZHgq2Ewm+zMGdsETCcC8etLIduwEcs3wHAgMBAAGjfTB7
MA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBRMORIP
KzW22Efl95+FbKsIjFRM8jAfBgNVHSMEGDAWgBRMORIPKzW22Efl95+FbKsIjFRM
8jAYBgNVHREEETAPgg1teS13ZWJzaXRlLnJ1MA0GCSqGSIb3DQEBCwUAA4IBAQBL
zrZUsJf7p6TVS+zaTYLeIOsBZwyiMMmEN3UZlJ/a8zTJRYB7n/+g1FGDk7nRhBRN
IGROuUPtrz/hHMpEdwJWwzMMC64Wph4VDnzG6fK+aehTtKt1LWdL8HPvvuN1QBAM
Y0KVb+0csT1RPqXyCQc2phO3f0tRPgOm2eua6+s6p2tzN5BiN3yNFkMpaQBI6h5B
N69l4T9Q4Pf9a9UBmobvssLf73iK3csjBGFlMp9i6+CfJ04GlKP18UOGW4/0naTe
JwcRRIr6Bo3m5F404jjj6MQ4SiaE3t5DH9bW6ogCkCI0MtemxbLJ9Tk6RKYCgUZX
Rrcrw7YXK5bUEqe1Plrb
-----END CERTIFICATE-----]
certificate         -----BEGIN CERTIFICATE-----
MIID9DCCAtygAwIBAgIUAQI9XLBh2I/zDTDs0X2jaT0Sm/IwDQYJKoZIhvcNAQEL
BQAwLzEtMCsGA1UEAxMkbXljb21wYW55LmNvbSBJbnRlcm1lZGlhdGUgQXV0aG9y
aXR5MB4XDTI0MTIyODE1MTczMloXDTI0MTIzMTE1MTgwMlowGzEZMBcGA1UEAxMQ
YmxhaC5leGFtcGxlLmNvbTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB
AL79k4LvvumFK4wVPHP47vJWHPI+jnitsId9vUS6H9RLT5BzfPQ8mxOHXySfaYyc
ZrFAGxAF0lAqxt2BZh/D0AN+BkjLi0hwWbVp08Y0v3R9J/ZIvfAwqTWUKETZMPId
w97icbHOxLFH8AZ3/t219asroaFIlwrqnx1vkJNJe1KxKN+Ndx2AS0WyrYY8azTd
pntwaCg0RVzd4jyj4wRS06fsAtszdIryCY/MNPUtoRYMoOO35EjhYnbzKLL0MydY
Cr4ZPO49ZRDNYhGTzAc+s04gKhcZAjm7TLNzqyTeFVMDNoDOpSZFL8qH5dVexryn
oRm7pvHyBAKj4ZgcgurfkNMCAwEAAaOCARowggEWMA4GA1UdDwEB/wQEAwIDqDAd
BgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwHQYDVR0OBBYEFHWDnAeNIDsM
WdLDOrtGfZIYRlyrMB8GA1UdIwQYMBaAFBSgSa1ZH9PY6CkinxGnl6j1UJeRMEgG
CCsGAQUFBwEBBDwwOjA4BggrBgEFBQcwAoYsaHR0cHM6Ly9zdHJvbmdob2xkLmV4
YW1wbGUuY29tL3YxL3BraV9pbnQvY2EwGwYDVR0RBBQwEoIQYmxhaC5leGFtcGxl
LmNvbTA+BgNVHR8ENzA1MDOgMaAvhi1odHRwczovL3N0cm9uZ2hvbGQuZXhhbXBs
ZS5jb20vdjEvcGtpX2ludC9jcmwwDQYJKoZIhvcNAQELBQADggEBAHUtwQQ5jiCT
RNUNLv/EOBS6wZhEEjctAoxBg788tAKr083sfshWLdRoumIjfpRS8CYpOzkcwSPQ
GKcK839zACpIxHSZk2+no5BZcFhcJwzmks+TLdogTkRnwJn8iXg8as24NpzS25AL
fn5S2gW7E0EE3IhnlZBtczrM4WfzUCGJkWuiCMYfNUo4CNELcWcLes8JjfauA6g6
lAzvFPFdju8T8edHgi1BF+D3/iNaMNw6OlwFy9ce9uH5AP5ShxHiYvL7sYG+zTKH
5Urz13/xZV7JVnvBG1bkj/eR4sJjm9j481VeOjKA/sBuQqh3Qlgwgy0GWMMmzAkA
KPsER1F6QkE=
-----END CERTIFICATE-----
expiration          1735658282
issuing_ca          -----BEGIN CERTIFICATE-----
MIIDvDCCAqSgAwIBAgIUMRHtnLYjFuUgE8BpcBOaSLuTgXwwDQYJKoZIhvcNAQEL
BQAwGDEWMBQGA1UEAxMNbXktd2Vic2l0ZS5ydTAeFw0yNDEyMjgxNTEzNDhaFw0y
NTEyMjgxNTE0MThaMC8xLTArBgNVBAMTJG15Y29tcGFueS5jb20gSW50ZXJtZWRp
YXRlIEF1dGhvcml0eTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBALfI
Ad0hUSkALMlMWAo71SZ78kuoZgzKXunJ0rCJsNZUZVv7RbWUPKFn3h98aCnULxwa
QSEzLw/lRFUF7AFsWD48+Em/0xOTiJqMftLJ9PrwMjsNcUaqFoW3zatV3LqLvNE5
fTjqL45UwurMbcz7pbaU/Ptov4NCNx2motR6oaILojEjX2gTCSz12U+xyOT3MEnU
H+U86SKVtT+S9Tw1zhkgEtLqFD8v6iPsQvU8R3cZOKXpgJoJyt5LIA7oJIAyPX8U
sl5p6o/1fd5ydsqlQyiyN3Kq/OrqmVEzi+OPm/NmEG2VoP9xrD9iMczaniHY0UnO
zwPsHWQ9sFI7dg25qqsCAwEAAaOB5jCB4zAOBgNVHQ8BAf8EBAMCAQYwDwYDVR0T
AQH/BAUwAwEB/zAdBgNVHQ4EFgQUFKBJrVkf09joKSKfEaeXqPVQl5EwHwYDVR0j
BBgwFoAUTDkSDys1tthH5fefhWyrCIxUTPIwRAYIKwYBBQUHAQEEODA2MDQGCCsG
AQUFBzAChihodHRwczovL3N0cm9uZ2hvbGQuZXhhbXBsZS5jb20vdjEvcGtpL2Nh
MDoGA1UdHwQzMDEwL6AtoCuGKWh0dHBzOi8vc3Ryb25naG9sZC5leGFtcGxlLmNv
bS92MS9wa2kvY3JsMA0GCSqGSIb3DQEBCwUAA4IBAQAwP+iei6UZ5RtyJ9hTaygk
K3nJxQbAyGm5O4pf5JjUKRJJHB09OHWY+K/QlOgef8f+pSmebame+JDUO4yc4tK2
8lzFA6wCeI98mk14TA/oHFkbPy3U0bHhfwmFDxy6umip/G9xGCoxrZ/kFuUBjMkB
ZBUsw1QO0iexGuTr7JpcLoseQFUTaT/MB9i9LIqjc/TuhTVNmv2uMSD803kfxr1/
gi4tIrBwcILu+TQKoLZmS5AzESvWX9lg6gi+E2AUvrjqxLNXTzDuUQz+k2YgeE1b
fSOWlAlFAkJBIuR9uMvkEoRmcuXMjkgxnjcKKKssr6/mQP3DRmsfzSlUiM7k9Ght
-----END CERTIFICATE-----
private_key         -----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEAvv2Tgu++6YUrjBU8c/ju8lYc8j6OeK2wh329RLof1EtPkHN8
9DybE4dfJJ9pjJxmsUAbEAXSUCrG3YFmH8PQA34GSMuLSHBZtWnTxjS/dH0n9ki9
8DCpNZQoRNkw8h3D3uJxsc7EsUfwBnf+3bX1qyuhoUiXCuqfHW+Qk0l7UrEo3413
HYBLRbKthjxrNN2me3BoKDRFXN3iPKPjBFLTp+wC2zN0ivIJj8w09S2hFgyg47fk
SOFidvMosvQzJ1gKvhk87j1lEM1iEZPMBz6zTiAqFxkCObtMs3OrJN4VUwM2gM6l
JkUvyofl1V7GvKehGbum8fIEAqPhmByC6t+Q0wIDAQABAoIBADO5jzq13Vl3FH0i
vzWVZHlDMyG0KXerapS3TAwR8E7ZepnffSbURxd54R2VAsvvC6zTdMDZIsVNYIBa
7CKMMIjRl7gdUAJ1UwZbu9wBKxzMTIdZ2f7z3s/A6UsEG0pnH0X8w9fo7MIqfmny
E5dOEVOjRGnes/Fj62XYcipBi2GwWxE25UBPyt9r1Dhar0+U9KE6vxPYExjp/w7p
9ZQV/vnHdI6Sumz1bU9Rt6zaHMGe4S1clatlMqWpPSrM4K8PoSr6P/cbdJ9DXOKd
EV3ZH7eZqSiVloj2Bvi/ScGhEIDR2Td9BeD1VC7GuCCD8ndMw5S/o/OPO8lGTy/3
GfJHYakCgYEAzbYniOFkCnQ7U/rkm+aUukujSNPAi0hyy0jmzg6dDDTnlBAgPK1x
gsGn3ReQFQu5Pjy1Vdty6g2iRPwrANPxKdZa0NVBQ7EDNhJB1fDWRyJ61BJ9FlQN
TH9llgTUQZ89XNBXlXkn9EXX7e43u6gXNDASqyP/8YP+m92svLZ4zh0CgYEA7a4m
tNOvSDqPHDat6FdESib0TmJ/ilpidwNNF4bJR1DFI2bqTJialfBh8gdUr5kFamie
NnIBcZOmb9YVfTZzHuNWtGzv5zzbSk0Xbn6uf1fVYjd0fH9sqfvbn1SzW2nGqFPo
MVN/uj2GJF3iatGmo5z0Gs/RZfvwmU6/a9ZBZ68CgYEAn8sLUsyiRWycWVPfGSs4
BK6UnBHA03Dnmvl6MD4xyDWgXedY40lnj0aW+qs/BNoifzHxOkxJK36DukqXrQD1
qKYVzXqaQ9bQw8PS2DlIeeFSwEHMYPfRjMa5RpthtcfYhqxgHIAMhTdr0CrnqCGe
RK/DEKXaPuVldfXwJHcpyBECgYEA3HJHQjaIf7SYobFxcWrnUuN4eu9OnhMg+oOc
UDLaowOeJSzCKZLs5h7TqXj1Kf0CkeRAwfzRq/cnStlEiyMieUagV64mgNHoDq0c
C4cB7+iWaIdIymQhdDO+SrRzuliMQfm5BW8Nq75+mWJeq3aSWXQs0GVqMW4QhREN
6EYL2c8CgYEAmB7E7Dwe+lu6mSEPs3WT0/Ah0sXoqZCVcF5eKcudvxeMfWMCNLR7
qvB0drLEdvwtvDjbIg4y3GGFpf0EkfXFB5VMVzmBWcUkBu8h//69p78RHpCchp43
R/tfWZYSP+L1a7/EEXNJ8Ni3Z/rq9CecHINc2pd06rjceG/3EzKIuhQ=
-----END RSA PRIVATE KEY-----
private_key_type    rsa
serial_number       01:02:3d:5c:b0:61:d8:8f:f3:0d:30:ec:d1:7d:a3:69:3d:12:9b:f2
```

Stronghold has now generated a new set of credentials using the `example-dot-com`
role configuration. Here we see the dynamically generated private key and
certificate. The issuing CA certificate and CA trust chain are returned as well.
The CA Chain returns all the intermediate authorities in the trust chain. The root
authority is not included since that will usually be trusted by the underlying
OS.

## API

The PKI secrets engine has a full HTTP API. Please see the
[PKI secrets engine API](/api-docs/secret/pki) for more
details.
