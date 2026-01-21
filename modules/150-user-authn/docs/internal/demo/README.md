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

**Browser setup (required for SPNEGO):**

Chrome on macOS requires explicit policy for Kerberos domains:
```bash
# Replace with your actual Dex domain (e.g., *.185.11.73.133.sslip.io)
defaults write com.google.Chrome AuthServerAllowlist "*.<your-domain>"
defaults write com.google.Chrome AuthNegotiateDelegateAllowlist "*.<your-domain>"
# Restart Chrome after setting policies
```

Firefox: set `network.negotiate-auth.trusted-uris` to your Dex domain in `about:config`.

Safari: usually works without extra config if the domain is in `.local` or Intranet zone.

0) Deploy OpenLDAP in d8-user-authn namespace (if not already present)

The Kerberos demo expects OpenLDAP at `openldap.d8-user-authn.svc:389`. Deploy it first:

```bash
d8 k -n d8-user-authn apply -f - <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: openldap
  namespace: d8-user-authn
spec:
  replicas: 1
  selector:
    matchLabels: { app: openldap }
  template:
    metadata:
      labels: { app: openldap }
    spec:
      initContainers:
      - name: copy-ldif
        image: busybox:1.36
        command: ["/bin/sh", "-c"]
        args:
        - |
          cp /config-src/config-ldap.ldif /bootstrap/config-ldap.ldif
        volumeMounts:
        - { name: config-src, mountPath: /config-src }
        - { name: bootstrap, mountPath: /bootstrap }
      containers:
      - name: openldap
        image: osixia/openldap:1.5.0
        args: ["--copy-service", "--loglevel", "debug"]
        env:
        - { name: LDAP_ORGANISATION, value: "Example" }
        - { name: LDAP_DOMAIN, value: "example.com" }
        - { name: LDAP_ADMIN_PASSWORD, value: "admin" }
        - { name: LDAP_TLS_VERIFY_CLIENT, value: "try" }
        volumeMounts:
        - name: bootstrap
          mountPath: /container/service/slapd/assets/config/bootstrap/ldif/custom
        ports:
        - { containerPort: 389, name: ldap }
      volumes:
      - name: config-src
        configMap: { name: ldap-bootstrap }
      - name: bootstrap
        emptyDir: {}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ldap-bootstrap
  namespace: d8-user-authn
data:
  config-ldap.ldif: |
    dn: ou=users,dc=example,dc=com
    objectClass: top
    objectClass: organizationalUnit
    ou: users

    dn: ou=groups,dc=example,dc=com
    objectClass: top
    objectClass: organizationalUnit
    ou: groups
---
apiVersion: v1
kind: Service
metadata:
  name: openldap
  namespace: d8-user-authn
spec:
  selector: { app: openldap }
  ports:
  - { port: 389, targetPort: 389, name: ldap }
EOF
d8 k -n d8-user-authn rollout status deploy/openldap
```

1) Deploy KDC in the cluster (namespace d8-user-authn)

```bash
d8 k -n d8-user-authn apply -f ./kerberos-kdc.yaml
d8 k -n d8-user-authn rollout status deploy/kdc

# Get assigned NodePorts (will be used later for SSH tunnel)
d8 k -n d8-user-authn get svc kdc -o jsonpath='{.spec.ports[?(@.name=="kdc-tcp")].nodePort}' && echo " # KDC TCP"
d8 k -n d8-user-authn get svc kdc -o jsonpath='{.spec.ports[?(@.name=="kadm")].nodePort}' && echo " # kadmin"
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
d8 k -n d8-user-authn run ldap-tools --restart=Never --image=osixia/openldap:1.5.0 -- sleep 3600
d8 k -n d8-user-authn wait --for=condition=Ready pod/ldap-tools --timeout=60s

# Prepare LDIF with john/bar and a devs group (dc=example,dc=com)
d8 k -n d8-user-authn exec -i ldap-tools -- bash -lc 'cat >/tmp/seed.ldif' <<'LDIF'
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
d8 k -n d8-user-authn exec -it ldap-tools -- \
  ldapadd -x -H ldap://openldap.d8-user-authn.svc:389 \
  -D "cn=admin,dc=example,dc=com" -w admin -f /tmp/seed.ldif

# Sanity checks
d8 k -n d8-user-authn exec -it ldap-tools -- \
  ldapsearch -x -H ldap://openldap.d8-user-authn.svc:389 \
  -D "cn=admin,dc=example,dc=com" -w admin \
  -b "ou=users,dc=example,dc=com" "(cn=john)" dn

d8 k -n d8-user-authn exec -it ldap-tools -- \
  ldapwhoami -x -H ldap://openldap.d8-user-authn.svc:389 \
  -D "cn=john,ou=users,dc=example,dc=com" -w bar
```

Optional: create an OAuth2 client and grant RBAC for quick curl/browser tests:
```bash
export DEX_FQDN="dex.<your-domain>"

d8 k -n d8-user-authn apply -f - <<EOF
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

d8 k apply -f - <<'EOF'
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

d8 k auth can-i list oauth2clients.dex.coreos.com -n d8-user-authn --as=system:serviceaccount:d8-user-authn:dex
d8 k -n d8-user-authn rollout restart deploy/dex
d8 k -n d8-user-authn rollout status deploy/dex
```

Note: Ensure the DexProvider LDAP host matches your OpenLDAP service endpoint. This demo assumes `openldap.d8-user-authn.svc:389`. If you use the `openldap-demo` namespace from the basic LDAP demo, adjust the host accordingly.

4) Enable Kerberos on the DexProvider (demo LDAP)

```bash
d8 k apply -f ./dex-provider-kerberos.yaml
d8 k -n d8-user-authn rollout status deploy/dex
```

5) macOS client setup and test

First, get the assigned NodePorts from step 1:
```bash
# Run on a machine with cluster access:
KDC_PORT=$(d8 k -n d8-user-authn get svc kdc -o jsonpath='{.spec.ports[?(@.name=="kdc-tcp")].nodePort}')
KADM_PORT=$(d8 k -n d8-user-authn get svc kdc -o jsonpath='{.spec.ports[?(@.name=="kadm")].nodePort}')
echo "KDC TCP: $KDC_PORT, kadmin: $KADM_PORT"
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

2) Start an SSH tunnel to the KDC NodePorts via bastion (replace placeholders with values from step 1):
```bash
# Replace <KDC_PORT> and <KADM_PORT> with values from step 1
ssh -f -N -o ExitOnForwardFailure=yes -J user@<bastion-host>:<port> \
  -L 8888:<node-ip>:<KDC_PORT> \
  -L 8749:<node-ip>:<KADM_PORT> \
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

4) Test via browser: open your protected app (e.g. Console or Kubeconfig Generator)
   and ensure it signs you in without a password. If you need to fully log out,
   run `kdestroy` on the client before re-opening the page.

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
# Get NodePorts first:
# KDC_PORT=$(d8 k -n d8-user-authn get svc kdc -o jsonpath='{.spec.ports[?(@.name=="kdc-tcp")].nodePort}')
# KADM_PORT=$(d8 k -n d8-user-authn get svc kdc -o jsonpath='{.spec.ports[?(@.name=="kadm")].nodePort}')

# TCP 88 -> node:<KDC_PORT>, TCP 749 -> node:<KADM_PORT>
sudo iptables -t nat -A PREROUTING -p tcp --dport 88  -j DNAT --to-destination <node-ip>:<KDC_PORT>
sudo iptables -t nat -A PREROUTING -p tcp --dport 749 -j DNAT --to-destination <node-ip>:<KADM_PORT>
sudo iptables -t nat -A POSTROUTING -p tcp -d <node-ip> --dport <KDC_PORT> -j MASQUERADE
sudo iptables -t nat -A POSTROUTING -p tcp -d <node-ip> --dport <KADM_PORT> -j MASQUERADE
# then on macOS /etc/krb5.conf use:
#   kdc = tcp/<public-ip>:88
#   admin_server = <public-ip>:749
```

- Cloud LoadBalancer: publish TCP 88 -> node:<KDC_PORT> and TCP 749 -> node:<KADM_PORT>, then point clients to the LB address in `/etc/krb5.conf`.

- The SSH jump method above is intended only for quick validation; prefer DNAT/LB for more stable testing.
