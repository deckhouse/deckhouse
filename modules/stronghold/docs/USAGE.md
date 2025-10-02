---
title: "The Stronghold module module: usage"
description: Usage of the Stronghold Deckhouse module.
---

## How to enable the module

The module can be enabled by applying ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: stronghold
spec:
  enabled: true
```

or by executing the command:

```shell
kubectl -n d8-system exec deploy/deckhouse -c deckhouse -it -- deckhouse-controller module enable stronghold
```

By default, the module will run in the `Automatic` mode with the `Ingress` inlet.
In the current version, there are no other inlets and modes.

## How to disable the module

You can disable a module by setting the `enabled` value in moduleconfig `stronghold` to `false`.
Or by running the command:

```bash
kubectl -n d8-system exec deploy/deckhouse -c deckhouse -it -- deckhouse-controller module disable stronghold
```

### ATTENTION!

When the module is disabled, all Stronghold containers will be deleted, as well as the `stronghold-keys` secret
with root and unseal keys from the namespace `d8-stronghold`. In this case, the sevice data will not be deleted from the nodes.
You can enable the module again and recreate a copy of the `stronghold-keys` secret in the `d8-stronghold` namespace,
then access to the data will be restored.

If the old data is no longer needed, you must manually delete the `/var/lib/deckhouse/stronghold` directory
on all master nodes of the cluster.

## How to access the service

Access to the service is provided through inlets. Currently, only one inlet, Ingress, is available.
The web interface address for Stronghold is formed as follows: in the template [publicDomainTemplate](../../../documentation/v1/deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) of the Deckhouse global configuration parameter, the `%s` placeholder is replaced with `stronghold` keyword.

For example, if `publicDomainTemplate` is set to `%s-kube.mycompany.tld`, than the Stronghold web interface will be accessible at the address `stronghold-kube.cmycompany.tld`.

## how to use Data Storage. Operating Modes

The data stored in Stronghold is encrypted. To decrypt the storage data, an encryption key is required. The encryption key is also stored with the data (as part of key bundles), but it is encrypted with another encryption key known as the root key.

To decrypt Stronghold data, it is necessary to decrypt the encryption key, which requires the root key. Unsealing the storage is the process of gaining access to this root key. The root key is stored along with all other storage data but is encrypted with another mechanism: the unseal key.

In the current version of the module, there is only the `Automatic` mode, in which the storage is automatically initialized during the first module launch. During the initialization process, the unlocking key and root token are both placed into the `stronghold-keys` secret in the `d8-stronghold` namespace in the Kubernetes cluster. After the initialization, the module automatically unseals the nodes of the Stronghold cluster.
In the automatic mode, in the event of a restart of Stronghold nodes, the storage will also be automatically unsealed without manual intervention.

## Access Management

The role named `deckhouse_administrators` is created after storage initialization using the `Automatic` mode of the Stronghold module. This role is granted access to the web interface through OIDC authentication via [Dex](https://deckhouse.ru/documentation/v1/modules/150-user-authn/).
Additionally, the automatic connection of the current Deckhouse cluster to Stronghold is configured. This is necessary for the operation of the [secrets-store-integration](../../secrets-store-integration/) module.

To provide access to the users with the `admins` group membership (group membership is conveyed from the used IdP or LDAP via Dex), you need to specify this group in the `administrators` array in the ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: stronghold
spec:
  enabled: true
  version: 1
  settings:
    management:
      mode: Automatic
      administrators:
      - type: Group
        name: admins
```

To grant administrator rights to users with roles `manager` and `securityoperator`, you can use the following parameters in the ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: stronghold
spec:
  enabled: true
  version: 1
  settings:
    management:
      mode: Automatic
      administrators:
      - type: User
        name: manager@mycompany.tld
      - type: User
        name: securityoperator@mycompany.tld
```

If needed, you can create users in Stronghold with different access rights to secrets using the built-in storage mechanisms.

## Running with a self-signed certificate

You need to create a CA, a certificate, and sign it with the created CA. If there is already a CA, you can sign the certificate using it.
Note that you have to create a fullchain certificat.

The `createCertificate.sh` script below creates the proper certificate + key pair
for the `mycompany.tld` domain (`*.mycompany.tld`) using openssl.

```shell
#!/bin/bash

set -e
caName="MyOrg-RootCA"            # CA name (CN)
publicDomain="mycompany.tld"     # Cluster domain name (see publicDomainTemplate)
certName="kubernetes"            # Cluster certificate name (CN)

mkdir -p "${caName}"
cd "${caName}"

[ ! -f "${caName}.key" ] && openssl genrsa -out "${caName}.key" 4096

[ ! -f "${caName}.crt" ] &&  openssl req -x509 -new -nodes -key "${caName}.key" -sha256 -days 1826 -out "${caName}.crt" \
   -subj "/CN=${caName}/O=MyOrganisation"

openssl req -new -nodes -out ${certName}.csr -newkey rsa:4096 -keyout "${certName}.key" \
  -subj "/CN=${certName}/O=MyOrganisation"

# v3 ext file
cat > "${certName}.v3.ext" << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${publicDomain}
DNS.2 = *.${publicDomain}
EOF

openssl x509 -req -in "${certName}.csr" -CA "${caName}.crt" -CAkey "${caName}.key" -CAcreateserial -out "${certName}.crt" -days 730 -sha256 -extfile "${certName}.v3.ext"

cat "${certName}.crt" "${caName}.crt" > "${certName}_fullchain.crt"
```

You have to create a secret in the d8-system namespace using the generated `kubernetes.key` and `kubernetes_fullchain.crt` files:

```shell
kubectl -n d8-system create secret tls mycompany-wildcard-tls --cert=kubernetes_fullchain.crt --key=kubernetes.key
```

To use the created certificate in the cluster, you need to configure the `global` module as follows
(use the `kubectl edit mc global` command):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  settings:
    modules:
      https:
        customCertificate:
          secretName: mycompany-wildcard-tls
        mode: CustomCertificate
      publicDomainTemplate: '%s.mycompany.tld'
  version: 2
```

If your domain fails to be resolved by DNS and you plan to use the hosts file, then for dex to work,
you have to add the balancer address or the IP of the front node to the cluster DNS so that pods can access the `dex.mycompany.tld` domain by name.

Here is how you can retrieve the IP for the `nginx-load-balancer` ingress of the `LoadBlancer` type.

```shell
kubectl -n d8-ingress-nginx get svc nginx-load-balancer -o jsonpath='{ .spec.clusterIP }'
```
Suppose your address is `10.202.166.188`, then kube-dns module-config will look as follows:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: kube-dns
spec:
  version: 1
  enabled: true
  settings:
    hosts:
    - domain: dex.mycompany.tld
      ip: 10.202.166.188
```

Note that you have to configure the `user-authn` module by setting `controlPlaneConfigurator.dexCAMode` to `FromIngressSecret`.
In this case, the CA will be retrieved from the chain we placed in the `kubernetes_fullchain.crt` file.

An example:
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  enabled: true
  settings:
    controlPlaneConfigurator:
      dexCAMode: FromIngressSecret
    ...
```

Now you can enable the `stronghold` module. It will automatically initialize and set up integration with `dex`.

```shell
kubectl -n d8-system exec deploy/deckhouse -c deckhouse -it -- deckhouse-controller module enable stronghold
```
