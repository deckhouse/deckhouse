---
title: "The cert-manager module: FAQ"
---

{% raw %}

## How do I check the certificate status?

```console
# kubectl -n default describe certificate example-com
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

```console
# kubectl get certificate --all-namespaces
NAMESPACE          NAME                            AGE
default            example-com                     13m
```

## What types of certificates are supported?

Currently the module installs the following four ClusterIssuers:
* letsencrypt
* letsencrypt-staging
* selfsigned
* selfsigned-no-trust

## Does the legacy tls-acme annotation work?

Yes, it works! The dedicated component (`cert-manager-ingress-shim`) automatically creates `Certificate` resources based on these annotations (in the same namespaces as those of Ingress resources with annotations).

> **Caution!** The Certificate for a particular annotation is linked to the existing Ingress resource. The additional records are put into the existing Ingress resource instead of creating a separate one. Thus, the process will fail if authentication or whitelist is set for the primary Ingress. In this case, you shouldn't use the annotation; use the Certificate instead.
>
> **Caution!** If you switched to the Certificate instead of annotation, then you need to delete the annotation-based Certificate. Otherwise, the same Secret will be updated for both Certificates (this may lead to exceeding the Let's Encrypt limits).

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    kubernetes.io/tls-acme: "true"           # here is the annotation!
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
  - host: www.example.com                    # the additional domain
    http:
      paths:
      - backend:
          service:
            name: site
            port:
              number: 80
        path: /
        pathType: ImplementationSpecific
  - host: admin.example.com                  # another additional domain
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
    - www.example.com                        # the additional domain
    - admin.example.com                      # another additional domain
    secretName: example-com-tls              # the name of the certificate & secret
```

## The "CAA record does not match issuer" error

Suppose `cert-manager` gets the following error when trying to provide a certificate:

```text
CAA record does not match issuer
```

In this case, you have to check the `CAA (Certificate Authority Authorization)` DNS record of the domain for which the certificate is intended. For Let's Encrypt certificates, the domain must have the `issue "letsencrypt.org"` CAA record. You can read more about CAA [here](https://www.xolphin.com/support/Terminology/CAA_DNS_Records) and [here](https://letsencrypt.org/docs/caa/).

## Vault integration

You can use [this manual](https://learn.hashicorp.com/tutorials/vault/kubernetes-cert-manager?in=vault/kubernetes) for configuring certificate issuance using Vault.

After configuring PKI and enabling Kubernetes [authorization](../../modules/140-user-authz/), you have to:
- Create a service account and copy its secret reference:

  ```shell
  kubectl create serviceaccount issuer
  ISSUER_SECRET_REF=$(kubectl get serviceaccount issuer -o json | jq -r ".secrets[].name")
  ```

- Create an Issuer:

  ```shell
  kubectl apply -f - <<EOF
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
  kubectl apply -f - <<EOF
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

## How do I use a custom or interim CA to issue certificates?

Follow the steps below to use a custom or interim CA:

- Generate a certificate (if necessary):

  ```shell
  openssl genrsa -out rootCAKey.pem 2048
  openssl req -x509 -sha256 -new -nodes -key rootCAKey.pem -days 3650 -out rootCACert.pem
  ```

- In the `d8-cert-manager` namespace, create a secret containing certificate file data.

  An example of creating a secret with kubectl:

  ```shell
  kubectl create secret tls internal-ca-key-pair -n d8-cert-manager --key="rootCAKey.pem" --cert="rootCACert.pem"
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

- Create a ClusterIssuer using the secret you created earlier:

  ```yaml
  apiVersion: cert-manager.io/v1
  kind: ClusterIssuer
  metadata:
    name: inter-ca
  spec:
    ca:
      secretName: internal-ca-key-pair    # Name of the secret you created earlier.
  ```

  You can use any name as your ClusterIssuer name.

You can now use the created ClusterIssuer to issue certificates for all Deckhouse components or a particular component.

For example, to issue certificates for all Deckhouse components, specify the ClusterIssuer name in the [clusterIssuerName](../../deckhouse-configure-global.html#parameters-modules-https-certmanager-clusterissuername) global parameter (`kubectl edit mc global`):

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

## How to secure cert-manager credentials?

If you don't want to store credentials in the Deckhouse configuration (security reasons, for example), feel free to create
your own ClusterIssuer / Issuer.
For example, you can create your own ClusterIssuer for a [route53](https://aws.amazon.com/route53/) service in this way:
- Create a Secret with credentials:

  ```shell
  kubectl apply -f - <<EOF
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

- Create a simple ClusterIssuer with reference to that secret:

  ```shell
  kubectl apply -f - <<EOF
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

- Order certificates as usual, using created ClusterIssuer:

  ```shell
  kubectl apply -f - <<EOF
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

{% endraw %}
