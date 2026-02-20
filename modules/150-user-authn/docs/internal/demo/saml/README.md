# SAML Demo (Keycloak)

## Prerequisites

- Deckhouse cluster with `user-authn` module enabled.
- `d8 k` CLI, `curl`, `jq` available.

## 1. Deploy Keycloak

Apply [**keycloak.yaml**](./keycloak.yaml) manifest to the cluster:

> **Note:** Replace `{{ __cluster__domain__ }}` with your actual cluster domain in the manifest before applying.

```shell
d8 k apply -f keycloak.yaml
d8 k -n saml-demo rollout status deploy/keycloak --timeout=300s
```

## 2. Configure Keycloak

Start port-forward and set variables:

```shell
d8 k -n saml-demo port-forward svc/keycloak 8080:8080 &
sleep 3
export KC_URL="http://localhost:8080"
export DEX_DOMAIN="dex.<your-cluster-domain>"
export KEYCLOAK_DOMAIN="keycloak.<your-cluster-domain>"
```

### 2.1. Create realm

```shell
export KC_TOKEN=$(curl -s -d "client_id=admin-cli" -d "username=admin" -d "password=admin" -d "grant_type=password" "${KC_URL}/realms/master/protocol/openid-connect/token" | jq -r '.access_token')

curl -s -X POST "${KC_URL}/admin/realms" \
  -H "Authorization: Bearer ${KC_TOKEN}" -H "Content-Type: application/json" \
  -d '{"realm":"dex-demo","enabled":true}'
```

### 2.2. Create SAML client with mappers

```shell
export KC_TOKEN=$(curl -s -d "client_id=admin-cli" -d "username=admin" -d "password=admin" -d "grant_type=password" "${KC_URL}/realms/master/protocol/openid-connect/token" | jq -r '.access_token')

curl -s -X POST "${KC_URL}/admin/realms/dex-demo/clients" \
  -H "Authorization: Bearer ${KC_TOKEN}" -H "Content-Type: application/json" \
  -d '{
    "clientId": "https://'"${DEX_DOMAIN}"'/callback",
    "protocol": "saml",
    "enabled": true,
    "frontchannelLogout": true,
    "attributes": {
      "saml.assertion.signature": "true",
      "saml.force.post.binding": "true",
      "saml_name_id_format": "persistent",
      "saml.server.signature": "true",
      "saml.signature.algorithm": "RSA_SHA256",
      "saml.authnstatement": "true",
      "saml_single_logout_service_url_post": "https://'"${DEX_DOMAIN}"'/saml/slo/saml-demo"
    },
    "redirectUris": ["https://'"${DEX_DOMAIN}"'/callback"],
    "protocolMappers": [
      {
        "name": "email",
        "protocol": "saml",
        "protocolMapper": "saml-user-property-mapper",
        "config": {"attribute.nameformat":"Basic","user.attribute":"email","friendly.name":"email","attribute.name":"email"}
      },
      {
        "name": "username",
        "protocol": "saml",
        "protocolMapper": "saml-user-property-mapper",
        "config": {"attribute.nameformat":"Basic","user.attribute":"username","friendly.name":"username","attribute.name":"name"}
      },
      {
        "name": "groups",
        "protocol": "saml",
        "protocolMapper": "saml-group-membership-mapper",
        "config": {"attribute.nameformat":"Basic","full.path":"false","attribute.name":"groups","single":"false"}
      }
    ]
  }'
```

### 2.3. Create groups and users

```shell
export KC_TOKEN=$(curl -s -d "client_id=admin-cli" -d "username=admin" -d "password=admin" -d "grant_type=password" "${KC_URL}/realms/master/protocol/openid-connect/token" | jq -r '.access_token') && \
curl -s -X POST "${KC_URL}/admin/realms/dex-demo/groups" -H "Authorization: Bearer ${KC_TOKEN}" -H "Content-Type: application/json" -d '{"name":"admins"}' && \
curl -s -X POST "${KC_URL}/admin/realms/dex-demo/groups" -H "Authorization: Bearer ${KC_TOKEN}" -H "Content-Type: application/json" -d '{"name":"developers"}' && \
export ADMINS_ID=$(curl -s "${KC_URL}/admin/realms/dex-demo/groups" -H "Authorization: Bearer ${KC_TOKEN}" | jq -r '.[] | select(.name=="admins") | .id') && \
export DEVS_ID=$(curl -s "${KC_URL}/admin/realms/dex-demo/groups" -H "Authorization: Bearer ${KC_TOKEN}" | jq -r '.[] | select(.name=="developers") | .id') && \
echo "Groups: admins=${ADMINS_ID}, developers=${DEVS_ID}"
```

**Create testuser1** (groups: admins, developers):

```shell
export KC_TOKEN=$(curl -s -d "client_id=admin-cli" -d "username=admin" -d "password=admin" -d "grant_type=password" "${KC_URL}/realms/master/protocol/openid-connect/token" | jq -r '.access_token') && \
curl -s -X POST "${KC_URL}/admin/realms/dex-demo/users" -H "Authorization: Bearer ${KC_TOKEN}" -H "Content-Type: application/json" \
  -d '{"username":"testuser1","email":"testuser1@example.com","enabled":true,"emailVerified":true,"credentials":[{"type":"password","value":"password123","temporary":false}]}' && \
export USER1_ID=$(curl -s "${KC_URL}/admin/realms/dex-demo/users?username=testuser1&exact=true" -H "Authorization: Bearer ${KC_TOKEN}" | jq -r '.[0].id') && \
curl -s -X PUT "${KC_URL}/admin/realms/dex-demo/users/${USER1_ID}/groups/${ADMINS_ID}" -H "Authorization: Bearer ${KC_TOKEN}" -H "Content-Type: application/json" -d '{}' && \
curl -s -X PUT "${KC_URL}/admin/realms/dex-demo/users/${USER1_ID}/groups/${DEVS_ID}" -H "Authorization: Bearer ${KC_TOKEN}" -H "Content-Type: application/json" -d '{}' && \
echo "testuser1 created (admins, developers)"
```

**Create testuser2** (group: developers):

```shell
export KC_TOKEN=$(curl -s -d "client_id=admin-cli" -d "username=admin" -d "password=admin" -d "grant_type=password" "${KC_URL}/realms/master/protocol/openid-connect/token" | jq -r '.access_token') && \
curl -s -X POST "${KC_URL}/admin/realms/dex-demo/users" -H "Authorization: Bearer ${KC_TOKEN}" -H "Content-Type: application/json" \
  -d '{"username":"testuser2","email":"testuser2@example.com","enabled":true,"emailVerified":true,"credentials":[{"type":"password","value":"password456","temporary":false}]}' && \
export USER2_ID=$(curl -s "${KC_URL}/admin/realms/dex-demo/users?username=testuser2&exact=true" -H "Authorization: Bearer ${KC_TOKEN}" | jq -r '.[0].id') && \
curl -s -X PUT "${KC_URL}/admin/realms/dex-demo/users/${USER2_ID}/groups/${DEVS_ID}" -H "Authorization: Bearer ${KC_TOKEN}" -H "Content-Type: application/json" -d '{}' && \
echo "testuser2 created (developers)"
```

### 2.4. Stop port-forward

```shell
kill %1 2>/dev/null
```

## 3. Add DexProvider

Apply [**dex-provider.yaml**](./dex-provider.yaml) manifest:

> **Note:** Replace `{{ __cluster__domain__ }}` with your actual cluster domain in the manifest before applying.

```shell
d8 k apply -f dex-provider.yaml
```

Wait for Dex to pick up the new connector (~30-60s):

```shell
d8 k -n d8-user-authn logs -l app=dex --tail=20 | grep -i "saml-demo"
```

## 4. Test login

Open Kubeconfig or any DexAuthenticator-protected app and select **SAML Demo (Keycloak)**.

- `testuser1` / `password123` — groups: `superadmins`, `admins`
- `testuser2` / `password456` — group: `admins`

Verify refresh token was issued:

```shell
d8 k -n d8-user-authn get refreshtokens.dex.coreos.com -o json | \
  jq '.items[] | select(.connectorID == "saml-demo") | {email: .claims.email, groups: .claims.groups}'
```

## Cleaning

Execute [**clean_up.sh**](./clean_up.sh):

```shell
chmod +x clean_up.sh
./clean_up.sh
```
