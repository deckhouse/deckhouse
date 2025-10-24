---
title: "MULTIFACTOR Ldap Adapter"
permalink: en/stronghold/documentation/user/auth/mfa/multifactor.html
---

MULTIFACTOR LDAP Adapter is an LDAP proxy server developed and maintained by MULTIFACTOR.
It is used to provide two-factor authentication for users of applications that use LDAP authentication.
The system ensures multi-factor authentication and access control for any remote connections such as RDP, VPN, VDI, SSH, and others.

## Configuring LDAP Adapter

### How it works

Stronghold can perform two-factor authentication for users from an LDAP or Active Directory catalog:

1. The user connects to Stronghold and enters their username and password.
1. Stronghold connects via LDAP protocol to [MULTIFACTOR LDAP Adapter](https://multifactor.ru/docs/ldap-adapter/ldap-adapter/).
1. The component verifies the user's login and password in Active Directory or another LDAP catalog and requests a second authentication factor.
1. The user confirms the access request using the selected authentication method.

### Configuring MULTIFACTOR

1. Log in to the [MULTIFACTOR management system](https://admin.multifactor.ru/account/login).
   In the "Resources" section, create a new LDAP application.
   After creation, two parameters will become available (`NAS Identifier` and `Shared Secret`), which you will need in the following steps.
1. Download and install [MULTIFACTOR LDAP Adapter](https://multifactor.ru/docs/ldap-adapter/ldap-adapter/).

### Running LDAP Adapter in Kubernetes

To run the adapter, use the image `multifactor-ldap-adapter:3.0.7` and the following manifest:

```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ldap-adapter
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ldap-adapter
  template:
    metadata:
      labels:
        app: ldap-adapter
    spec:
      containers:
      - image: registry.deckhouse.ru/stronghold/multifactor/multifactor-ldap-adapter:3.0.7
        name: ldap-adapter
        volumeMounts:
        - mountPath: /opt/multifactor/ldap/multifactor-ldap-adapter.dll.config
          name: config
          subPath: multifactor-ldap-adapter.dll.config
      volumes:
      - configMap:
          defaultMode: 420
          name: ldap-adapter
        name: config
---
apiVersion: v1
kind: Service
metadata:
  name: ldap-adapter
spec:
  ports:
  - port: 389
    protocol: TCP
    targetPort: 389
  selector:
    app: ldap-adapter
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ldap-adapter
data:
  multifactor-ldap-adapter.dll.config: |
    <?xml version="1.0" encoding="utf-8"?>
    <configuration>
      <configSections>
        <section name="UserNameTransformRules" type="MultiFactor.Ldap.Adapter.Configuration.UserNameTransformRulesSection, multifactor-ldap-adapter" />
      </configSections>
      <appSettings>
        <add key="adapter-ldap-endpoint" value="0.0.0.0:389"/>
        <add key="ldap-server" value="ldap://ldap.example.com"/>
        <add key="ldap-service-accounts" value="CN=admin,DC=example,DC=com"/>
        <add key="ldap-base-dn" value="ou=Users,dc=example,dc=com"/>
        <add key="multifactor-api-url" value="https://api.multifactor.ru" />
        <add key="multifactor-nas-identifier" value="YOUR-NAS-IDENTIFIER" />
        <add key="multifactor-shared-secret" value="YOUR-NAS-SECRET" />
        <add key="logging-level" value="Debug"/>
      </appSettings>
    </configuration>
```

In the configuration, specify the address of your LDAP server
and the values of `multifactor-nas-identifier` and `multifactor-shared-secret` obtained from the MULTIFACTOR management panel.

Available images:

- Ubuntu 24.04–based: `registry.deckhouse.ru/stronghold/multifactor/multifactor-ldap-adapter:3.0.7`
- Alpine 3.22–based: `registry.deckhouse.ru/stronghold/multifactor/multifactor-ldap-adapter:3.0.7-alpine`

## Configuring Stronghold

To configure Stronghold, create and set up an `ldap` authentication method and specify the `ldap-adapter` address as the server.
If you used the example manifest above to deploy the adapter, the address should be `ldap://ldap-adapter.default.svc`:

```shell
d8 stronghold auth enable ldap
d8 stronghold write auth/ldap/config url="ldap://ldap-adapter.default.svc" \
   binddn="cn=admin,dc=example,dc=com" bindpass="Password-1" \
   userdn="ou=Users,dc=example,dc=com" groupdn="ou=Groups,dc=example,dc=com" \
   username_as_alias=true
```

## Testing with local OpenLDAP server

Below is an example manifest that you can use to deploy an OpenLDAP service in Kubernetes for testing purposes:

```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: openldap
spec:
  replicas: 1
  selector:
    matchLabels:
      app: openldap
  template:
    metadata:
      labels:
        app: openldap
    spec:
      containers:
      - env:
        - name: LDAP_ADMIN_DN
          value: cn=admin,dc=example,dc=com
        - name: LDAP_ROOT
          value: dc=example,dc=com
        - name: LDAP_ADMIN_USERNAME
          value: admin
        - name: LDAP_ADMIN_PASSWORD
          value: Password-1
        image: bitnami/openldap:2.6.10
        name: openldap
---
apiVersion: v1
kind: Service
metadata:
  name: openldap
spec:
  ports:
  - name: p389
    port: 389
    protocol: TCP
    targetPort: 1389
  selector:
    app: openldap
```

After starting the container, create a user (for example, `alice` with the password `D3mo-Passw0rd`):

1. Access the OpenLDAP container:

   ```shell
   d8 k exec svc/openldap -it -- bash
   ```

1. Create the user with the following commands:

   ```shell
   cd /tmp
   cat << EOF > create_entries.ldif
   dn: uid=alice,ou=users,dc=example,dc=com
   objectClass: inetOrgPerson
   objectClass: person
   objectClass: top
   cn: Alice
   sn: User
   userPassword: D3mo-Passw0rd
   EOF

   ldapadd -H ldap://openldap -cxD "cn=admin,dc=example,dc=com" \
           -w "Password-1" -f "create_entries.ldif"
   ```

You can now log in as the `alice` user using the password `D3mo-Passw0rd`.
In the [MULTIFACTOR management panel](https://admin.multifactor.ru/account/login),
a user named `alice` will appear under the "Users" section, where you can assign a second factor to them.
In the future, confirmation will be required each time the user logs in to Stronghold.
In addition to audit logs on the Stronghold side, the second-factor confirmation will also be recorded on the MULTIFACTOR side.
