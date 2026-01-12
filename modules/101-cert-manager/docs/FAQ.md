---
title: "The cert-manager module: FAQ"
---

## What types of certificates are supported?

The module installs the following ClusterIssuers:
* `letsencrypt`
* `letsencrypt-staging`
* `selfsigned`
* `selfsigned-no-trust`

If you need support for other types of certificates, you can add them yourself.

## How to add an additional `ClusterIssuer`?

### When is an additional `ClusterIssuer` required?

The standard delivery set includes `ClusterIssuer` that issues certificates from the trusted public certificate authority Let's Encrypt or issues self-signed certificates.

To issue certificates for a domain name via Let's Encrypt, the service requires that you verify domain ownership.
The `cert-manager` supports several methods for verifying domain ownership when using `ACME` (Automated Certificate Management Environment):
* `HTTP-01` - when using this method `cert-manager` will create a temporary Pod in the cluster that will listen on a specific URL to verify domain ownership. For it to work, you must be able to direct external traffic to this Pod, usually via `Ingress`.
* `DNS-01` - when using this method `cert-manager` makes a TXT record in DNS to verify domain ownership. The `cert-manager` has built-in support for popular DNS providers: AWS Route53, Google Cloud DNS, Cloudflare, and others.

{% alert level="danger" %}
The `HTTP-01` method does not support issuing wildcard certificates.
{% endalert %}

The `ClusterIssuers` in standard delivery set that issue certificates via Let's Encrypt are divided into two types:

1. `ClusterIssuer` specific to the cloud provider being used.

   Added automatically when filling in the [module settings](./configuration.html) associated with the cloud provider.  
   These `ClusterIssuers` support the `DNS-01` method.
   * `clouddns`
   * `cloudflare`
   * `digitalocean`
   * `route53`
1. `ClusterIssuers` using the `HTTP-01` method.
  
   Added automatically unless their creation is disabled in the [module settings](./configuration.html#parameters-disableletsencrypt).
   * `letsencrypt`
   * `letsencrypt-staging`

In this way, an additional `ClusterIssuer` may be required in the following cases:
1. Certificates need to be issued in a CA other than Let's Encrypt (including a private one). Supported CAs are available [in the `cert-manager` documentation](https://cert-manager.io/docs/configuration/)
2. Certificates need to be issued via Let's Encrypt using the `DNS-01` method via a third-party provider.

### How to add an additional `ClusterIssuer` using Let's Encrypt and `DNS-01` verification method?

To verify domain ownership via Let's Encrypt using the `DNS-01` method, you need to configure the ability to create TXT records in a public DNS.

`cert-manager` has support for mechanisms for creating TXT records in some popular DNS: `AzureDNS`, `Cloudflare`, `Google Cloud DNS`, etc.  
The full list is available [in the `cert-manager` documentation](https://cert-manager.io/docs/configuration/acme/dns01/).

The module automatically creates `ClusterIssuer` of supported cloud providers when filling in the module settings related to the cloud used.  
If necessary, you can create such `ClusterIssuer` yourself.

An example of using AWS Route53 is available in the section [How to protect `cert-manager` credentials](#how-to-secure-cert-manager-credentials).  
The list of all possible `ClusterIssuer`s that can be created is available in the [module templates](https://github.com/deckhouse/deckhouse/tree/main/modules/101-cert-manager/templates/cert-manager).

Using third-party DNS providers is implemented via the `webhook` method.  

When cert-manager makes an `ACME` `DNS-01` call, it sends a request to the webhook server, which then performs the necessary operations to update the DNS record.  
When using this method, you need to place a service that will process the hook and create a TXT record in the DNS provider.  

As an example, let's consider using the `Yandex Cloud DNS` service.

1. To process the webhook, first place the `Yandex Cloud DNS ACME webhook` service in the cluster according to the [official documentation](https://github.com/yandex-cloud/cert-manager-webhook-yandex)  

1. Then, create the `ClusterIssuer` resource:

   ```yaml
   apiVersion: cert-manager.io/v1
   kind: ClusterIssuer
   metadata:
     name: yc-clusterissuer
     namespace: default
   spec:
     acme:
       # You must replace this email address with your own.
       # Let's Encrypt will use this to contact you about expiring
       # certificates, and issues related to your account.
       email: your@email.com
       server: https://acme-staging-v02.api.letsencrypt.org/directory
       privateKeySecretRef:
         # Secret resource that will be used to store the account's private key.
         name: secret-ref
       solvers:
         - dns01:
             webhook:
               config:
                 # The ID of the folder where dns-zone located in
                 folder: <your folder ID>
                 # This is the secret used to access the service account
                 serviceAccountSecretRef:
                   name: cert-manager-secret
                   key: iamkey.json
               groupName: acme.cloud.yandex.com
               solverName: yandex-cloud-dns
   ```

## How to add an additional `Issuer` and `ClusterIssuer` using HashiCorp Vault to issue certificates?

You can use [this manual](https://learn.hashicorp.com/tutorials/vault/kubernetes-cert-manager?in=vault/kubernetes) for configuring certificate issuance using Vault.

After configuring PKI and enabling Kubernetes [authorization](../../modules/user-authz/), you have to:
- Create a service account and copy its secret reference:

  ```shell
  d8 k create serviceaccount issuer
  ISSUER_SECRET_REF=$(d8 k get serviceaccount issuer -o json | jq -r ".secrets[].name")
  ```

- Create an Issuer:

  ```shell
  d8 k apply -f - <<EOF
  apiVersion: cert-manager.io/v1
  kind: Issuer
  metadata:
    name: vault-issuer
    namespace: default
  spec:
    vault:
      # HashiCorp instruction has mistype here
      server: http://vault.default.svc.cluster.local:8200 
      path: pki/sign/example-dot-com # configure in pki setup step
      auth:
        kubernetes:
          mountPath: /v1/auth/kubernetes
          role: issuer
          secretRef:
            name: $ISSUER_SECRET_REF
            key: token
  EOF
  ```

- Create a Certificate resource, to get a TLS certificate, which is issued by Vault CA:

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
    # domains are set on PKI setup
    commonName: www.example.com 
    dnsNames:
    - www.example.com
  EOF
  ```

## How to add `ClusterIssuer` using its own or intermediate CA to to issue certificates?

Follow the steps below to use a custom or interim CA:

- Generate a certificate (if necessary):

  ```shell
  openssl genrsa -out rootCAKey.pem 2048
  openssl req -x509 -sha256 -new -nodes -key rootCAKey.pem -days 3650 -out rootCACert.pem
  ```

- In the `d8-cert-manager` namespace, create a secret containing certificate file data.

  An example of creating a secret with d8 k:

  ```shell
  d8 k create secret tls internal-ca-key-pair -n d8-cert-manager --key="rootCAKey.pem" --cert="rootCACert.pem"
  ```

  An example of creating a secret from a YAML file (the contents of the certificate files must be Base64-encoded):

  ```yaml
  apiVersion: v1
  data:
    tls.crt: <OUTPUT OF `cat rootCACert.pem | base64 -w0`>
    tls.key: <OUTPUT OF `cat rootCAKey.pem | base64 -w0`>
  kind: Secret
  metadata:
    name: internal-ca-key-pair
    namespace: d8-cert-manager
  type: Opaque
  ```

  You can use any name you like for the secret.

- Create a `ClusterIssuer` using the secret you created earlier:

  ```yaml
  apiVersion: cert-manager.io/v1
  kind: ClusterIssuer
  metadata:
    name: inter-ca
  spec:
    ca:
      secretName: internal-ca-key-pair    # Name of the secret you created earlier.
  ```

  You can use any name as your `ClusterIssuer` name.

You can now use the created `ClusterIssuer` to issue certificates for all Deckhouse components or a particular component.

For example, to issue certificates for all Deckhouse components, specify the `ClusterIssuer` name in the [ClusterIssuerName](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-https-certmanager-clusterissuername) global parameter (`d8 k edit mc global`):

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

## How to secure `cert-manager` credentials?

If you don't want to store credentials in the Deckhouse configuration (security reasons, for example), feel free to create
your own ClusterIssuer / Issuer.
For example, you can create your own `ClusterIssuer` for a [route53](https://aws.amazon.com/route53/) service in this way:
- Create a Secret with credentials:

  ```shell
  d8 k apply -f - <<EOF
  apiVersion: v1
  kind: Secret
  type: Opaque
  metadata:
    name: route53
    namespace: default
  data:
    secret-access-key: {{ "MY-AWS-ACCESS-KEY-TOKEN" | b64enc | quote }}
  EOF
  ```

- Create a simple `ClusterIssuer` with reference to that secret:

  ```shell
  d8 k apply -f - <<EOF
  apiVersion: cert-manager.io/v1
  kind: ClusterIssuer
  metadata:
    name: route53
    namespace: default
  spec:
    acme:
      server: https://acme-v02.api.letsencrypt.org/directory
      privateKeySecretRef:
        name: route53-tls-key
      solvers:
      - dns01:
          route53:
            region: us-east-1
            accessKeyID: {{ "MY-AWS-ACCESS-KEY-ID" }}
            secretAccessKeySecretRef:
              name: route53
              key: secret-access-key
  EOF
  ```

- Order certificates as usual, using created `ClusterIssuer`:

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
      name: route53
    commonName: www.example.com 
    dnsNames:
    - www.example.com
  EOF
  ```

## Does the legacy tls-acme annotation work?

Yes, it works! The dedicated component (`cert-manager-ingress-shim`) automatically creates `Certificate` resources based on these annotations (in the same namespaces as those of Ingress resources with annotations).
HashiCorp
> **Caution!** The Certificate for a particular annotation is linked to the existing Ingress resource. The additional records are put into the existing Ingress resource instead of creating a separate one. Thus, the process will fail if authentication or whitelist is set for the primary Ingress. In this case, you shouldn't use the annotation; use the Certificate instead.
>
> **Caution!** If you switched to the Certificate instead of annotation, then you need to delete the annotation-based Certificate. Otherwise, the same Secret will be updated for both Certificates (this may lead to exceeding the Let's Encrypt limits).

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    kubernetes.io/tls-acme: "true"           # The annotation.
  name: example-com
  namespace: default
spec:
  ingressClassName: nginx
  rules:
  - host: example.com
    http:
      paths:
      - backend:
          service:
            name: site
            port:
              number: 80
        path: /
        pathType: ImplementationSpecific
  - host: www.example.com                    # The additional domain
    http:
      paths:
      - backend:
          service:
            name: site
            port:
              number: 80
        path: /
        pathType: ImplementationSpecific
  - host: admin.example.com                  # Another additional domain
    http:
      paths:
      - backend:
          service:
            name: site
            port:
              number: 80
        path: /
        pathType: ImplementationSpecific
  tls:
  - hosts:
    - example.com
    - www.example.com                        # The additional domain
    - admin.example.com                      # Another additional domain
    secretName: example-com-tls              # The name of the certificate and secret
```

## How do I check the certificate status?

```shell
d8 k -n default describe certificate example-com
...
Status:
  Acme:
    Authorizations:
      Account:  https://acme-v01.api.letsencrypt.org/acme/reg/22442061
      Domain:   example.com
      Uri:      https://acme-v01.api.letsencrypt.org/acme/challenge/qJA9MGCZnUnVjAgxhoxONvDnKAsPatRILJ4n0lJ7MMY/4062050823
      Account:  https://acme-v01.api.letsencrypt.org/acme/reg/22442061
      Domain:   admin.example.com
      Uri:      https://acme-v01.api.letsencrypt.org/acme/challenge/pW2tFKLBDTll2Gx8UBqmEl846x5W-YpBs8a4HqstJK8/4062050808
      Account:  https://acme-v01.api.letsencrypt.org/acme/reg/22442061
      Domain:   www.example.com
      Uri:      https://acme-v01.api.letsencrypt.org/acme/challenge/LaZJMM9_OKcTYbEThjT3oLtwgpkNfbHVdl8Dz-yypx8/4062050792
  Conditions:
    Last Transition Time:  2018-04-02T18:01:04Z
    Message:               Certificate issued successfully
    Reason:                CertIssueSuccess
    Status:                True
    Type:                  Ready
Events:
  Type     Reason                 Age                 From                     Message
  ----     ------                 ----                ----                     -------
  Normal   PrepareCertificate     1m                cert-manager-controller  Preparing certificate with issuer
  Normal   PresentChallenge       1m                cert-manager-controller  Presenting http-01 challenge for domain example.com
  Normal   PresentChallenge       1m                cert-manager-controller  Presenting http-01 challenge for domain www.example.com
  Normal   PresentChallenge       1m                cert-manager-controller  Presenting http-01 challenge for domain admin.example.com
  Normal   SelfCheck              1m                cert-manager-controller  Performing self-check for domain admin.example.com
  Normal   SelfCheck              1m                cert-manager-controller  Performing self-check for domain example.com
  Normal   SelfCheck              1m                cert-manager-controller  Performing self-check for domain www.example.com
  Normal   ObtainAuthorization    55s               cert-manager-controller  Obtained authorization for domain example.com
  Normal   ObtainAuthorization    54s               cert-manager-controller  Obtained authorization for domain admin.example.com
  Normal   ObtainAuthorization    53s               cert-manager-controller  Obtained authorization for domain www.example.com
```

## How do I get a list of certificates?

```shell
d8 k get certificate --all-namespaces
NAMESPACE          NAME                            AGE
default            example-com                     13m
```

## The "CAA record does not match issuer" error

Suppose `cert-manager` gets the following error when trying to provide a certificate:

```text
CAA record does not match issuer
```

In this case, you have to check the `CAA (Certificate Authority Authorization)` DNS record of the domain for which the certificate is intended. For Let's Encrypt certificates, the domain must have the `issue "letsencrypt.org"` CAA record. You can read more about CAA [in the Let's Encrypt documentation](https://letsencrypt.org/docs/caa/).
