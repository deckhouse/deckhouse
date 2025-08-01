---
title: Certificate management
permalink: en/admin/configuration/security/certificates.html
---

Deckhouse Kubernetes Platform (DKP) provides built-in tools for managing TLS certificates in the cluster and supports:

- requesting certificates from all supported sources, such as [Let’s Encrypt](https://letsencrypt.org/), [HashiCorp Vault](https://developer.hashicorp.com/vault), [Venafi](https://docs.venafi.com/);
- issuing self-signed certificates;
- automatic renewal and expiration monitoring;
- deploying `cm-acme-http-solver` on master nodes and dedicated nodes.

This page describes the available certificate management features in DKP and the procedure for working with certificate issuers.

{% alert level="info" %}
Examples of certificate configuration, usage of the `tls-acme` annotation, and secure handling of credentials are described [on the "Using TLS Certificates" page](../../../user/tls.html).
{% endalert %}

## Monitoring

DKP exports metrics to Prometheus, allowing you to monitor:

- certificate expiration dates;
- certificate renewal status.

## Access roles

DKP provides several predefined roles for accessing resources:

| Role           | Access rights |
|----------------|---------------|
| `User`         | View `Certificate` and `Issuer` resources in available namespaces, as well as global `ClusterIssuer` resources. |
| `Editor`       | Manage `Certificate` and `Issuer` resources in available namespaces. |
| `ClusterEditor`| Manage `Certificate` and `Issuer` resources in all namespaces. |
| `SuperAdmin`   | Manage internal system objects. |

## Working with certificate issuers

DKP supports the following default certificate issuers (`ClusterIssuer`):

- `letsencrypt` — issues TLS certificates using the public CA Let’s Encrypt and ACME HTTP validation.  
  Suitable for automatically obtaining trusted certificates for most public services.  
  Configuration details can be found in the [official `cert-manager` documentation](https://cert-manager.io/docs/configuration/acme/).

- `letsencrypt-staging` — similar to `letsencrypt`, but uses Let’s Encrypt’s staging server.  
  Useful for debugging configuration and testing the certificate issuance process.  
  More details available in the [Let’s Encrypt staging docs](https://letsencrypt.org/docs/staging-environment/).

- `selfsigned` — issues self-signed certificates.  
  Useful for internal services where external trust is not required.

- `selfsigned-no-trust` — also issues self-signed certificates but without automatically adding the root certificate to the trust store. Suitable for manual trust management.

In some cases, additional types of `ClusterIssuer` may be needed:

- if you want to use a Let’s Encrypt certificate with DNS validation via a third-party DNS provider;
- if you need to use a certificate authority (CA) other than Let’s Encrypt.  
  See the full list of supported CAs in the [`cert-manager` documentation](https://cert-manager.io/docs/configuration/issuers/).

### Adding a ClusterIssuer with `DNS-01` validation via webhook

To verify domain ownership with Let’s Encrypt using the `DNS-01` method, the `cert-manager` module must be able to create TXT records in the DNS zone associated with the domain.

`cert-manager` includes built-in support for popular DNS providers such as AWS Route53, Google Cloud DNS, Cloudflare, and others.  
A full list is available in the [official `cert-manager` documentation](https://cert-manager.io/docs/configuration/acme/dns01/).

If your provider is not directly supported, you can configure a webhook and deploy a custom ACME handler in the cluster that performs the necessary DNS record updates.

The following example is based on using Yandex Cloud DNS:

1. To handle the webhook, deploy the `Yandex Cloud DNS ACME webhook` service in the cluster according to the [official documentation](https://github.com/yandex-cloud/cert-manager-webhook-yandex).

1. Create a ClusterIssuer resource using the following example:

   ```yaml
   apiVersion: cert-manager.io/v1
   kind: ClusterIssuer
   metadata:
     name: yc-clusterissuer
     namespace: default
   spec:
     acme:
       # Replace this email address with your own.
       # Let’s Encrypt will use it to notify you about expiring certificates
       # and issues related to your account.
       email: your@email.com
       server: https://acme-staging-v02.api.letsencrypt.org/directory
       privateKeySecretRef:
         # The Secret resource used to store the account private key.
         name: secret-ref
       solvers:
         - dns01:
             webhook:
               config:
                 # The folder ID containing your DNS zone.
                 folder: <your-folder-ID>
                 # Secret used to access the service account.
                 serviceAccountSecretRef:
                   name: cert-manager-secret
                   key: iamkey.json
               groupName: acme.cloud.yandex.com
               solverName: yandex-cloud-dns
   ```

### Adding a ClusterIssuer that uses a custom certificate authority (CA)

1. Generate the certificate:

   ```shell
   openssl genrsa -out rootCAKey.pem 2048
   openssl req -x509 -sha256 -new -nodes -key rootCAKey.pem -days 3650 -out rootCACert.pem
   ```

1. Create a secret in the `d8-cert-manager` namespace with an arbitrary name, containing the certificate files.

   - Example of creating a secret using the `d8 k` command:

     ```shell
     d8 k create secret tls internal-ca-key-pair -n d8-cert-manager --key="rootCAKey.pem" --cert="rootCACert.pem"
     ```

   - Example of creating a secret from a YAML file (the certificate file contents must be base64-encoded):

     ```yaml
     apiVersion: v1
     data:
       tls.crt: <output of `cat rootCACert.pem | base64 -w0`>
       tls.key: <output of `cat rootCAKey.pem | base64 -w0`>
     kind: Secret
     metadata:
       name: internal-ca-key-pair
       namespace: d8-cert-manager
     type: Opaque
     ```

1. Create a ClusterIssuer with any name using the created secret:

   ```yaml
   apiVersion: cert-manager.io/v1
   kind: ClusterIssuer
   metadata:
     name: inter-ca
   spec:
     ca:
       secretName: internal-ca-key-pair    # Name of the created secret.
   ```

You can now use the created ClusterIssuer to issue certificates for all DKP components or for a specific component.

For example, to use this ClusterIssuer for issuing certificates for all DKP components, set its name in the global parameter `clusterIssuerName`:

```yaml
  spec:
    settings:
      modules:
        https:
          certManager:
            clusterIssuerName: inter-ca
          mode: CertManager
        publicDomainTemplate: '%s.<public_domain_template>'
    version: 1
```

### Adding an Issuer and ClusterIssuer using HashiCorp Vault to request certificates

To configure certificate issuance using Vault, refer to the [HashiCorp documentation](https://developer.hashicorp.com/vault/tutorials/archive/kubernetes-cert-manager?in=vault%2Fkubernetes).

After configuring the PKI and [enabling Kubernetes authentication](../access/authorization/), follow these steps:

1. Create a ServiceAccount and copy the reference to its secret:

   ```shell
   d8 k create serviceaccount issuer
     
   ISSUER_SECRET_REF=$(d8 k get serviceaccount issuer -o json | jq -r ".secrets[].name")
   ```

1. Create the Issuer resource:

   ```yaml
   d8 k apply -f - <<EOF
   apiVersion: cert-manager.io/v1
   kind: Issuer
   metadata:
     name: vault-issuer
     namespace: default
   spec:
     vault:
       server: http://vault.default.svc.cluster.local:8200
       # Defined during PKI configuration.
       path: pki/sign/example-dot-com 
       auth:
         kubernetes:
           mountPath: /v1/auth/kubernetes
           role: issuer
           secretRef:
             name: $ISSUER_SECRET_REF
             key: token
   EOF
   ```

1. Create the Certificate resource to obtain a TLS certificate signed by Vault CA:

   ```yaml
   d8 k apply -f - <<EOF
   apiVersion: cert-manager.io/v1
   kind: Certificate
   metadata:
     name: example-com
     namespace: default
   spec:
     secretName: example-com-tls
     issuerRef:
       name: vault-issuer
     # Domains must match those configured in the PKI in Vault.
     commonName: www.example.com 
     dnsNames:
     - www.example.com
   EOF
   ```

## Self-signed certificate generation

When generating certificates manually, it is important to correctly fill in all fields of the certificate signing request to ensure the resulting certificate is valid and successfully passes validation across different services.

Follow these guidelines:

1. Specify domain names in the `SAN` (Subject Alternative Name) field.

   The `SAN` field is a more modern and widely used method for specifying domain names covered by the certificate.  
   Some services no longer consider the `CN` (Common Name) field as a source for domain names.

1. Set the `keyUsage`, `basicConstraints`, and `extendedKeyUsage` fields appropriately:

   - `basicConstraints = CA:FALSE`  

     This field defines whether the certificate belongs to an end-entity (end-entity certificate) or a certificate authority (CA certificate).  
     A CA certificate must not be used as a service certificate.

   - `keyUsage = digitalSignature, keyEncipherment`  

     The `keyUsage` field restricts how the key can be used:

     - `digitalSignature` — allows using the key for signing messages and ensuring connection integrity.
     - `keyEncipherment` — allows using the key for encrypting other keys, which is essential for secure data exchange via TLS (Transport Layer Security).

   - `extendedKeyUsage = serverAuth`  

     This field defines additional usage scenarios for the key that might be required by specific protocols or applications:

     - `serverAuth` — indicates that the certificate is intended for use on a server for server-side authentication during secure connection establishment.

Additionally, it is recommended to:

1. Issue the certificate for no more than 1 year (365 days).

   The certificate's validity period affects its security.  
   A 1-year term helps keep cryptographic standards up-to-date and ensures timely certificate rotation if any vulnerabilities arise.  
   Some modern browsers currently reject certificates valid for more than 1 year.

1. Use strong cryptographic algorithms, such as elliptic curve-based algorithms (e.g., `prime256v1`).

   Elliptic Curve Cryptography (ECC) provides high levels of security with smaller key sizes compared to traditional methods like RSA.  
   This makes ECC-based certificates more efficient and secure over time.

1. Avoid specifying domains in the `CN` (Common Name) field.

   Historically, the `CN` field was used to specify the primary domain name for the certificate.  
   However, modern standards, such as [RFC 2818](https://datatracker.ietf.org/doc/html/rfc2818), recommend using the `SAN` field instead.  
   If the certificate includes multiple domains via `SAN`, but only one is listed in `CN`, some services may fail validation when accessing domains not mentioned in the `CN`.  
   Including non-domain information (e.g., a service name or ID) in the `CN` may unintentionally extend the certificate's scope and create security risks.

### Example: generating a certificate

To generate a certificate, use the `openssl` utility.

1. Create a configuration file named `cert.cnf`:

   ```ini
   [ req ]
   default_bits       = 2048
   default_md         = sha256
   prompt             = no
   distinguished_name = dn
   req_extensions     = req_ext

   [ dn ]
   C = RU
   ST = Moscow
   L = Moscow
   O = Example Company
   OU = IT Department
   # CN = Do not specify the CN field.

   [ req_ext ]
   subjectAltName = @alt_names

   [ alt_names ]
   # List all domain names.
   DNS.1 = example.com
   DNS.2 = www.example.com
   DNS.3 = api.example.com
   # Add IP addresses if needed.
   IP.1 = 192.0.2.1
   IP.2 = 192.0.4.1

   [ v3_ca ]
   basicConstraints = CA:FALSE
   keyUsage = digitalSignature, keyEncipherment
   extendedKeyUsage = serverAuth

   [ v3_req ]
   basicConstraints = CA:FALSE
   keyUsage = digitalSignature, keyEncipherment
   extendedKeyUsage = serverAuth
   subjectAltName = @alt_names

   # Elliptic curve parameters.
   [ ec_params ]
   name = prime256v1
   ```

1. Generate a private key using elliptic curve cryptography:

   ```shell
   openssl ecparam -genkey -name prime256v1 -noout -out ec_private_key.pem
   ```

1. Create a certificate signing request (CSR):

   ```shell
   openssl req -new -key ec_private_key.pem -out example.csr -config cert.cnf
   ```

1. Generate a self-signed certificate:

   ```shell
   openssl x509 -req -in example.csr -signkey ec_private_key.pem -out example.crt -days 365 -extensions v3_req -extfile cert.cnf
   ```
