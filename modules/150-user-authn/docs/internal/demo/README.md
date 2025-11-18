# Steps

## Install OpenLDAP

* Apply [**ldap**](./ldap.yaml) manifest to the cluster.

## Add DexProvider

* Apply [**dex-provider**](./dex-provider.yaml) manifest to the cluster.
* Login to Kubeconfig (username: `janedoe@example.com`, password: `foo` or username: `johndoe@example.com`, password: `bar`).

## Generate kubeconfig

* Copy kubeconfig to your PC.
* Show how kubeconfig works.

## Deploy simple echo server

* Apply [**echo-service**](./echo-service.yaml) manifest to the cluster.
  > **Note!** Do not forget to use your cluster public domain instead of `{{ __cluster__domain__ }}` in the manifest.
* Show how you can access this service without authorization.

## Deploy DexAuthenticator

* Apply [**dex-authenticator**](./dex-authenticator.yaml) manifest to the cluster.
  > **NOTE**: Do not forget to use your cluster public domain instead of `{{ __cluster__domain__ }}` in the manifest.
* Add annotations to the ingress resource.
  ```shell
  d8 k -n openldap-demo annotate ingress echoserver 'nginx.ingress.kubernetes.io/auth-signin=https://$host/dex-authenticator/sign_in'
  d8 k -n openldap-demo annotate ingress echoserver 'nginx.ingress.kubernetes.io/auth-url=https://echoserver-dex-authenticator.openldap-demo.svc.cluster.local/dex-authenticator/auth'
  ```
* Show that access is protected (`janedoe@example.com` can access echo server, `johndoe@example.com` cannot).

## Create a user

* Apply [**dex-user**](./dex-user.yaml) manifest to the cluster.
* Show that you can log in with credentials from the custom resource.
* Add `groups: ["developers"]` to the User spec to show that this user now has access to the echo server. 
  ```shell
  d8 k patch user openldap-demo --type='merge' -- patch '{"spec": {"groups": ["developers"]}}'
  ```
# Cleaning

Execute [**clean_up.sh**](./clean_up.sh)

## Kerberos (SPNEGO) LDAP SSO — quick demo

This adds passwordless Kerberos (SPNEGO) on top of the LDAP demo using a test KDC inside the cluster. You will:
- run a KDC in-cluster and expose it to your macOS via SSH port-forward;
- create the HTTP SPN/keytab for Dex and a user principal mapped to LDAP;
- enable Kerberos for the existing `DexProvider`.

Prereqs:
- Dex is reachable at `https://dex.<your-domain>/`.
- You have SSH access to a node or bastion that can reach the cluster API and nodes (for the `-J` jump).

1) Deploy KDC in the cluster (namespace d8-user-authn)

```bash
d8 k -n d8-user-authn apply -f ./kerberos-kdc.yaml
d8 k -n d8-user-authn rollout status deploy/kdc
```

2) Create SPN and keytab for Dex, and a test user principal

Assume Dex FQDN is `dex.<your-domain>` (replace below).

```bash
DEX_FQDN="dex.<your-domain>"

d8 k -n d8-user-authn exec deploy/kdc -- \
  kadmin.local -q "addprinc -randkey HTTP/${DEX_FQDN}@EXAMPLE.COM"

d8 k -n d8-user-authn exec deploy/kdc -- \
  kadmin.local -q "ktadd -k /tmp/krb5.keytab HTTP/${DEX_FQDN}@EXAMPLE.COM"

# principal mapped to LDAP 'john' (existing in this demo LDAP)
d8 k -n d8-user-authn exec deploy/kdc -- \
  kadmin.local -q 'addprinc -pw bar john@EXAMPLE.COM'

# export keytab into a Secret for Dex
POD=$(d8 k -n d8-user-authn get pods -l app=kdc -o jsonpath='{.items[0].metadata.name}')
d8 k -n d8-user-authn cp "$POD":/tmp/krb5.keytab ./krb5.keytab
d8 k -n d8-user-authn create secret generic dex-kerberos-test \
  --from-file=krb5.keytab=./krb5.keytab
```

Note about the keytab contents and rotation:
- The keytab mounted to Dex should contain only the HTTP service principal for Dex, e.g. `HTTP/dex.<your-domain>@EXAMPLE.COM`. Do not include user principals (like `john@EXAMPLE.COM`) in this keytab — users authenticate on their clients (kinit) with a password.
- To rotate the Dex keytab, re-run `ktadd` to produce a new `/tmp/krb5.keytab`, then update the Secret and restart Dex:
```bash
d8 k -n d8-user-authn create secret generic dex-kerberos-test \
  --from-file=krb5.keytab=./krb5.keytab --dry-run=client -o yaml | \
  d8 k -n d8-user-authn apply -f -
d8 k -n d8-user-authn rollout restart deploy/dex
```

3) Seed LDAP test data (required for the demo)

```bash
# Launch a temporary tools pod
kubectl -n d8-user-authn run ldap-tools --restart=Never --image=osixia/openldap:1.5.0 -- sleep 3600
kubectl -n d8-user-authn wait --for=condition=Ready pod/ldap-tools --timeout=60s

# Prepare LDIF with john/bar and a devs group (dc=example,dc=com)
kubectl -n d8-user-authn exec -i ldap-tools -- bash -lc 'cat >/tmp/seed.ldif' <<'LDIF'
dn: ou=users,dc=example,dc=com
objectClass: top
objectClass: organizationalUnit
ou: users

dn: ou=groups,dc=example,dc=com
objectClass: top
objectClass: organizationalUnit
ou: groups

dn: cn=john,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
cn: john
sn: User
mail: john@example.com
userPassword: bar

dn: cn=devs,ou=groups,dc=example,dc=com
objectClass: groupOfNames
cn: devs
member: cn=john,ou=users,dc=example,dc=com
LDIF

# Load entries using the demo OpenLDAP admin
kubectl -n d8-user-authn exec -it ldap-tools -- \
  ldapadd -x -H ldap://openldap.d8-user-authn.svc:389 \
  -D "cn=admin,dc=example,dc=com" -w admin -f /tmp/seed.ldif

# Sanity checks
kubectl -n d8-user-authn exec -it ldap-tools -- \
  ldapsearch -x -H ldap://openldap.d8-user-authn.svc:389 \
  -D "cn=admin,dc=example,dc=com" -w admin \
  -b "ou=users,dc=example,dc=com" "(cn=john)" dn

kubectl -n d8-user-authn exec -it ldap-tools -- \
  ldapwhoami -x -H ldap://openldap.d8-user-authn.svc:389 \
  -D "cn=john,ou=users,dc=example,dc=com" -w bar
```

Optional: create an OAuth2 client and grant RBAC for quick curl/browser tests:
```bash
export DEX_FQDN="dex.<your-domain>"

kubectl -n d8-user-authn apply -f - <<EOF
apiVersion: dex.coreos.com/v1
kind: OAuth2Client
metadata:
  name: spnego-test
  namespace: d8-user-authn
id: spnego-test
name: spnego-test
secret: dummy
redirectURIs:
- https://$DEX_FQDN/spnego-test-cb
EOF

kubectl apply -f - <<'EOF'
apiVersion: deckhouse.io/v1alpha1
kind: ClusterAuthorizationRule
metadata:
  name: john-superadmin
spec:
  subjects:
  - kind: User
    name: john@example.com
  accessLevel: SuperAdmin
EOF

kubectl auth can-i list oauth2clients.dex.coreos.com -n d8-user-authn --as=system:serviceaccount:d8-user-authn:dex
kubectl -n d8-user-authn rollout restart deploy/dex
kubectl -n d8-user-authn rollout status deploy/dex
```

Note: Ensure the DexProvider LDAP host matches your OpenLDAP service endpoint. This demo assumes `openldap.d8-user-authn.svc:389`. If you use the `openldap-demo` namespace from the basic LDAP demo, adjust the host accordingly.

4) Enable Kerberos on the DexProvider (demo LDAP)

```bash
d8 k apply -f ./dex-provider-kerberos.yaml
d8 k -n d8-user-authn rollout status deploy/dex
```

5) macOS client setup and test

```bash
# /etc/krb5.conf
sudo tee /etc/krb5.conf >/dev/null <<'EOF'
[libdefaults]
  default_realm = EXAMPLE.COM
  rdns = false
  dns_lookup_kdc = false
  udp_preference_limit = 1
[realms]
  EXAMPLE.COM = {
    kdc = tcp/127.0.0.1:8888
    admin_server = 127.0.0.1:8749
  }
EOF

# QUICK TEST ONLY: forward KDC NodePorts via bastion jump host (replace placeholders)
# Note: use this only for quick validation; see alternatives below for proper exposure.
ssh -f -N -J user@<bastion-host>:<port> \
  -L 8888:<node-ip>:30089 \
  -L 8749:<node-ip>:30749 \
  user@<node-ip>

kdestroy || true
kinit -V john@EXAMPLE.COM   # password: bar
klist

# Test via browser: open your protected app (e.g. Console or Kubeconfig Generator)
# and ensure it signs you in without a password. If you need to fully log out,
# run `kdestroy` on the client before re-opening the page.
```

macOS quick path (SSH jump), steps:

1) Configure `/etc/krb5.conf` on macOS:
```bash
sudo tee /etc/krb5.conf >/dev/null <<'EOF'
[libdefaults]
  default_realm = EXAMPLE.COM
  rdns = false
  dns_lookup_kdc = false
  udp_preference_limit = 1
[realms]
  EXAMPLE.COM = {
    kdc = tcp/127.0.0.1:8888
    admin_server = 127.0.0.1:8749
  }
EOF
```

2) Start an SSH tunnel to the KDC NodePorts via bastion (replace placeholders):
```bash
ssh -f -N -o ExitOnForwardFailure=yes -J user@<bastion-host>:<port> \
  -L 8888:<node-ip>:30089 \
  -L 8749:<node-ip>:30749 \
  user@<node-ip>

# Validate local ports:
nc -vz 127.0.0.1 8888
nc -vz 127.0.0.1 8749
```

3) Obtain a Kerberos TGT for the test user:
```bash
kdestroy || true
kinit -V john@EXAMPLE.COM   # password: bar
klist                        # ensure krbtgt/EXAMPLE.COM@EXAMPLE.COM is present
```

Notes:
- Provider selection screen appears only if you start the flow without `connector_id` and you have multiple providers.
- Logout with Kerberos: after Dex cookie is cleared, the browser may immediately re-authenticate via SPNEGO. To test a “clean” logout, run `kdestroy` (or temporarily remove the Dex host from the trusted SPNEGO list in your browser).

### Alternative: use your own external KDC

If you already have a KDC (e.g., Active Directory or MIT Kerberos):

1) Create SPN and keytab on your KDC for the Dex host:
```bash
# on external KDC
kadmin.local -q "addprinc -randkey HTTP/dex.<your-domain>@EXAMPLE.COM"
kadmin.local -q "ktadd -k /tmp/krb5.keytab HTTP/dex.<your-domain>@EXAMPLE.COM"
# demo user principal mapped to LDAP 'john'
kadmin.local -q 'addprinc -pw bar john@EXAMPLE.COM'
```

2) Copy keytab to the cluster and create a Secret for Dex:
```bash
d8 k -n d8-user-authn create secret generic dex-kerberos-test \
  --from-file=krb5.keytab=/path/to/krb5.keytab
```

3) Enable Kerberos on DexProvider (same as step 3 above). For clients, configure `/etc/krb5.conf` to point to your external KDC:
```ini
[realms]
  EXAMPLE.COM = {
    kdc = tcp/<external-kdc-host-or-ip>:88
    admin_server = <external-kdc-host-or-ip>:749
  }
```

### Alternative to SSH jump: expose KDC via NAT/LB (recommended over SSH for longer tests)

- NodePort + firewall DNAT on bastion (public IP = <public-ip>, KDC Node IP = <node-ip>):
```bash
# TCP 88 -> node:30089, TCP 749 -> node:30749
sudo iptables -t nat -A PREROUTING -p tcp --dport 88  -j DNAT --to-destination <node-ip>:30089
sudo iptables -t nat -A PREROUTING -p tcp --dport 749 -j DNAT --to-destination <node-ip>:30749
sudo iptables -t nat -A POSTROUTING -p tcp -d <node-ip> --dport 30089 -j MASQUERADE
sudo iptables -t nat -A POSTROUTING -p tcp -d <node-ip> --dport 30749 -j MASQUERADE
# then on macOS /etc/krb5.conf use:
#   kdc = tcp/<public-ip>:88
#   admin_server = <public-ip>:749
```

- Cloud LoadBalancer: publish TCP 88 -> node:30089 and TCP 749 -> node:30749, then point clients to the LB address in `/etc/krb5.conf`.

- The SSH jump method above is intended only for quick validation; prefer DNAT/LB for more stable testing.
