---
title: Certificate management
permalink: en/virtualization-platform/documentation/admin/platform-management/security/certificates.html
---

Deckhouse Virtualization Platform (DVP) provides built-in tools for managing TLS certificates in the cluster and supports:

- Requesting certificates from all supported sources, such as [Let's Encrypt](https://letsencrypt.org/), [HashiCorp Vault](https://developer.hashicorp.com/vault), [Venafi](https://docs.venafi.com/).
- Issuing self-signed certificates.
- Automatic renewal and expiration monitoring.
- Deploying `cm-acme-http-solver` on master nodes and dedicated nodes.

This page describes the available certificate management features in DVP
and the procedure for working with certificate issuers.

{% alert level="info" %}
Examples of certificate configuration, usage of the `tls-acme` annotation,
and secure handling of credentials are described in [Using TLS certificates](/products/virtualization-platform/documentation/user/security/tls.html).
{% endalert %}

## Monitoring

DVP exports metrics to Prometheus, allowing you to monitor:

- Certificate expiration dates
- Certificate renewal status

## Access roles

DVP provides several predefined roles for accessing resources:

| Role           | Permissions |
|----------------|---------------|
| `User`         | View Certificate and Issuer resources in available namespaces, as well as global ClusterIssuer resources. |
| `Editor`       | Manage Certificate and Issuer resources in available namespaces. |
| `ClusterEditor`| Manage Certificate and Issuer resources in all namespaces. |
| `SuperAdmin`   | Manage internal system objects. |

## Working with certificate issuers

DVP supports the following default certificate issuers (ClusterIssuer):

- `letsencrypt`: Issues TLS certificates using the public CA Let's Encrypt and ACME HTTP validation.
  Suitable for automatically obtaining trusted certificates for most public services.
  Configuration details can be found in the [official `cert-manager` documentation](https://cert-manager.io/docs/configuration/acme/).

- `letsencrypt-staging`: Similar to `letsencrypt`, but uses Let's Encrypt's staging server.
  Useful for debugging configuration and testing the certificate issuance process.
  More details available in the [Let's Encrypt staging docs](https://letsencrypt.org/docs/staging-environment/).

- `selfsigned`: Issues self-signed certificates.
  Useful when external trust is not required (for example, for internal services).

- `selfsigned-no-trust`: Also issues self-signed certificates
  but without automatically adding the root certificate to the trust store.
  Suitable for manual trust management.

In some cases, additional types of ClusterIssuer may be needed:

- If you want to use a Let's Encrypt certificate with DNS validation via a third-party DNS provider.
- if you need to use a certificate authority (CA) other than Let's Encrypt.
  See the full list of supported CAs in the [`cert-manager` documentation](https://cert-manager.io/docs/configuration/issuers/).

### Adding a ClusterIssuer with DNS-01 validation via webhook

To verify domain ownership with Let's Encrypt using the `DNS-01` method,
the `cert-manager` module must be able to create TXT records in the DNS zone associated with the domain.

`cert-manager` includes built-in support for popular DNS providers such as AWS Route53, Google Cloud DNS, Cloudflare, and others.
A full list is available in the [official `cert-manager` documentation](https://cert-manager.io/docs/configuration/acme/dns01/).

If your provider is not directly supported, you can configure a webhook
and deploy a custom ACME handler in the cluster that performs the necessary DNS record updates.

The following example is based on using Yandex Cloud DNS:

1. To handle the webhook, deploy the `Yandex Cloud DNS ACME webhook` service in the cluster
   according to the [official documentation](https://github.com/yandex-cloud/cert-manager-webhook-yandex).

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
       # Let's Encrypt will use it to notify you about expiring certificates
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

   - Example of creating a secret from a YAML file (the certificate file contents must be Base64-encoded):

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

You can now use the created ClusterIssuer to issue certificates for all DVP components or for a specific component.

For example, to use this ClusterIssuer for issuing certificates for all DVP components,
set its name in the global parameter [`clusterIssuerName`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-https-certmanager-clusterissuername):

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

After configuring the PKI and enabling Kubernetes authentication, follow these steps:

1. Create a ServiceAccount and copy the reference to its secret:

   ```shell
   d8 k create serviceaccount issuer
     
   ISSUER_SECRET_REF=$(d8 k get serviceaccount issuer -o json | jq -r ".secrets[].name")
   ```

1. Create the Issuer resource:

   ```shell
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

   ```shell
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
