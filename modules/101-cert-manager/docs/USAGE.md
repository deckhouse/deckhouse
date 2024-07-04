---
title: "The cert-manager module: usage"
---


## An example of provisioning a certificate

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: example-com                          # the name of the certificate; you can use it to view the cert's status
  namespace: default
spec:
  secretName: example-com-tls                # the name of the secret to store a private key and a certificate
  issuerRef:
    kind: ClusterIssuer                      # the link to the certificate "issuer", see more below
    name: letsencrypt
  commonName: example.com                    # the main certificate domain
  dnsNames:                                  # additional domains (At least one DNS Name or IP address is required)
  - www.example.com
  - admin.example.com
```

Here:
* a separate Ingress resource is created for the duration of the challenge (thus, authentication and whitelist of the primary Ingress will not interfere with the process),
* you can issue a single certificate for several Ingress resources (the deletion of the resource based on the `tls-acme` annotation won't affect it in any way),
* you can issue a certificate with multiple DNS names (as in the example above),
* you can validate different domains that are part of the same certificate using different Ingress controllers.

Read more in the [cert-manager documentation](https://cert-manager.io/docs/tutorials/acme/http-validation/).

## Issuing a DNS wildcard certificate using Cloudflare

1. Get the `Global API Key` and `Email Address`:
   * Go to <https://dash.cloudflare.com/profile>.
   * You can find an active `Email Address` at the very top of the page.
   * Click the `View` button at the bottom of the page next to the `Global API Key`.

   You will see the key for interacting with the Cloudflare API (as well as the account email).

2. Edit the [cert-manager module configuration](configuration.html) and add the following parameters:

   ```yaml
   settings:
     cloudflareGlobalAPIKey: APIkey
     cloudflareEmail: some@mail.somedomain
   ```

   or

   ```yaml
   settings:
     cloudflareAPIToken: some-token
     cloudflareEmail: some@mail.somedomain
   ```

   After that, Deckhouse will automatically create ClusterIssuer and Secret for Cloudflare in the `d8-cert-manager` namespace.

   * Configuration with [APIToken](https://cert-manager.io/docs/configuration/acme/dns01/cloudflare/#api-tokens) is more secure and recommended for use.

3. Create a Certificate with validation via Cloudflare. Note that you must specify `cloudflareGlobalAPIKey` and `cloudflareEmail` in Deckhouse beforehand:

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

4. Create an Ingress:

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

## Issuing a DNS wildcard certificate using Route53

1. Create a user with the appropriate permissions.

   * For this, go to the policy [management page](https://console.aws.amazon.com/iam/home?region=us-east-2#/policies) and create a policy as follows:

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

   * Go to the [user management page](https://console.aws.amazon.com/iam/home?region=us-east-2#/users) and create a user with the above policy.

2. Edit the [cert-manager module configuration](configuration.html) and add the following parameters:

   ```yaml
   settings:
     route53AccessKeyID: AKIABROTAITAJMPASA4A
     route53SecretAccessKey: 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
   ```

   After that, Deckhouse will automatically create ClusterIssuer and Secret for route53 in the `d8-cert-manager` namespace.

3. Create a Certificate with validation via route53. Note that you must specify `route53AccessKeyID` and `route53SecretAccessKey` in Deckhouse beforehand:

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

## Issuing a DNS wildcard certificate using Google

1. Create a service account with the appropriate role:

   * Go to the [policy management page](https://console.cloud.google.com/iam-admin/serviceaccounts).
   * Select your project.
   * Create a service account with the desired name (e.g., `dns01-solver`).
   * Switch to the service account created.
   * Add a key by clicking the "Add key" button.
   * The `.json` file with the key data will be saved to your computer.
   * Encode the resulting file using the **base64** algorithm:

     ```shell
     base64 project-209317-556c656b81c4.json
     ```

2. Use the resulting **base-64** string for setting the  `cloudDNSServiceAccount` module parameter.

   After that, Deckhouse will automatically create ClusterIssuer and Secret for cloudDNS in the `d8-cert-manager` namespace.

3. Create a Certificate with validation via cloudDNS:

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

## Issuing a self-signed certificate

In this case, the entire process is even more straightforward than that of LetsEncrypt. Simply replace the issuer name (`letsencrypt`) with `selfsigned`:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: example-com                          # the name of the certificate; you can use it to view the cert's status
  namespace: default
spec:
  secretName: example-com-tls                # the name of the secret to store a private key and a certificate
  issuerRef:
    kind: ClusterIssuer                      # the link to the certificate "issuer", see more below
    name: selfsigned
  commonName: example.com                    # the main certificate domain
  dnsNames:                                  # additional certificate domains (optional)
  - www.example.com
  - admin.example.com
```
