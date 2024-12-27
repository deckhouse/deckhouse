---
title: "Quick Start - Root CA Setup"
permalink: en/stronghold/documentation/user/secrets-engines/pki/quick-start-root-ca.html
lang: en
description: The PKI secrets engine for Stronghold generates TLS certificates.
---

This document provides a brief overview of setting up an Stronghold PKI Secrets
Engine with a Root CA certificate.

#### Mount the backend

The first step to using the PKI backend is to mount it. Unlike the `kv`
backend, the `pki` backend is not mounted by default.

```shell-session
$ d8 stronghold secrets enable pki
Successfully mounted 'pki' at 'pki'!
```

#### Configure a CA certificate

Next, Stronghold must be configured with a CA certificate and associated private
key. We'll take advantage of the backend's self-signed root generation support,
but Stronghold also supports generating an intermediate CA (with a CSR for signing)
or setting a PEM-encoded certificate and private key bundle directly into the
backend.

Generally you'll want a root certificate to only be used to sign CA
intermediate certificates, but for this example we'll proceed as if you will
issue certificates directly from the root. As it's a root, we'll want to set a
long maximum life time for the certificate; since it honors the maximum mount
TTL, first we adjust that:

```shell-session
$ d8 stronghold secrets tune -max-lease-ttl=87600h pki
Successfully tuned mount 'pki'!
```

That sets the maximum TTL for secrets issued from the mount to 10 years. (Note
that roles can further restrict the maximum TTL.)

Now, we generate our root certificate:

```shell-session
$ d8 stronghold write pki/root/generate/internal common_name=mycompany.com ttl=87600h
Key              Value
---              -----
certificate      -----BEGIN CERTIFICATE-----
MIIDOzCCAiOgAwIBAgIUH2xy1ESWK1JIpVNpVItdxm+XrWYwDQYJKoZIhvcNAQEL
BQAwGDEWMBQGA1UEAxMNbXljb21wYW55LmNvbTAeFw0yNDEyMjgxNTA4MThaFw0y
NTEyMjgxNTA4NDdaMBgxFjAUBgNVBAMTDW15Y29tcGFueS5jb20wggEiMA0GCSqG
SIb3DQEBAQUAA4IBDwAwggEKAoIBAQDNejVzqfX7mtEoFDL7+89POU5Yl9b2hAN7
CIOJXV6Xc2IirWx7QO/Xs7ytSwWZXn8DfSelonk0L00ZoSh0DARE38mhSvpDSGDR
aaZOxVOAH3J/JkNscqMAklKRHBBwZUjRNwXbAL6v7MkfLXiwCcjE+k84JOw8oLmT
i656ocDIMr6k8juZlzR2RZx2YgQ0Cd56L0tYfn07O73y4NmCfqg10ZiygO65S4LF
raam4BtaUsQ/NqZDrexT8fiJKy9SbHWfkryTnooCErMnpy+F8m8SrhSoiIgN1FOP
xlHq8iLcIYpjzXDWRlJcQ0SkO7Jqw+QOKgcBaQaPwhcHFAxTZgINAgMBAAGjfTB7
MA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBQPWzSl
ljoNIVt4hISwrIAY5iDlrTAfBgNVHSMEGDAWgBQPWzSlljoNIVt4hISwrIAY5iDl
rTAYBgNVHREEETAPgg1teWNvbXBhbnkuY29tMA0GCSqGSIb3DQEBCwUAA4IBAQCP
fZ3LEWvoZdWjubv4ZvhLzb5zyV1+cKBSNI3BakSIp6yWyG1U1ujEkvzchYKMFMbZ
yhiaCzRYuBK7iE1JSpTY5/yHXtVq+UW4ORYT3m1JB2uIyYohp56fLtHTCdVsi2cq
Tm5FpIO870TqJ57FgDEyaCdYW4cO2JSmDQm8oe+WsU90WUVW/LJf24tyawWVtmM4
hfUuaZknChd47InA/ZJfhBFF/XZG4kzzQqxMvpCqOllXi74+cbzuleXFwlEXNJfM
WozbLq+IuMcM5q4On1hEpbN+20p57UnvY9K25tzx5X0ARLnFeo06fHmHB59MKSjo
U1E9GUPbLx1hCiTzsTKv
-----END CERTIFICATE-----
expiration       1766934527
issuer_id        523c7e27-90ca-0df7-6b8f-462f0749a768
issuer_name      n/a
issuing_ca       -----BEGIN CERTIFICATE-----
MIIDOzCCAiOgAwIBAgIUH2xy1ESWK1JIpVNpVItdxm+XrWYwDQYJKoZIhvcNAQEL
BQAwGDEWMBQGA1UEAxMNbXljb21wYW55LmNvbTAeFw0yNDEyMjgxNTA4MThaFw0y
NTEyMjgxNTA4NDdaMBgxFjAUBgNVBAMTDW15Y29tcGFueS5jb20wggEiMA0GCSqG
SIb3DQEBAQUAA4IBDwAwggEKAoIBAQDNejVzqfX7mtEoFDL7+89POU5Yl9b2hAN7
CIOJXV6Xc2IirWx7QO/Xs7ytSwWZXn8DfSelonk0L00ZoSh0DARE38mhSvpDSGDR
aaZOxVOAH3J/JkNscqMAklKRHBBwZUjRNwXbAL6v7MkfLXiwCcjE+k84JOw8oLmT
i656ocDIMr6k8juZlzR2RZx2YgQ0Cd56L0tYfn07O73y4NmCfqg10ZiygO65S4LF
raam4BtaUsQ/NqZDrexT8fiJKy9SbHWfkryTnooCErMnpy+F8m8SrhSoiIgN1FOP
xlHq8iLcIYpjzXDWRlJcQ0SkO7Jqw+QOKgcBaQaPwhcHFAxTZgINAgMBAAGjfTB7
MA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBQPWzSl
ljoNIVt4hISwrIAY5iDlrTAfBgNVHSMEGDAWgBQPWzSlljoNIVt4hISwrIAY5iDl
rTAYBgNVHREEETAPgg1teWNvbXBhbnkuY29tMA0GCSqGSIb3DQEBCwUAA4IBAQCP
fZ3LEWvoZdWjubv4ZvhLzb5zyV1+cKBSNI3BakSIp6yWyG1U1ujEkvzchYKMFMbZ
yhiaCzRYuBK7iE1JSpTY5/yHXtVq+UW4ORYT3m1JB2uIyYohp56fLtHTCdVsi2cq
Tm5FpIO870TqJ57FgDEyaCdYW4cO2JSmDQm8oe+WsU90WUVW/LJf24tyawWVtmM4
hfUuaZknChd47InA/ZJfhBFF/XZG4kzzQqxMvpCqOllXi74+cbzuleXFwlEXNJfM
WozbLq+IuMcM5q4On1hEpbN+20p57UnvY9K25tzx5X0ARLnFeo06fHmHB59MKSjo
U1E9GUPbLx1hCiTzsTKv
-----END CERTIFICATE-----
key_id           94faaebc-0f5f-5fde-7e9f-64bf29ebecfc
key_name         n/a
serial_number    1f:6c:72:d4:44:96:2b:52:48:a5:53:69:54:8b:5d:c6:6f:97:ad:66
```

The returned certificate is purely informational; it and its private key are
safely stored in the backend mount.

#### Set URL configuration

Generated certificates can have the CRL location and the location of the
issuing certificate encoded. These values must be set manually and typically to FQDN associated to the Stronghold server, but can be changed at any time.

```shell-session
$ d8 stronghold write pki/config/urls issuing_certificates="https://stronghold.example.com/v1/pki/ca" crl_distribution_points="https://stronghold.example.com/v1/pki/crl"
Key                        Value
---                        -----
crl_distribution_points    [https://stronghold.example.com/v1/pki/crl]
enable_templating          false
issuing_certificates       [https://stronghold.example.com/v1/pki/ca]
ocsp_servers               []
```

#### Configure a role

The next step is to configure a role. A role is a logical name that maps to a
policy used to generate those credentials. For example, let's create an
"example-dot-com" role:

```shell-session
$ d8 stronghold write pki/roles/example-dot-com \
    allowed_domains=example.com \
    allow_subdomains=true max_ttl=72h
```

#### Issue certificates

By writing to the `roles/example-dot-com` path we are defining the
`example-dot-com` role. To generate a new certificate, we simply write
to the `issue` endpoint with that role name: Stronghold is now configured to create
and manage certificates!

```shell-session
$ d8 stronghold write pki/issue/example-dot-com \
    common_name=blah.example.com
Key                 Value
---                 -----
ca_chain            [-----BEGIN CERTIFICATE-----
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
MIID1TCCAr2gAwIBAgIUKjOsigLTSo0s4OhoULv+1/qSHZowDQYJKoZIhvcNAQEL
BQAwGDEWMBQGA1UEAxMNbXktd2Vic2l0ZS5ydTAeFw0yNDEyMjgxNTEwNDZaFw0y
NDEyMzExNTExMTZaMBsxGTAXBgNVBAMTEGJsYWguZXhhbXBsZS5jb20wggEiMA0G
CSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC4I3hnGI5W4HrwXQoz18kgTmG7pOkw
GltUXLiRvYqnMOYM758WMVdQDphNo+j9KAUzIhL+ENtkPc/rVV7Lh+hKvw5WRRAk
RRwEzwjyczKDwn2iHw5s2XdCKJzySyAHLkqOd53rmEQhtEdEWOKPuseuHWk11bjC
CYEKpDQqzBVbpsmeVoT3MG1+QOSwdyJVHZV1436Ah7UKpwrAWdoe+7WzQkwyFRup
iDstMSM5luRJ2XLYQgDTky93nxXvJxO5xGBDT1VRKl9Sns3lDhzE2fI5ENsIm9YG
76PED3v5S7CxesCGrtjw8ajrdhv2vYlsCtfeiOxxdzebz50HaE5GLSlnAgMBAAGj
ggESMIIBDjAOBgNVHQ8BAf8EBAMCA6gwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsG
AQUFBwMCMB0GA1UdDgQWBBQOvV8aezSQt4efp0ZrA9gHYE4GvDAfBgNVHSMEGDAW
gBRMORIPKzW22Efl95+FbKsIjFRM8jBEBggrBgEFBQcBAQQ4MDYwNAYIKwYBBQUH
MAKGKGh0dHBzOi8vc3Ryb25naG9sZC5leGFtcGxlLmNvbS92MS9wa2kvY2EwGwYD
VR0RBBQwEoIQYmxhaC5leGFtcGxlLmNvbTA6BgNVHR8EMzAxMC+gLaArhilodHRw
czovL3N0cm9uZ2hvbGQuZXhhbXBsZS5jb20vdjEvcGtpL2NybDANBgkqhkiG9w0B
AQsFAAOCAQEAEFraU8C6UjzXFFwJC0sBey+KXbjaUOCtcsZpfSeWfllL4dNhYZbR
FGKTiUoK52dcnD/TCrchr8WuUZcMIKqiyczty2jTxOvGqrCcjp26GNwDWegNQDmf
WysW+fT/eBPgPeTjwXyVTKG+6Es9LONoc25vgAvK3tlBBQpvSD/8POEiep4sCyPc
P9JSojb5thduE2Pc8MdhmVuq9Jyylpdo0wZJ4PHkML0KFVXtTWcQ1ZnYxQmFaoQx
nRcg5neKt82eNWzaf9HxCQ5QZyzGu1XEUZJwmmrJ6ZztmNeR/Wq6L/hlqhGzAOJM
blR1xIdawARu2G0CRMwqaj2RoQuthLzb8w==
-----END CERTIFICATE-----
expiration          1735657876
issuing_ca          -----BEGIN CERTIFICATE-----
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
private_key         -----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAuCN4ZxiOVuB68F0KM9fJIE5hu6TpMBpbVFy4kb2KpzDmDO+f
FjFXUA6YTaPo/SgFMyIS/hDbZD3P61Vey4foSr8OVkUQJEUcBM8I8nMyg8J9oh8O
bNl3Qiic8ksgBy5Kjned65hEIbRHRFjij7rHrh1pNdW4wgmBCqQ0KswVW6bJnlaE
9zBtfkDksHciVR2VdeN+gIe1CqcKwFnaHvu1s0JMMhUbqYg7LTEjOZbkSdly2EIA
05Mvd58V7ycTucRgQ09VUSpfUp7N5Q4cxNnyORDbCJvWBu+jxA97+UuwsXrAhq7Y
8PGo63Yb9r2JbArX3ojscXc3m8+dB2hORi0pZwIDAQABAoIBADtA5sC+LSeVqtno
Bp1yJb1om5iHU6ZwBM2b3KTBSnnMiWrGPPomPIN9ftMVGKdGFo5Cu7vX7tFN9rcy
zINQI5bR7ioipTQWrRJ7ENT77thpYIYn2jt6qx619PMe65qD8efwY/fpEpuJ6Jj8
xUMdBp5nxnBVatO9vTGQb10KOSE5ecyAZ7sBITC7eqCMFcgzM3m2zjnWbCsdlJj1
WG79W427NbxB4MNxjFCXSsXPNV207gDvFyTJK1J3OFJq2PyNcRZkWU7AOpO0Ygx9
g5A8szCN0YjkW2QMtVmVQnWS+XbZkYRi8hetRdwe9M+2aVn6swMUdKym/6yzolag
RDa1rvkCgYEA3Pgl0//X0MIZh63UiWUWeZi3+HtT5BwjVb6ccGfezF9l6YN/0fwm
+xTgcP8mmpmfJLt/D09lA/9O/sRD+wDrBXIdMmOPVGf3VwkySMBhUH/EtGctjewU
XtyfFYaSAkbkD6vDlT3OiJ0w+9XXUaJaM5i44mWs6y2/YZgM0HHNl50CgYEA1VST
t/pC3m7eWtZLdZ8oYsf75x/v00rSnT6CMapSKKTIU32ItiXRzm+ApBXHa2+YkXYa
hox3rrWe2eHIpyHe6Br4uPQFSqW3k2YzTSfm34t495VOIlLEhySpMrzm024ub7e4
6wdadiXvK3uGzesrOMKPTgX3q5dnutS5HBMDD9MCgYEAyYqv/ggJUQfofz8Gbna8
JBYuHj5mStV7SRa82y1yIhgU/QKKj/0blMD64TVngXUCmV9GSbGRoi64X1il5Id2
1RW7GZ2DOmpFR6ZEreSCHgkbYawF+b9M6STzGJAQFnGQS9bPYgzoluRArEHjzTp2
aT8vypcQO8UTHLGxZmGWMmUCgYEAsYpX/c9Lg27lotehqVwx8jPZUzrjDwfATJlP
JSJIigbJqaJZ+q1y9MkbWHO/qYwQf065OK0CleYVM+OSaHXp22VHBjYfiUZth0CR
BW9l1zluDS62/h2/7XD3V4Ca4e9auiM+xGs0QAvGBnwhbpJ/QBe7yAVzX9z7uSN8
gv7Xl30CgYA0ZSIsVpN8ih3drpGZrtV53zo6NBTDKemHGzgwFvBKbKxiGLQOtrLE
ragwsCFdCtGK1CkDsQgD5+PIHY2yf6FJoa0qYTUo+pM11hUOy3B3E0iXrMNgUICY
m01cuTFQ7mJD/HYfuRzEqsbwZyRlhvgS8cs+6mZRUX3Fd1qLV6nz5A==
-----END RSA PRIVATE KEY-----
private_key_type    rsa
serial_number       2a:33:ac:8a:02:d3:4a:8d:2c:e0:e8:68:50:bb:fe:d7:fa:92:1d:9a
```

Stronghold has now generated a new set of credentials using the `example-dot-com`
role configuration. Here we see the dynamically generated private key and
certificate.

Using ACLs, it is possible to restrict using the pki backend such that trusted
operators can manage the role definitions, and both users and applications are
restricted in the credentials they are allowed to read.

If you get stuck at any time, simply run `d8 stronghold path-help pki` or with a
subpath for interactive help output.

## API

The PKI secrets engine has a full HTTP API. Please see the
[PKI secrets engine API](/api-docs/secret/pki) for more
details.
