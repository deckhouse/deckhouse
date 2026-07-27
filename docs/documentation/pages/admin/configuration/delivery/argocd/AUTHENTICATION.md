---
title: "Configuring authentication and authorization"
permalink: en/admin/configuration/delivery/argocd/authentication/
description: "Configuring Argo CD authentication and authorization in Deckhouse Kubernetes Platform."
lang: en
relatedLinks:
  - title: "Official Argo CD website"
    url: "https://argo-cd.readthedocs.io"
  - title: "Official Argo CD Operator website"
    url: "https://argocd-operator.readthedocs.io"
---

Argo CD supports local authentication and is also integrated with the Deckhouse Kubernetes Platform identity and access subsystem.
You can learn more about configuring [authentication](../../../access/authentication/) and [authorization](../../../access/authorization/) in the DKP documentation.

{% alert level="warning" %}
If no additional settings are defined in the ArgoCD object, the local `admin` account with the `admin` role is active by default.
{% endalert %}

You can learn about managing user and group permissions in the [Configuring role-based access control](../rbac/) section.

## Local authentication

When an ArgoCD object is created, the `admin` user with the `admin` role is created automatically. The `admin` user password is generated automatically. To retrieve it, run:

```bash
d8 k -n argocd get secret argocd-cluster -o jsonpath='{.data.admin\.password}' | base64 -d
```

### Creating additional local users

When creating a local user, you can define whether the user has access to the Argo CD web interface (the `login` attribute) and/or to the Argo CD API (the `apiKey` attribute).

To create local users, set the [`spec.localUsers`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-localusers) parameter of ArgoCD, for example:

```bash
d8 k create -f - <<EOF
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
  namespace: argocd
spec:
  localUsers:
    # The "deploy" user with rights to sign in to the web interface and generate tokens.
    - name: deploy
      apiKey: true
      login: true
      enabled: true
    # The "ci-bot" user for automation only, without web interface access.
    - name: ci-bot
      apiKey: true
      login: false
      enabled: true
EOF
```

After creating the user and applying the ArgoCD object manifest, additionally generate a password for web interface access if the user has been granted the corresponding rights.

To set a password through a Secret, first calculate its bcrypt hash, then encode the result in Base64:

```bash
ARGOCD_USER=<USER_NAME>
ARGOCD_PASS=$(echo "<USER_PASSWORD>" | htpasswd -BinC 10 "" | cut -d: -f2 | base64 -w0)
d8 k -n argocd patch secret argocd-secret -p "{\"data\":{\"accounts.$ARGOCD_USER.password\":\"$ARGOCD_PASS\"}}"
```

{% alert level="info" %}
You can set or change a local user password with the `argocd` CLI utility:

```bash
argocd login <ARGOCD_DOMAIN>:443 --username admin --password <ADMIN_PASSWORD>
argocd account update-password \
  --account <ACCOUNT> \
  --current-password <ADMIN_PASSWORD> \
  --new-password <NEW_PASSWORD>
```

{% endalert %}

### Creating a user token

To work with the API, the user must have the corresponding rights (the [`apiKey`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-localusers-apikey) parameter; see the example in the section above). To issue a token, use the Argo CD web interface or the `argocd` CLI utility.

To issue a token through the web interface, go to "Settings" → "Accounts", select the required user, and click "Generate New" in the "Tokens" section.

To issue a token with the `argocd` CLI utility, run the following commands (provide the required values):

```bash
argocd login <ARGOCD_DOMAIN>:443 --username admin
argocd account generate-token --account <ACCOUNT>
```

{% alert level="info" %}
If the user does not have web interface access rights (`login`), another user with administrator rights can issue a token for them.
{% endalert %}

### Disabling local authentication

To disable local authentication, disable the `admin` user by setting the [`spec.disableAdmin`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-disableadmin) parameter of ArgoCD to `true`. Also remove all additional local users if they were created earlier (for details, see [Creating additional local users](#creating-additional-local-users)).

After disabling local authentication, review the access rules in the [Configuring role-based access control](../rbac/) section.

## SSO authentication

Before configuring the ArgoCD object, create an OAuth2 client — a [DexClient](/modules/user-authn/cr.html#dexclient) object required for integration with Deckhouse Kubernetes Platform:

```bash
d8 k create -f -<<EOF
apiVersion: deckhouse.io/v1
kind: DexClient
metadata:
  name: argocd
  namespace: argocd
spec:
  redirectURIs:
    - https://<ARGOCD_DOMAIN>/api/dex/callback
    - https://<ARGOCD_DOMAIN>/api/dex/callback-reserve
EOF
```

`<ARGOCD_DOMAIN>` is the fully qualified domain name (FQDN) set in the [`.spec.server.host`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-server-host) section of ArgoCD.

Wait until Deckhouse Kubernetes Platform creates a Secret with the client secret key:

```shell
d8 k -n argocd get secret/dex-client-argocd
```

Configure the ArgoCD object to use SSO in Deckhouse Kubernetes Platform:

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
  namespace: argocd
spec:
  sso:
    dex:
      config: |
        connectors:
          - type: oidc
            id: deckhouse
            name: deckhouse
            config:
              issuer: "https://dex.<CLUSTER_DOMAIN>/"
              clientID: "dex-client-argocd@argocd"
              clientSecret: "$dex-client-argocd:clientSecret"
              insecureEnableGroups: true
              scopes:
                - profile
                - email
                - openid
                - groups
    provider: dex
  server:
    host: <ARGOCD_DOMAIN>
    ingress:
      enabled: true
      ingressClassName: <INGRESS_CLASS_NAME>
      tls:
        - hosts:
            - <ARGOCD_DOMAIN>
          secretName: argocd-ingress-tls
    insecure: true
```

Restart the Argo CD server:

```shell
d8 k -n argocd rollout restart deploy/argocd-server
```

{% alert level="warning" %}
If you do not restart the Argo CD server, the sign-in attempt fails, and an error message appears in the Argo CD server log ([issue argoproj/argo-cd#13526](https://github.com/argoproj/argo-cd/issues/13526)).

{% offtopic title="Example error message..." %}

<!-- markdownlint-disable MD013 -->

```text
time="2024-10-16T14:12:59Z" level=warning msg="Failed to verify token: failed to verify token: token verification failed for all audiences: error for aud \"argo-cd\": Failed to query provider \"https://argocd.<ARGOCD_DOMAIN>/api/dex\": Get \"https://argocd.<ARGOCD_DOMAIN>/api/dex/.well-known/openid-configuration\": tls: failed to verify certificate: x509: certificate is valid for ingress.local, not argocd.<ARGOCD_DOMAIN>, error for aud \"argo-cd-cli\": Failed to query provider \"https://argocd.<ARGOCD_DOMAIN>/api/dex\": Get \"https://argocd.<ARGOCD_DOMAIN>/api/dex/.well-known/openid-configuration\": tls: failed to verify certificate: x509: certificate is valid for ingress.local, not argocd.<ARGOCD_DOMAIN>"
```

<!-- markdownlint-enable MD013 -->

{% endofftopic %}
{% endalert %}

### Using a self-signed certificate

First obtain the self-signed certificate used by the Deckhouse Kubernetes Platform identity and access subsystem:

```bash
d8 k -n d8-user-authn get secret ingress-tls -o jsonpath='{.data.tls\.crt}' | base64 -d
```

Then add the obtained certificate to the OIDC connector configuration in the `rootCAs` section of ArgoCD:

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
  namespace: argocd
spec:
  sso:
    dex:
      config: |
        connectors:
          - type: oidc
            id: deckhouse
            name: deckhouse
            config:
              issuer: "https://dex.<CLUSTER_DOMAIN>/"
              rootCAs:
                - |
                  -----BEGIN CERTIFICATE-----
                  <Self-signed certificate obtained in the previous step>
                  -----END CERTIFICATE-----
              clientID: "dex-client-argocd@argocd"
              clientSecret: "$dex-client-argocd:clientSecret"
              insecureEnableGroups: true
              scopes:
                - profile
                - email
                - openid
                - groups
    provider: dex
  server:
    host: <ARGOCD_DOMAIN>
    ingress:
      enabled: true
      tls:
        - hosts:
            - <ARGOCD_DOMAIN>
          secretName: argocd-ingress-tls
    insecure: true
```

### Creating a user token

Argo CD does not allow issuing permanent tokens for users authenticated with SSO. At the same time, such users can use the `argocd` CLI utility by specifying the `--sso` flag during authentication:

```bash
argocd login <ARGOCD_DOMAIN>:443 --sso
```

When this command runs on the administrator workstation, a web browser opens with the Deckhouse Kubernetes Platform authentication form.

{% alert level="info" %}
To configure a permanent token, create a local Argo CD user and set the `apiKey` attribute for them. In this case, the user has access rights only to the Argo CD API.
For details, see [Creating additional local users](#creating-additional-local-users).
{% endalert %}
