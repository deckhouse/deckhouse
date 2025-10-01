---
title: Using TLS certificates
permalink: en/user/security/tls.html
---

Deckhouse Kubernetes Platform (DKP) provides built-in tools for managing TLS certificates,
simplifying setup and management of traffic encryption for applications running in the cluster.

This page covers the following aspects of certificate usage in DKP:

- Manually requesting TLS certificates using the Certificate and ClusterIssuer resources.
- Securely storing and using credentials for accessing certificate authorities (CAs).
- Automatically obtaining certificates using the `tls-acme` annotation in Ingress resources.

{% alert level="info" %}
For a general overview of certificate management in DKP, the list of supported issuers, and setup recommendations,
refer to [Certificate management](../../admin/configuration/security/certificates.html).
{% endalert %}

## Working with certificates

### Viewing certificate details

- To list all certificates in the cluster, use the following command:

  ```shell
  d8 k get certificate --all-namespaces
  ```

- To check the status of a specific certificate, run:

  ```shell
  d8 k -n <NAMESPACE> describe certificate <CERTIFICATE-NAME>
  ```

### Automatic certificate requesting

To request a `letsencrypt` certificate, follow these steps:

1. Create a Certificate resource as described in the [`cert-manager` documentation](https://cert-manager.io/docs/usage/certificate/).
   Use the following example as a reference:

   ```yaml
   apiVersion: cert-manager.io/v1
   kind: Certificate
   metadata:
     name: example-com            # Certificate name.
     namespace: default
   spec:
     secretName: example-com-tls  # Name of the Secret that will store the private key and the certificate.
     issuerRef:
       kind: ClusterIssuer        # Certificate issuer details.
       name: letsencrypt
     commonName: example.com      # Certificate main domain.
     dnsNames:                    # Optional additional certificate domains (at least one DNS name or IP address).
     - www.example.com
     - admin.example.com
   ```

1. The `cert-manager` module will automatically run a domain ownership challenge
   using the method defined in the ClusterIssuer resource (for example, `HTTP-01` or `DNS-01`).
1. The `cert-manager` module will automatically create a temporary Ingress resource for the challenge.
   This temporary Ingress will not affect your existing Ingress configuration.
1. Once validated, the issued certificate will be stored in the Secret specified in the `secretName` field.

{% alert level="info" %}
If you see a `CAA record does not match issuer` error during certificate issuance, check the DNS records for the domain.
To use `letsencrypt`, your domain must have the following CAA record: `issue "letsencrypt.org"`.

For mode details on CAA records, refer to [Let's Encrypt documentation](https://letsencrypt.org/docs/caa/).
{% endalert %}

#### Requesting a wildcard certificate using DNS in Cloudflare

1. Obtain your `GlobalAPIKey` and `Email`:
   - Go to [`dash.cloudflare.com/profile`](https://dash.cloudflare.com/profile).
   - Your email is shown at the top under **Email Address**.
   - To view the key, click **View** next to **Global API Key** at the bottom.

1. Edit the [`cert-manager` module settings](/modules/cert-manager/configuration.html), adding the following section:

   ```yaml
   settings:
     cloudflareGlobalAPIKey: APIkey
     cloudflareEmail: some@mail.somedomain
   ```

   Or, use an [API token](https://cert-manager.io/docs/configuration/acme/dns01/cloudflare/#api-tokens) instead (recommended):

   ```yaml
   settings:
     cloudflareAPIToken: some-token
     cloudflareEmail: some@mail.somedomain
   ```

   DKP will automatically create a ClusterIssuer and Secret for Cloudflare in the `d8-cert-manager` namespace.

1. Create a Certificate resource using Cloudflare for DNS validation.
   This option becomes available only after the `cloudflareGlobalAPIKey` and `cloudflareEmail` parameters are configured:

   ```yaml
   apiVersion: cert-manager.io/v1
   kind: Certificate
   metadata:
     name: domain-wildcard
     namespace: app-namespace
   spec:
     secretName: tls-wildcard
     issuerRef:
       name: cloudflare
       kind: ClusterIssuer
     commonName: "*.domain.com"
     dnsNames:
     - "*.domain.com"
   ```

1. Create the Ingress resource:

   ```yaml
   apiVersion: networking.k8s.io/v1
   kind: Ingress
   metadata:
     name: domain-wildcard
     namespace: app-namespace
   spec:
     ingressClassName: nginx
     rules:
     - host: "*.domain.com"
       http:
         paths:
         - backend:
             service:
               name: svc-web
               port:
                 number: 80
           path: /
     tls:
     - hosts:
       - "*.domain.com"
       secretName: tls-wildcard
   ```

#### Requesting a wildcard certificate using DNS in AWS Route53

1. Create a user with the required permissions:

   - On the [policy management page](https://console.aws.amazon.com/iam/home?region=us-east-2#/policies), create a policy with the following permissions:

     ```json
     {
         "Version": "2012-10-17",
         "Statement": [
             {
                 "Effect": "Allow",
                 "Action": "route53:GetChange",
                 "Resource": "arn:aws:route53:::change/*"
             },
             {
                 "Effect": "Allow",
                 "Action": "route53:ChangeResourceRecordSets",
                 "Resource": "arn:aws:route53:::hostedzone/*"
             },
             {
                 "Effect": "Allow",
                 "Action": "route53:ListHostedZonesByName",
                 "Resource": "*"
             }
         ]
     }
     ```

   - Open the [user management page](https://console.aws.amazon.com/iam/home?region=us-east-2#/users) and create a user with the above policy.

1. Edit the [`cert-manager` module settings](/modules/cert-manager/configuration.html) to add the following section:

   ```yaml
   settings:
     route53AccessKeyID: AKIABROTAITAJMPASA4A
     route53SecretAccessKey: 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
   ```

   DKP will automatically create a ClusterIssuer and Secret for Route53 in the `d8-cert-manager` namespace.

1. Create a Certificate resource for validation with Route53.
   This option becomes available only after the `route53AccessKeyID` and `route53SecretAccessKey` parameters are configured:

   ```yaml
   apiVersion: cert-manager.io/v1
   kind: Certificate
   metadata:
     name: domain-wildcard
     namespace: app-namespace
   spec:
     secretName: tls-wildcard
     issuerRef:
       name: route53
       kind: ClusterIssuer
     commonName: "*.domain.com"
     dnsNames:
     - "*.domain.com"
   ```

#### Requesting a wildcard certificate using DNS in Google

1. Create a ServiceAccount with the required role:

   - Open the [policy management page](https://console.cloud.google.com/iam-admin/serviceaccounts).
   - Choose the target project and create a ServiceAccount under any name (for example, `dns01-solver`).
   - In the created account, create a key by clicking **Add key**.
     A JSON file with the key data will be downloaded.
   - Encode the JSON file as a Base64 string:

     ```shell
     base64 project-209317-556c656b81c4.json
     ```

1. Save the Base64 string in the [`cloudDNSServiceAccount`](/modules/cert-manager/configuration.html#parameters-clouddnsserviceaccount) parameter.

   DKP will automatically create a ClusterIssuer and Secret for CloudDNS in the `d8-cert-manager` namespace.

1. Create a Certificate resource using CloudDNS for validation:

   ```yaml
   apiVersion: cert-manager.io/v1
   kind: Certificate
   metadata:
     name: domain-wildcard
     namespace: app-namespace
   spec:
     secretName: tls-wildcard
     issuerRef:
       name: clouddns
       kind: ClusterIssuer
     dnsNames:
     - "*.domain.com"
   ```

#### Requesting a self-signed certificate

To issue a self-signed certificate, specify `selfsigned` as the value for `issuerRef.name`:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: example-com            # Certificate name.
  namespace: default
spec:
  secretName: example-com-tls  # Name of the Secret that will store the private key and the certificate.
  issuerRef:
    kind: ClusterIssuer        # Certificate issuer details.
    name: selfsigned
  commonName: example.com      # Certificate main domain.
  dnsNames:                    # Optional additional certificate domains (at least one DNS name or IP address).
  - www.example.com
  - admin.example.com
```

### Generating a self-signed certificate

When generating certificates manually, it is important to fill out all fields of the certificate request correctly
to ensure that the final certificate is issued properly and can be validated across various services.

It is important to follow these guidelines:

1. Specify domain names in the `SAN` (Subject Alternative Name) field.

   The `SAN` field is a more modern and commonly used method for specifying the domain names covered by the certificate.
   Some services no longer consider the `CN` (Common Name) field as the source for domain names.

2. Correctly fill out the `keyUsage`, `basicConstraints`, `extendedKeyUsage` fields, specifically:
   - `basicConstraints = CA:FALSE`

     This field determines whether the certificate is an end-entity certificate or a certification authority (CA) certificate.
     CA certificates cannot be used as service certificates.

   - `keyUsage = digitalSignature, keyEncipherment`

     The `keyUsage` field limits the permissible usage scenarios of this key:

     - `digitalSignature`: Allows the key to be used for signing digital messages and ensuring data integrity.
     - `keyEncipherment`: Allows the key to be used for encrypting other keys, which is necessary for secure data exchange using TLS (Transport Layer Security).

   - `extendedKeyUsage = serverAuth`

     The `extendedKeyUsage` field specifies additional key usage scenarios required by specific protocols or applications:

     - `serverAuth`: Indicates that the certificate is intended for server use, authenticating the server to the client during the establishment of a secure connection.

It is also recommended to:

1. Issue the certificate for no more than 1 year (365 days).

   The validity period of the certificate affects its security. A one-year validity ensures the cryptographic methods remain current and allows for timely certificate updates in case of threats. Furthermore, some modern browsers now reject certificates with a validity period longer than 1 year.

2. Use robust cryptographic algorithms, such as elliptic curve algorithms (including `prime256v1`).

   Elliptic curve algorithms (ECC) provide a high level of security with a smaller key size compared to traditional methods like RSA. This makes the certificates more efficient in terms of performance and secure in the long term.

3. Do not specify domains in the `CN` (Common Name) field.

   Historically, the `CN` field was used to specify the primary domain name for which the certificate was issued. However, modern standards, such as [RFC 2818](https://datatracker.ietf.org/doc/html/rfc2818), recommend using the `SAN` (Subject Alternative Name) field for this purpose.
   If the certificate is intended for multiple domain names listed in the `SAN` field, specifying one of the domains additionally in `CN` can cause a validation error in some services when accessing domains not listed in `CN`.
   If non-domain-related information is specified in `CN` (for example, an identifier or service name), the certificate will also extend to these names, which could be exploited for malicious purposes.

#### Certificate generation example

To generate a certificate, we'll use the `openssl` utility.

1. Fill in the `cert.cnf` configuration file:

   ```ini
   [ req ]
   default_bits       = 2048
   default_md         = sha256
   prompt             = no
   distinguished_name = dn
   req_extensions     = req_ext
   [ dn ]
   C = GB
   ST = London
   L = London
   O = Example Company
   OU = IT Department
   # CN = Do not specify the CN field.
   [ req_ext ]
   subjectAltName = @alt_names
   [ alt_names ]
   # Specify all domain names.
   DNS.1 = example.co.uk
   DNS.2 = www.example.co.uk
   DNS.3 = api.example.co.uk
   # Specify IP addresses (if required).
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

2. Generate an elliptic curve key:

   ```shell
   openssl ecparam -genkey -name prime256v1 -noout -out ec_private_key.pem
   ```

3. Create a certificate signing request:

   ```shell
   openssl req -new -key ec_private_key.pem -out example.csr -config cert.cnf
   ```

4. Generate a self-signed certificate:

   ```shell
   openssl x509 -req -in example.csr -signkey ec_private_key.pem -out example.crt -days 365 -extensions v3_req -extfile cert.cnf
   ```

## Protecting credentials

If you prefer not to store credentials in the DKP configuration, you can create a separate Secret and reference it in the ClusterIssuer resource.

To do this, follow these steps:

1. Create a Secret with the access key:

   ```yaml
   d8 k apply -f - <<EOF
   apiVersion: v1
   kind: Secret
   type: Opaque
   metadata:
     name: route53
     namespace: default
   data:
     secret-access-key: MY-AWS-ACCESS-KEY-TOKEN
   EOF
   ```

1. Create a ClusterIssuer that references the Secret:

   ```yaml
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
             accessKeyID: MY-AWS-ACCESS-KEY-ID
             secretAccessKeySecretRef:
               name: route53
               key: secret-access-key
   EOF
   ```

1. Request certificates as usual, using the created ClusterIssuer:

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
       name: route53
     commonName: www.example.com 
     dnsNames:
     - www.example.com
   EOF
   ```

## Support for tls-acme annotation

DKP supports the annotation `kubernetes.io/tls-acme: "true"` in Ingress resources.
The `cert-manager-ingress-shim` component watches for this annotation
and automatically creates Certificate resources in the same namespace as the Ingress.

{% alert level="warning" %}
When using the annotation, the Certificate resource is linked to the existing Ingress.
No separate Ingress is created for domain validation; instead, additional records are added to the existing Ingress.
If the Ingress has authentication or a whitelist configured, the validation will fail.
Therefore, it’s recommended that you use a Certificate resource directly instead.

If switching from annotation to a manual Certificate, make sure to delete the automatically generated Certificate first.
Otherwise, both will try to update the same Secret, potentially exceeding Let's Encrypt rate limits.
{% endalert %}

Example Ingress configuration with the annotation:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    kubernetes.io/tls-acme: "true"           # Annotation.
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
  - host: www.example.com                    # Additional domain.
    http:
      paths:
      - backend:
          service:
            name: site
            port:
              number: 80
        path: /
        pathType: ImplementationSpecific
  - host: admin.example.com                  # Another additional domain.
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
    - www.example.com                        # Additional domain.
    - admin.example.com                      # Another additional domain.
    secretName: example-com-tls              # Name of the Certificate and Secret.
```
